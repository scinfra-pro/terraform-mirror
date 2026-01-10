package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/scinfra-pro/terraform-mirror/internal/cache"
	"github.com/scinfra-pro/terraform-mirror/internal/upstream"
)

// Registry represents a client for working with Terraform Registry API
type Registry struct {
	client    *upstream.Client
	hashCache *cache.HashCache
	logger    *slog.Logger
}

// New creates a new Registry client
func New(client *upstream.Client, hashCache *cache.HashCache, logger *slog.Logger) *Registry {
	return &Registry{
		client:    client,
		hashCache: hashCache,
		logger:    logger,
	}
}

// HashCache returns the hash cache
func (r *Registry) HashCache() *cache.HashCache {
	return r.hashCache
}

// ProviderVersions returns list of provider versions in Mirror Protocol format
// GET /v1/providers/{hostname}/{namespace}/{type}/versions -> index.json
func (r *Registry) ProviderVersions(ctx context.Context, namespace, name string) ([]byte, error) {
	// Request to Registry API
	// https://registry.terraform.io/v1/providers/{namespace}/{type}/versions
	path := fmt.Sprintf("/v1/providers/%s/%s/versions", namespace, name)

	r.logger.Debug("fetching provider versions", "path", path)

	body, statusCode, err := r.client.GetJSON(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("upstream returned status %d", statusCode)
	}

	// Parse Registry API response
	var registryResp RegistryVersionsResponse
	if err := json.Unmarshal(body, &registryResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Transform to Mirror Protocol format
	mirrorResp := MirrorVersionsResponse{
		Versions: make(map[string]struct{}),
	}

	for _, v := range registryResp.Versions {
		mirrorResp.Versions[v.Version] = struct{}{}
	}

	return json.Marshal(mirrorResp)
}

// ProviderVersion returns information about a specific version in Mirror Protocol format
// GET /v1/providers/{hostname}/{namespace}/{type}/{version} -> {version}.json
func (r *Registry) ProviderVersion(ctx context.Context, namespace, name, version string) ([]byte, error) {
	// Request to Registry API to get platform information
	// https://registry.terraform.io/v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}
	// But it's easier to get all platforms through versions endpoint

	path := fmt.Sprintf("/v1/providers/%s/%s/versions", namespace, name)

	r.logger.Debug("fetching provider version", "path", path, "version", version)

	body, statusCode, err := r.client.GetJSON(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("upstream returned status %d", statusCode)
	}

	// Parse Registry API response
	var registryResp RegistryVersionsResponse
	if err := json.Unmarshal(body, &registryResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Find target version
	var targetVersion *RegistryVersion
	for _, v := range registryResp.Versions {
		if v.Version == version {
			targetVersion = &v
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("version %s not found", version)
	}

	// Transform to Mirror Protocol format
	mirrorResp := MirrorVersionResponse{
		Archives: make(map[string]MirrorArchive),
	}

	// Get all hashes for this version from cache
	cachedHashes := r.hashCache.GetAll(namespace, name, version)

	for _, p := range targetVersion.Platforms {
		platform := fmt.Sprintf("%s_%s", p.OS, p.Arch)
		filename := fmt.Sprintf("terraform-provider-%s_%s_%s_%s.zip", name, version, p.OS, p.Arch)

		archive := MirrorArchive{
			URL: filename,
		}

		// Add h1 hash if it exists in cache
		if h1, ok := cachedHashes[platform]; ok {
			archive.Hashes = []string{h1}
		}

		mirrorResp.Archives[platform] = archive
	}

	return json.Marshal(mirrorResp)
}

// DownloadURL returns the download URL for a provider
func (r *Registry) DownloadURL(ctx context.Context, namespace, name, version, os, arch string) (string, error) {
	// GET /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}
	path := fmt.Sprintf("/v1/providers/%s/%s/%s/download/%s/%s", namespace, name, version, os, arch)

	r.logger.Debug("fetching download URL", "path", path)

	body, statusCode, err := r.client.GetJSON(ctx, path)
	if err != nil {
		return "", fmt.Errorf("fetching download URL: %w", err)
	}

	if statusCode != 200 {
		return "", fmt.Errorf("upstream returned status %d", statusCode)
	}

	var downloadResp RegistryDownloadResponse
	if err := json.Unmarshal(body, &downloadResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	return downloadResp.DownloadURL, nil
}

// ParseZipFilename parses a provider filename
// terraform-provider-{name}_{version}_{os}_{arch}.zip
func ParseZipFilename(filename string) (name, version, os, arch string, err error) {
	// Remove .zip
	filename = strings.TrimSuffix(filename, ".zip")

	// Remove terraform-provider- prefix
	if !strings.HasPrefix(filename, "terraform-provider-") {
		return "", "", "", "", fmt.Errorf("invalid filename format")
	}
	filename = strings.TrimPrefix(filename, "terraform-provider-")

	// Split: name_version_os_arch
	parts := strings.Split(filename, "_")
	if len(parts) < 4 {
		return "", "", "", "", fmt.Errorf("invalid filename format: not enough parts")
	}

	// name may contain _, so take last 3 parts
	arch = parts[len(parts)-1]
	os = parts[len(parts)-2]
	version = parts[len(parts)-3]
	name = strings.Join(parts[:len(parts)-3], "_")

	return name, version, os, arch, nil
}

// === Types ===

// RegistryVersionsResponse — Registry API response /v1/providers/{ns}/{type}/versions
type RegistryVersionsResponse struct {
	Versions []RegistryVersion `json:"versions"`
}

type RegistryVersion struct {
	Version   string             `json:"version"`
	Platforms []RegistryPlatform `json:"platforms"`
}

type RegistryPlatform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// RegistryDownloadResponse — Registry API response /download/{os}/{arch}
type RegistryDownloadResponse struct {
	DownloadURL string `json:"download_url"`
	Filename    string `json:"filename"`
	SHA256Sum   string `json:"shasum"`
}

// MirrorVersionsResponse — Mirror Protocol response index.json
type MirrorVersionsResponse struct {
	Versions map[string]struct{} `json:"versions"`
}

// MirrorVersionResponse — Mirror Protocol response {version}.json
type MirrorVersionResponse struct {
	Archives map[string]MirrorArchive `json:"archives"`
}

type MirrorArchive struct {
	URL    string   `json:"url"`
	Hashes []string `json:"hashes,omitempty"`
}

