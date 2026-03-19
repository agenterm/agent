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
