package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
)

// Server is the Fleet Status HTTP server.
type Server struct {
	client    client.Client
	namespace string
	refresh   time.Duration
	metrics   *dashboardMetrics
	registry  *prometheus.Registry

	mu    sync.RWMutex
	cache *NodeListResponse
}

// Config holds Server constructor parameters.
type Config struct {
	Client    client.Client
	Namespace string
	Refresh   time.Duration
}

// New creates a Server and registers Prometheus metrics on a fresh registry.
func New(cfg Config) *Server {
	reg := prometheus.NewRegistry()
	// Also register default Go/process collectors.
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	s := &Server{
		client:    cfg.Client,
		namespace: cfg.Namespace,
		refresh:   cfg.Refresh,
		registry:  reg,
		metrics:   newMetrics(reg),
	}
	return s
}

// Handler returns the root http.Handler with all routes registered.
func (s *Server) Handler(fs http.FileSystem) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/api/nodes/", s.handleNodeDetail)
	mux.HandleFunc("/api/nodes", s.handleNodeList)
	mux.Handle("/", http.FileServer(fs))
	return mux
}

// Start launches the background cache refresh loop and blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) {
	// Initial fill.
	s.refreshCache(ctx)

	ticker := time.NewTicker(s.refresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshCache(ctx)
		}
	}
}

func (s *Server) refreshCache(ctx context.Context) {
	list := &v1alpha1.BlockchainNodeList{}
	opts := []client.ListOption{}
	if s.namespace != "" {
		opts = append(opts, client.InNamespace(s.namespace))
	}

	if err := s.client.List(ctx, list, opts...); err != nil {
		s.metrics.scrapeErrors.Inc()
		return
	}

	nodes := make([]NodeInfo, 0, len(list.Items))
	for i := range list.Items {
		nodes = append(nodes, nodeToInfo(&list.Items[i]))
	}

	summary := buildSummary(nodes)
	resp := &NodeListResponse{
		Nodes:     nodes,
		Summary:   summary,
		FetchedAt: time.Now().UTC(),
	}

	s.metrics.update(nodes)
	s.metrics.lastScrapeTime.SetToCurrentTime()

	s.mu.Lock()
	s.cache = resp
	s.mu.Unlock()
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleNodeList(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	resp := s.cache
	s.mu.RUnlock()

	if resp == nil {
		resp = &NodeListResponse{
			Nodes:     []NodeInfo{},
			FetchedAt: time.Now().UTC(),
		}
	}
	writeJSON(w, resp)
}

func (s *Server) handleNodeDetail(w http.ResponseWriter, r *http.Request) {
	// Path: /api/nodes/{namespace}/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/nodes/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "expected /api/nodes/{namespace}/{name}", http.StatusBadRequest)
		return
	}
	ns, name := parts[0], parts[1]

	s.mu.RLock()
	resp := s.cache
	s.mu.RUnlock()

	if resp != nil {
		for _, n := range resp.Nodes {
			if n.Namespace == ns && n.Name == name {
				writeJSON(w, n)
				return
			}
		}
	}
	http.Error(w, fmt.Sprintf("node %s/%s not found", ns, name), http.StatusNotFound)
}

// nodeToInfo converts a BlockchainNode CR to the API type.
func nodeToInfo(n *v1alpha1.BlockchainNode) NodeInfo {
	replicas := int32(1)
	if n.Spec.Replicas != nil {
		replicas = *n.Spec.Replicas
	}

	return NodeInfo{
		Name:         n.Name,
		Namespace:    n.Namespace,
		Chain:        string(n.Spec.Chain),
		Network:      string(n.Spec.Network),
		NodeType:     string(n.Spec.NodeType),
		Phase:        string(n.Status.Phase),
		BlockHeight:  n.Status.BlockHeight,
		SyncProgress: n.Status.SyncProgress,
		SyncETA:      n.Status.SyncETA,
		PeersCount:   n.Status.PeersCount,
		Ready:        n.Status.IsReady(),
		Age:          formatAge(n.CreationTimestamp.Time),
		Client:       n.Spec.Client,
		Replicas:     replicas,
	}
}

// buildSummary counts nodes per phase.
func buildSummary(nodes []NodeInfo) Summary {
	s := Summary{Total: len(nodes)}
	for _, n := range nodes {
		switch v1alpha1.NodePhase(n.Phase) {
		case v1alpha1.NodePhaseHealthy:
			s.Healthy++
		case v1alpha1.NodePhaseSyncing:
			s.Syncing++
		case v1alpha1.NodePhaseDegraded:
			s.Degraded++
		case v1alpha1.NodePhaseFailed:
			s.Failed++
		default:
			s.Pending++
		}
	}
	return s
}

// formatAge returns a human-readable age string like "3d", "2h", "5m".
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
