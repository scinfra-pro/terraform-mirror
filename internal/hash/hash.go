package hash

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/mod/sumdb/dirhash"
)

// CalculateH1 calculates h1 hash for a provider ZIP file
// Uses the same algorithm as Terraform
// See https://github.com/hashicorp/terraform/blob/main/internal/getproviders/hash.go
func CalculateH1(zipPath string) (string, error) {
	return dirhash.HashZip(zipPath, dirhash.Hash1)
}

// CalculateH1FromReader calculates h1 hash by saving data to a temporary file
func CalculateH1FromReader(r io.Reader) (string, error) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "provider-*.zip")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy data
	if _, err := io.Copy(tmpFile, r); err != nil {
		return "", fmt.Errorf("writing temp file: %w", err)
	}

	// Close for reading
	tmpFile.Close()

	// Calculate hash
	return CalculateH1(tmpFile.Name())
}

