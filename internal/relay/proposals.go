package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// ErrGateDisabled is returned when the gate takeover is not enabled for this user.
var ErrGateDisabled = errors.New("gate disabled: takeover not enabled")

// Proposal represents a proposal in the relay system.
type Proposal struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Body        string     `json:"body,omitempty"`
	Memory      *Memory    `json:"memory,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`
}

// Memory is optional memory attached to a proposal.
type Memory struct {
	Content string `json:"content"`
	Type    string `json:"type"`
}

type createRequest struct {
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Body      string  `json:"body,omitempty"`
	Memory    *Memory `json:"memory,omitempty"`
	Blocking  *bool   `json:"blocking,omitempty"`
	ExpiresIn *int    `json:"expires_in,omitempty"`
}

// CreateOption configures optional fields on CreateProposal.
type CreateOption func(*createRequest)

// WithMemory attaches memory content to the proposal.
func WithMemory(content, mtype string) CreateOption {
	return func(r *createRequest) {
		r.Memory = &Memory{Content: content, Type: mtype}
	}
}

// WithBlocking sets the blocking flag on the proposal.
func WithBlocking(blocking bool) CreateOption {
	return func(r *createRequest) {
		r.Blocking = &blocking
	}
}

// WithExpiresIn sets the expiration time in seconds.
func WithExpiresIn(seconds int) CreateOption {
	return func(r *createRequest) {
		r.ExpiresIn = &seconds
	}
}

// CreateProposal submits a new proposal to the relay.
func (c *Client) CreateProposal(pType, title, body string, opts ...CreateOption) (*Proposal, error) {
	req := createRequest{
		Type:  pType,
		Title: title,
		Body:  body,
	}
	for _, o := range opts {
		o(&req)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	resp, err := c.doRequest("POST", "/proposals", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create proposal: HTTP %d: %s", resp.StatusCode, string(b))
	}

	var proposal Proposal
	if err := json.NewDecoder(resp.Body).Decode(&proposal); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if proposal.Status == "disabled" {
		return &proposal, ErrGateDisabled
	}
	return &proposal, nil
}

// GetProposal retrieves a proposal by ID.
func (c *Client) GetProposal(id string) (*Proposal, error) {
	resp, err := c.doRequest("GET", "/proposals/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get proposal: HTTP %d: %s", resp.StatusCode, string(b))
	}

	var proposal Proposal
	if err := json.NewDecoder(resp.Body).Decode(&proposal); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &proposal, nil
}

// WaitForProposal long-polls until the proposal reaches a terminal status or the timeout elapses.
func (c *Client) WaitForProposal(id string, timeout time.Duration) (*Proposal, error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timeout waiting for proposal %s", id)
		}

		// Each poll asks the server to hold the connection up to 30s
		pollTimeout := 30
		if int(remaining.Seconds()) < pollTimeout {
			pollTimeout = int(remaining.Seconds())
			if pollTimeout < 1 {
				pollTimeout = 1
			}
		}

		path := fmt.Sprintf("/proposals/%s?timeout=%d", id, pollTimeout)
		resp, err := c.doRequest("GET", path, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 204 {
			resp.Body.Close()
			continue
		}

		var proposal Proposal
		err = json.NewDecoder(resp.Body).Decode(&proposal)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		switch proposal.Status {
		case "approved", "remembered", "denied", "dismissed", "expired":
			return &proposal, nil
		}
		// Status is still pending — loop and poll again
	}
}
