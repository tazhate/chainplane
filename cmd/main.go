/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	nodesv1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/controller"
	"github.com/tazhate/chainplane/internal/health"

	// Register all chain adapters via init().
	_ "github.com/tazhate/chainplane/internal/adapters"
	// +kubebuilder:scaffold:imports
)

// ---------------------------------------------------------------------------
// Build information — set via ldflags at compile time:
//
//	go build -ldflags "-X main.version=v1.2.3 -X main.gitCommit=$(git rev-parse --short HEAD)"
//
// ---------------------------------------------------------------------------
var (
	version   = "dev"
	gitCommit = "unknown"
)

// ---------------------------------------------------------------------------
// Default values
// ---------------------------------------------------------------------------

const (
	defaultMetricsAddr   = ":8080"
	defaultProbeAddr     = ":8081"
	defaultTLSCertName   = "tls.crt"
	defaultTLSKeyName    = "tls.key"
	defaultPrometheusURL = "http://prometheus:9090"
	defaultLeaderElectID = "f8c0f89a.nodes.chainplane.io"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(nodesv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// Config holds all command-line flags grouped by concern.
type Config struct {
	// Server
	MetricsAddr string
	ProbeAddr   string

	// TLS — metrics
	MetricsCertPath string
	MetricsCertName string
	MetricsCertKey  string

	// TLS — webhook
	WebhookCertPath string
	WebhookCertName string
	WebhookCertKey  string

	// Operator behaviour
	LeaderElect   bool
	SecureMetrics bool
	EnableHTTP2   bool
	PrometheusURL string
}

// bindFlags registers all CLI flags on the standard flag set.
func (c *Config) bindFlags() {
	// Server flags
	flag.StringVar(&c.MetricsAddr, "metrics-bind-address", defaultMetricsAddr,
		"The address the metrics endpoint binds to. Use :8443 for HTTPS or :8080 for HTTP.")
	flag.StringVar(&c.ProbeAddr, "health-probe-bind-address", defaultProbeAddr,
		"The address the health/readiness probe endpoint binds to.")

	// TLS — webhook
	flag.StringVar(&c.WebhookCertPath, "webhook-cert-path", "",
		"Directory containing the webhook TLS certificate and key.")
	flag.StringVar(&c.WebhookCertName, "webhook-cert-name", defaultTLSCertName,
		"Filename of the webhook TLS certificate inside --webhook-cert-path.")
	flag.StringVar(&c.WebhookCertKey, "webhook-cert-key", defaultTLSKeyName,
		"Filename of the webhook TLS key inside --webhook-cert-path.")

	// TLS — metrics
	flag.StringVar(&c.MetricsCertPath, "metrics-cert-path", "",
		"Directory containing the metrics server TLS certificate and key.")
	flag.StringVar(&c.MetricsCertName, "metrics-cert-name", defaultTLSCertName,
		"Filename of the metrics server TLS certificate inside --metrics-cert-path.")
	flag.StringVar(&c.MetricsCertKey, "metrics-cert-key", defaultTLSKeyName,
		"Filename of the metrics server TLS key inside --metrics-cert-path.")

	// Operator behaviour
	flag.BoolVar(&c.LeaderElect, "leader-elect", false,
		"Enable leader election for controller manager to ensure only one active instance.")
	flag.BoolVar(&c.SecureMetrics, "metrics-secure", true,
		"Serve the metrics endpoint via HTTPS. Use --metrics-secure=false for HTTP.")
	flag.BoolVar(&c.EnableHTTP2, "enable-http2", false,
		"Enable HTTP/2 for metrics and webhook servers (disabled by default for CVE mitigation).")
	flag.StringVar(&c.PrometheusURL, "prometheus-url", defaultPrometheusURL,
		"Base URL of the Prometheus server used for node health checks.")
}

// validate checks that the configuration is internally consistent.
func (c *Config) validate() error {
	if c.MetricsCertPath != "" {
		if _, err := os.Stat(filepath.Join(c.MetricsCertPath, c.MetricsCertName)); err != nil {
			return fmt.Errorf("metrics certificate not found: %w", err)
		}
		if _, err := os.Stat(filepath.Join(c.MetricsCertPath, c.MetricsCertKey)); err != nil {
			return fmt.Errorf("metrics key not found: %w", err)
		}
	}
	if c.WebhookCertPath != "" {
		if _, err := os.Stat(filepath.Join(c.WebhookCertPath, c.WebhookCertName)); err != nil {
			return fmt.Errorf("webhook certificate not found: %w", err)
		}
		if _, err := os.Stat(filepath.Join(c.WebhookCertPath, c.WebhookCertKey)); err != nil {
			return fmt.Errorf("webhook key not found: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// TLS helpers
// ---------------------------------------------------------------------------

// baseTLSOpts returns the common TLS options, disabling HTTP/2 when requested.
func baseTLSOpts(enableHTTP2 bool) []func(*tls.Config) {
	if enableHTTP2 {
		return nil
	}
	// Disable HTTP/2 to mitigate Stream Cancellation and Rapid Reset CVEs:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	return []func(*tls.Config){
		func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		},
	}
}

// setupCertWatcher creates a CertWatcher for the given cert directory.
// Returns nil if certPath is empty.
func setupCertWatcher(certPath, certName, certKey, label string) (*certwatcher.CertWatcher, error) {
	if certPath == "" {
		return nil, nil
	}
	setupLog.Info(fmt.Sprintf("initializing %s certificate watcher", label),
		"cert-path", certPath, "cert-name", certName, "cert-key", certKey)

	cw, err := certwatcher.New(
		filepath.Join(certPath, certName),
		filepath.Join(certPath, certKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize %s certificate watcher: %w", label, err)
	}
	return cw, nil
}

// ---------------------------------------------------------------------------
// Controller setup helpers
// ---------------------------------------------------------------------------

// setupBlockchainController creates and registers the BlockchainNode reconciler.
func setupBlockchainController(mgr ctrl.Manager) error {
	kubeClientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("creating kubernetes clientset: %w", err)
	}

	reconciler := &controller.BlockchainNodeReconciler{
		Client:        mgr.GetClient(),
		APIReader:     mgr.GetAPIReader(),
		Scheme:        mgr.GetScheme(),
		KubeClientset: kubeClientset,
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("creating BlockchainNode controller: %w", err)
	}
	return nil
}

// setupVersionCatalogController creates and registers the ChainVersionCatalog reconciler.
func setupVersionCatalogController(mgr ctrl.Manager) error {
	reconciler := &controller.ChainVersionCatalogReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("creating ChainVersionCatalog controller: %w", err)
	}
	return nil
}

// setupHealthController creates and registers the NodeHealth reconciler.
func setupHealthController(mgr ctrl.Manager, prometheusURL string) error {
	promClient := health.NewPrometheusClient(prometheusURL)
	healthChecker := health.NewChecker(promClient, health.DefaultThresholds())
	trafficMgr := health.NewLabelBasedTrafficManager(mgr.GetClient())
	replacementMgr := health.NewReplacementManager(
		mgr.GetClient(), healthChecker, trafficMgr, health.DefaultReplacementConfig(),
	)

	reconciler := &controller.NodeHealthReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Checker:     healthChecker,
		Replacement: replacementMgr,
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("creating NodeHealth controller: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	var cfg Config
	cfg.bindFlags()

	zapOpts := zap.Options{Development: true}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	setupLog.Info("starting chainplane",
		"version", version,
		"commit", gitCommit,
		"metrics-addr", cfg.MetricsAddr,
		"probe-addr", cfg.ProbeAddr,
		"leader-elect", cfg.LeaderElect,
		"secure-metrics", cfg.SecureMetrics,
		"http2", cfg.EnableHTTP2,
		"prometheus-url", cfg.PrometheusURL,
	)

	if err := cfg.validate(); err != nil {
		setupLog.Error(err, "invalid configuration")
		os.Exit(1)
	}

	tlsOpts := baseTLSOpts(cfg.EnableHTTP2)

	// --- Webhook server ------------------------------------------------
	webhookCertWatcher, err := setupCertWatcher(
		cfg.WebhookCertPath, cfg.WebhookCertName, cfg.WebhookCertKey, "webhook")
	if err != nil {
		setupLog.Error(err, "webhook certificate watcher setup failed")
		os.Exit(1)
	}

	webhookTLSOpts := append([]func(*tls.Config){}, tlsOpts...)
	if webhookCertWatcher != nil {
		webhookTLSOpts = append(webhookTLSOpts, func(c *tls.Config) {
			c.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// --- Metrics server ------------------------------------------------
	metricsServerOptions := metricsserver.Options{
		BindAddress:   cfg.MetricsAddr,
		SecureServing: cfg.SecureMetrics,
		TLSOpts:       tlsOpts,
	}

	if cfg.SecureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	metricsCertWatcher, err := setupCertWatcher(
		cfg.MetricsCertPath, cfg.MetricsCertName, cfg.MetricsCertKey, "metrics")
	if err != nil {
		setupLog.Error(err, "metrics certificate watcher setup failed")
		os.Exit(1)
	}
	if metricsCertWatcher != nil {
		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(c *tls.Config) {
			c.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	// --- Controller manager --------------------------------------------
	leaseDuration := 40 * time.Second
	renewDeadline := 25 * time.Second
	retryPeriod := 5 * time.Second
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsServerOptions,
		WebhookServer:           webhookServer,
		HealthProbeBindAddress:  cfg.ProbeAddr,
		LeaderElection:          cfg.LeaderElect,
		LeaderElectionID:        defaultLeaderElectID,
		LeaseDuration:           &leaseDuration,
		RenewDeadline:           &renewDeadline,
		RetryPeriod:             &retryPeriod,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// --- Controllers ---------------------------------------------------
	if err := setupBlockchainController(mgr); err != nil {
		setupLog.Error(err, "controller setup failed", "controller", "BlockchainNode")
		os.Exit(1)
	}
	if err := setupHealthController(mgr, cfg.PrometheusURL); err != nil {
		setupLog.Error(err, "controller setup failed", "controller", "NodeHealth")
		os.Exit(1)
	}
	if err := setupVersionCatalogController(mgr); err != nil {
		setupLog.Error(err, "controller setup failed", "controller", "ChainVersionCatalog")
		os.Exit(1)
	}

	// --- Webhook -------------------------------------------------------
	// Webhook requires TLS certs (injected by cert-manager in production).
	// When certs are not present (e.g. local dev, e2e tests without cert-manager),
	// skip webhook setup gracefully instead of crashing.
	webhookCertDir := cfg.WebhookCertPath
	if webhookCertDir == "" {
		webhookCertDir = "/tmp/k8s-webhook-server/serving-certs"
	}
	if _, err := os.Stat(filepath.Join(webhookCertDir, cfg.WebhookCertName)); err == nil {
		if err := (&nodesv1alpha1.BlockchainNode{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "BlockchainNode")
			os.Exit(1)
		}
		setupLog.Info("webhook registered", "cert-dir", webhookCertDir)
	} else {
		setupLog.Info("webhook skipped — TLS certs not found, running without admission validation",
			"cert-dir", webhookCertDir)
	}
	// +kubebuilder:scaffold:builder

	// --- Certificate watchers ------------------------------------------
	if metricsCertWatcher != nil {
		setupLog.Info("adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}
	if webhookCertWatcher != nil {
		setupLog.Info("adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	// --- Health probes -------------------------------------------------
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// --- Start ---------------------------------------------------------
	// ctrl.SetupSignalHandler() returns a context that is cancelled on
	// SIGTERM or SIGINT, enabling graceful shutdown of all controllers.
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
