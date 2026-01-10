package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/scinfra-pro/terraform-mirror/internal/hash"
	"github.com/scinfra-pro/terraform-mirror/internal/registry"
)

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleVersions handles GET index.json — list of versions
func (s *Server) handleVersions(ctx context.Context, w http.ResponseWriter, namespace, name string) {
	s.logger.Info("fetching versions", "provider", namespace+"/"+name)

	data, err := s.registry.ProviderVersions(ctx, namespace, name)
	if err != nil {
		s.logger.Error("failed to fetch versions", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// handleVersion handles GET {version}.json — platform information
func (s *Server) handleVersion(ctx context.Context, w http.ResponseWriter, namespace, name, version string) {
	s.logger.Info("fetching version", "provider", namespace+"/"+name, "version", version)

	data, err := s.registry.ProviderVersion(ctx, namespace, name, version)
	if err != nil {
		s.logger.Error("failed to fetch version", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// handleDownload handles GET *.zip — proxy archive with h1 hash calculation
func (s *Server) handleDownload(ctx context.Context, w http.ResponseWriter, namespace, providerName, filename string) {
	s.logger.Info("downloading provider", "provider", namespace+"/"+providerName, "file", filename)

	// Parse filename: terraform-provider-{name}_{version}_{os}_{arch}.zip
	name, version, osName, arch, err := registry.ParseZipFilename(filename)
	if err != nil {
		s.logger.Error("failed to parse filename", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	platform := fmt.Sprintf("%s_%s", osName, arch)

	// Check if h1 hash exists in cache
	_, hasHash := s.hashCache.Get(namespace, name, version, platform)

	// Get download URL
	downloadURL, err := s.registry.DownloadURL(ctx, namespace, name, version, osName, arch)
	if err != nil {
		s.logger.Error("failed to get download URL", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	s.logger.Debug("proxying download", "url", downloadURL, "hasHash", hasHash)

	// Make request to download URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		s.logger.Error("failed to create request", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("failed to download", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("download failed", "status", resp.StatusCode)
		http.Error(w, "download failed", resp.StatusCode)
		return
	}

	// If no hash — save to temp file, calculate h1, serve from file
	if !hasHash {
		s.downloadWithHash(w, resp, namespace, name, version, platform)
		return
	}

	// Hash already exists — just stream
	w.Header().Set("Content-Type", "application/zip")
	if resp.ContentLength > 0 {
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	}
	_, _ = io.Copy(w, resp.Body)
}

// downloadWithHash downloads ZIP, calculates h1, saves to cache and serves to client
func (s *Server) downloadWithHash(w http.ResponseWriter, resp *http.Response, namespace, name, version, platform string) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "provider-*.zip")
	if err != nil {
		s.logger.Error("failed to create temp file", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy data to temporary file
	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		s.logger.Error("failed to write temp file", "error", err)
		http.Error(w, "download error", http.StatusBadGateway)
		return
	}

	// Calculate h1 hash
	h1, err := hash.CalculateH1(tmpFile.Name())
	if err != nil {
		s.logger.Error("failed to calculate h1", "error", err)
		// Continue without hash — this is a non-critical error
	} else {
		// Save h1 to cache
		if err := s.hashCache.Set(namespace, name, version, platform, h1); err != nil {
			s.logger.Error("failed to cache h1", "error", err)
		} else {
			s.logger.Info("cached h1 hash", "provider", namespace+"/"+name, "version", version, "platform", platform, "h1", h1)
		}
	}

	// Seek back to beginning of file
	if _, err := tmpFile.Seek(0, 0); err != nil {
		s.logger.Error("failed to seek temp file", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Serve file to client
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", written))
	_, _ = io.Copy(w, tmpFile)
}
