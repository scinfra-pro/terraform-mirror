package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/scinfra-pro/terraform-mirror/internal/cache"
	"github.com/scinfra-pro/terraform-mirror/internal/config"
	"github.com/scinfra-pro/terraform-mirror/internal/registry"
	"github.com/scinfra-pro/terraform-mirror/internal/upstream"
)

// Server represents the HTTP server
type Server struct {
	cfg       *config.Config
	logger    *slog.Logger
	mux       *http.ServeMux
	registry  *registry.Registry
	upstream  *upstream.Client
	hashCache *cache.HashCache
}

// New creates a new server
func New(cfg *config.Config, logger *slog.Logger) *Server {
	upstreamClient, err := upstream.New(cfg.UpstreamURL, cfg.UpstreamTimeout, cfg.SOCKS5Addr)
	if err != nil {
		logger.Error("failed to create upstream client", "error", err)
		panic(err)
	}

	if cfg.SOCKS5Addr != "" {
		logger.Info("SOCKS5 proxy enabled", "addr", cfg.SOCKS5Addr)
	}

	hashCache := cache.NewHashCache(cfg.CacheDir)
	reg := registry.New(upstreamClient, hashCache, logger)

	s := &Server{
		cfg:       cfg,
		logger:    logger,
		mux:       http.NewServeMux(),
		registry:  reg,
		upstream:  upstreamClient,
		hashCache: hashCache,
	}
	s.setupRoutes()
	return s
}

// setupRoutes configures the routes
func (s *Server) setupRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Mirror Protocol endpoints
	// /v1/providers/{hostname}/{namespace}/{type}/...
	s.mux.HandleFunc("GET /v1/providers/", s.handleProviders)
}

// handleProviders handles Mirror Protocol requests
// /v1/providers/{hostname}/{namespace}/{type}/index.json
// /v1/providers/{hostname}/{namespace}/{type}/{version}.json
// /v1/providers/{hostname}/{namespace}/{type}/*.zip
func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	// Parse path: /v1/providers/{hostname}/{namespace}/{type}/{file}
	path := strings.TrimPrefix(r.URL.Path, "/v1/providers/")
	parts := strings.Split(path, "/")

	if len(parts) < 4 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	hostname := parts[0]  // registry.terraform.io
	namespace := parts[1] // hashicorp
	name := parts[2]      // random
	file := parts[3]      // index.json, 3.6.0.json, or *.zip

	s.logger.Debug("provider request",
		"hostname", hostname,
		"namespace", namespace,
		"name", name,
		"file", file,
	)

	ctx := r.Context()

	switch {
	case file == "index.json":
		s.handleVersions(ctx, w, namespace, name)

	case strings.HasSuffix(file, ".json"):
		version := strings.TrimSuffix(file, ".json")
		s.handleVersion(ctx, w, namespace, name, version)

	case strings.HasSuffix(file, ".zip"):
		s.handleDownload(ctx, w, namespace, name, file)

	default:
		http.Error(w, "unknown file type", http.StatusBadRequest)
	}
}

// Run starts the server with graceful shutdown
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.cfg.ListenAddr,
		Handler:      s.mux,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("starting server", "addr", s.cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

