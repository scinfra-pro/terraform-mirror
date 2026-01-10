package cache

import (
	"os"
	"path/filepath"
	"strings"
)

// HashCache stores h1 hashes of providers in files
type HashCache struct {
	baseDir string
}

// NewHashCache creates a new hash cache
func NewHashCache(baseDir string) *HashCache {
	return &HashCache{baseDir: baseDir}
}

// keyToPath converts key to file path
// Key: "hashicorp/random/3.6.0/linux_amd64"
// Path: cache/hashes/hashicorp/random/3.6.0_linux_amd64.h1
func (c *HashCache) keyToPath(namespace, name, version, platform string) string {
	filename := version + "_" + platform + ".h1"
	return filepath.Join(c.baseDir, "hashes", namespace, name, filename)
}

// Get returns h1 hash from cache
func (c *HashCache) Get(namespace, name, version, platform string) (string, bool) {
	path := c.keyToPath(namespace, name, version, platform)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

// Set saves h1 hash to cache
func (c *HashCache) Set(namespace, name, version, platform, hash string) error {
	path := c.keyToPath(namespace, name, version, platform)

	// Create directories
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(hash), 0644)
}

// GetAll returns all hashes for a provider version
func (c *HashCache) GetAll(namespace, name, version string) map[string]string {
	result := make(map[string]string)

	dir := filepath.Join(c.baseDir, "hashes", namespace, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}

	prefix := version + "_"
	suffix := ".h1"

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, suffix) {
			continue
		}

		// Extract platform from filename
		platform := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), suffix)

		data, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			continue
		}

		result[platform] = strings.TrimSpace(string(data))
	}

	return result
}

