package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateProposal_Success(t *testing.T) {
	// TC-P01: successful create returns ID and pending status
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Proposal{
			ID:     "p-abc",
			Status: "pending",
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	p, err := c.CreateProposal("test", "title", "body")
	if err != nil {
		t.Fatalf("CreateProposal error: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if p.Status != "pending" {
		t.Fatalf("Status = %q, want pending", p.Status)
	}
}

func TestCreateProposal_401(t *testing.T) {
	// TC-P02: 401 returns error containing "401"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "bad", HTTPClient: http.DefaultClient}
	_, err := c.CreateProposal("test", "title", "body")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want substring '401'", err.Error())
	}
}

func TestCreateProposal_WithMemory(t *testing.T) {
	// TC-P03: WithMemory option includes memory in request body
	var gotMemory *Memory
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Memory *Memory `json:"memory"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		gotMemory = req.Memory
		json.NewEncoder(w).Encode(Proposal{ID: "p-mem", Status: "pending"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	_, err := c.CreateProposal("test", "title", "body", WithMemory("remember this", "note"))
	if err != nil {
		t.Fatalf("CreateProposal error: %v", err)
	}
	if gotMemory == nil {
		t.Fatal("expected memory in request body")
	}
	if gotMemory.Content != "remember this" || gotMemory.Type != "note" {
		t.Fatalf("memory = %+v, want content='remember this' type='note'", gotMemory)
	}
}

func TestGetProposal_Success(t *testing.T) {
	// TC-P04: successful get returns correct proposal
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Proposal{
			ID:     "p-get",
			Type:   "approval",
			Title:  "test title",
			Status: "approved",
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	p, err := c.GetProposal("p-get")
	if err != nil {
		t.Fatalf("GetProposal error: %v", err)
	}
	if p.ID != "p-get" || p.Status != "approved" || p.Title != "test title" {
		t.Fatalf("GetProposal = %+v, unexpected", p)
	}
}

func TestGetProposal_404(t *testing.T) {
	// TC-P05: 404 returns error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	_, err := c.GetProposal("nonexistent")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want substring '404'", err.Error())
	}
}

func TestCreateProposal_Disabled(t *testing.T) {
	// TC-P07: server returns disabled status → ErrGateDisabled
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Proposal{
			ID:     "disabled",
			Status: "disabled",
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	p, err := c.CreateProposal("test", "title", "body")
	if err == nil {
		t.Fatal("expected error for disabled gate")
	}
	if err != ErrGateDisabled {
		t.Fatalf("expected ErrGateDisabled, got %v", err)
	}
	if p.ID != "disabled" || p.Status != "disabled" {
		t.Fatalf("expected disabled proposal, got %+v", p)
	}
}

func TestWaitForProposal_ImmediateApproval(t *testing.T) {
	// TC-P06: first poll returns approved → returns immediately
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Proposal{
			ID:     "p-wait",
			Status: "approved",
		})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	p, err := c.WaitForProposal("p-wait", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForProposal error: %v", err)
	}
	if p.Status != "approved" {
		t.Fatalf("Status = %q, want approved", p.Status)
	}
}
