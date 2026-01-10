package upstream

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// Client represents an HTTP client for requests to upstream registry
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new upstream client
// If socks5Addr is empty, direct connection is used
// If socks5Addr is provided (e.g., "127.0.0.1:1080"), SOCKS5 proxy is used
func New(baseURL string, timeout time.Duration, socks5Addr string) (*Client, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Configure SOCKS5 proxy if provided
	if socks5Addr != "" {
		dialer, err := proxy.SOCKS5("tcp", socks5Addr, nil, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("creating SOCKS5 dialer: %w", err)
		}

		// Use DialContext if available, otherwise wrap Dial
		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			// Wrap Dial in DialContext for compatibility
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}, nil
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

