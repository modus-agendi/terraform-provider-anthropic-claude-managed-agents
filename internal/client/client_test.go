package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(Config{APIKey: "sk-test", BaseURL: srv.URL, MaxRetries: 2})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNew_RequiresAPIKey(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error when APIKey is empty")
	}
}

func TestDo_SendsHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "sk-test" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != "managed-agents-2026-04-01" {
			t.Errorf("anthropic-beta = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var out map[string]any
	if err := c.do(context.Background(), http.MethodGet, "/v1/ping", nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
}

func TestDo_ReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("request-id", "req_xyz")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/v1/agents/agent_nope", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.RequestID != "req_xyz" {
		t.Errorf("RequestID = %q", apiErr.RequestID)
	}
}

func TestDo_RetriesOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var out map[string]any
	if err := c.do(context.Background(), http.MethodGet, "/v1/ping", nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDo_DoesNotRetryOn400(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodPost, "/v1/agents", map[string]string{"name": ""}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 400)", calls)
	}
}

func TestDo_BodyIsMarshalled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if got["name"] != "alice" {
			t.Errorf("body name = %v", got["name"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.do(context.Background(), http.MethodPost, "/v1/x", map[string]string{"name": "alice"}, nil); err != nil {
		t.Fatalf("do: %v", err)
	}
}

func TestRedactJSON_MasksSensitiveKeys(t *testing.T) {
	in := []byte(`{"name":"alice","api_key":"sk-xxx","auth":{"token":"abc","public":"ok"},"items":[{"client_secret":"s"}]}`)
	got := redactJSON(in)
	for _, banned := range []string{"sk-xxx", `"abc"`, `"s"`} {
		if strings.Contains(got, banned) {
			t.Errorf("redacted output still contains %q: %s", banned, got)
		}
	}
	for _, kept := range []string{"alice", "public"} {
		if !strings.Contains(got, kept) {
			t.Errorf("redacted output lost benign field %q: %s", kept, got)
		}
	}
}

func TestRedactJSON_InvalidJSONPassesThrough(t *testing.T) {
	in := []byte("not json")
	got := redactJSON(in)
	if got != "not json" {
		t.Errorf("redactJSON = %q", got)
	}
}
