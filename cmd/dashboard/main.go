// Command dashboard runs the Fleet Status HTTP server.
// It connects to a Kubernetes cluster, watches BlockchainNode CRs, and
// serves a web UI plus a Prometheus metrics endpoint.
//
// Flags:
//
//	--port       int      HTTP listen port (default 8090)
//	--namespace  string   Namespace to watch; empty = all namespaces
//	--kubeconfig string   Path to kubeconfig (registered by controller-runtime)
//	--refresh    duration Cache refresh interval (default 15s)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcfg "sigs.k8s.io/controller-runtime/pkg/client/config"

	v1alpha1 "github.com/tazhate/chainplane/api/v1alpha1"
	"github.com/tazhate/chainplane/internal/dashboard"
)

func main() {
	var (
		port      = flag.Int("port", 8090, "HTTP listen port")
		namespace = flag.String("namespace", "", "Namespace to watch (empty = all)")
		refresh   = flag.Duration("refresh", 15*time.Second, "Cache refresh interval")
	)
	flag.Parse()

	// --kubeconfig is registered by sigs.k8s.io/controller-runtime/pkg/client/config init().
	// ctrlcfg.GetConfig() reads that flag automatically.
	restCfg, err := ctrlcfg.GetConfig()
	if err != nil {
		log.Fatalf("build rest config: %v", err)
	}

	k8sClient, err := client.New(restCfg, client.Options{Scheme: buildScheme()})
	if err != nil {
		log.Fatalf("build k8s client: %v", err)
	}

	srv := dashboard.New(dashboard.Config{
		Client:    k8sClient,
		Namespace: *namespace,
		Refresh:   *refresh,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go srv.Start(ctx)

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      srv.Handler(http.FS(dashboard.WebFS())),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("Fleet Status dashboard listening on :%d", *port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func buildScheme() *k8sruntime.Scheme {
	s := k8sruntime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		log.Fatalf("add scheme: %v", err)
	}
	return s
}
