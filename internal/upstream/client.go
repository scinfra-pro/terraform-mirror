package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents an HTTP client for requests to upstream registry
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new upstream client
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Get performs a GET request to upstream
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "terraform-mirror/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

// GetJSON performs a GET request and returns the response body
func (c *Client) GetJSON(ctx context.Context, path string) ([]byte, int, error) {
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	return body, resp.StatusCode, nil
}

