package config

import (
	"os"
	"time"
)

// Config holds application settings
type Config struct {
	// Server
	ListenAddr   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Upstream
	UpstreamURL     string
	UpstreamTimeout time.Duration

	// Cache
	CacheEnabled bool
	CacheDir     string

	// Logging
	LogLevel string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		ListenAddr:      getEnv("TF_MIRROR_LISTEN", ":8080"),
		ReadTimeout:     getDurationEnv("TF_MIRROR_READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    getDurationEnv("TF_MIRROR_WRITE_TIMEOUT", 300*time.Second),
		UpstreamURL:     getEnv("TF_MIRROR_UPSTREAM_URL", "https://registry.terraform.io"),
		UpstreamTimeout: getDurationEnv("TF_MIRROR_UPSTREAM_TIMEOUT", 60*time.Second),
		CacheEnabled:    getBoolEnv("TF_MIRROR_CACHE_ENABLED", true),
		CacheDir:        getEnv("TF_MIRROR_CACHE_DIR", "./cache"),
		LogLevel:        getEnv("TF_MIRROR_LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

