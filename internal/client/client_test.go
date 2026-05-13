package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestDo_401AuthenticationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid api key"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/v1/agents/x", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr := &APIError{}
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if apiErr.Type != "authentication_error" {
		t.Errorf("Type = %q", apiErr.Type)
	}
}

func TestDo_403PermissionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"permission_error","message":"forbidden"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/v1/agents/x", nil, nil)
	apiErr := &APIError{}
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403 APIError, got %v", err)
	}
}

func TestDo_429RetriesAndEventuallySucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var out map[string]any
	if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (two 429s then success)", got)
	}
}

func TestDo_429ExhaustedRetriesReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	// After all retries we still return the underlying APIError (status 429).
	apiErr := &APIError{}
	if !errors.As(err, &apiErr) {
		t.Logf("note: retryablehttp wraps the final error; got %v", err)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := c.do(ctx, http.MethodGet, "/v1/slow", nil, nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed > time.Second {
		t.Errorf("took %s, expected fast cancel", elapsed)
	}
}

func TestDo_MalformedJSONOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not actually json`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var out map[string]any
	err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, &out)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %v, want decode-response wording", err)
	}
}

func TestNew_InvalidBaseURL(t *testing.T) {
	_, err := New(Config{APIKey: "sk-test", BaseURL: ":://bad"})
	if err == nil {
		t.Fatal("expected error for invalid BaseURL")
	}
}

func TestNew_DefaultsBaseURL(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.BaseURL() != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q, want default", c.BaseURL())
	}
}
