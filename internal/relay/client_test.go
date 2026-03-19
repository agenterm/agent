package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoRequest_AuthHeader(t *testing.T) {
	// TC-H01: request carries Authorization: Bearer {key}
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "secret-key", HTTPClient: http.DefaultClient}
	resp, err := c.doRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer secret-key" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer secret-key")
	}
}

func TestDoRequest_PostContentType(t *testing.T) {
	// TC-H02: POST with body sets Content-Type: application/json
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	resp, err := c.doRequest("POST", "/test", strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	resp.Body.Close()

	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "application/json")
	}
}

func TestDoRequest_GetNoContentType(t *testing.T) {
	// TC-H03: GET without body has no Content-Type header
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "k", HTTPClient: http.DefaultClient}
	resp, err := c.doRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	resp.Body.Close()

	if gotCT != "" {
		t.Fatalf("Content-Type = %q, want empty", gotCT)
	}
}

func TestDoRequest_NoPushKeyNoAuth(t *testing.T) {
	// TC-H04: empty PushKey → no Authorization header
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, PushKey: "", HTTPClient: http.DefaultClient}
	resp, err := c.doRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth)
	}
}
