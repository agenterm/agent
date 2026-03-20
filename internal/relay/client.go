package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agenterm/cli/internal/config"
)

// Client talks to the AgenTerm relay API.
type Client struct {
	BaseURL    string
	PushKey    string
	HTTPClient *http.Client
}

// NewClient creates a Client from the given config.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		BaseURL: strings.TrimRight(cfg.RelayURL, "/"),
		PushKey: cfg.PushKey,
		HTTPClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// Ping validates that the push key is accepted by the relay server.
// It sends a test POST /proposals request and treats HTTP 401 as invalid.
func (c *Client) Ping() error {
	resp, err := c.doRequest("POST", "/proposals", strings.NewReader(`{"type":"status","title":"__validate__","body":"__validate__"}`))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid push key (HTTP 401)")
	}
	return nil
}

// doRequest executes an HTTP request with Authorization header.
// body may be nil for requests without a body.
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.PushKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.PushKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	return resp, nil
}
