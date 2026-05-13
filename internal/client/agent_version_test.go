package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAgentVersions_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agents/agent_x/versions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data":[
				{"type":"agent_version","agent_id":"agent_x","version":1,"name":"v1","model":{"id":"claude-opus-4-7"},"system":null,"description":null,"metadata":{},"created_at":"2026-05-13T00:00:00Z","updated_at":"2026-05-13T00:00:00Z"},
				{"type":"agent_version","agent_id":"agent_x","version":2,"name":"v2","model":{"id":"claude-opus-4-7"},"system":null,"description":null,"metadata":{},"created_at":"2026-05-13T00:00:00Z","updated_at":"2026-05-13T00:00:00Z"}
			],
			"has_more":false,"first_id":"","last_id":""
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.ListAgentVersions(context.Background(), "agent_x", ListAgentVersionsParams{Limit: 100})
	if err != nil {
		t.Fatalf("ListAgentVersions: %v", err)
	}
	if len(page.Data) != 2 || page.Data[1].Version != 2 {
		t.Errorf("unexpected page: %+v", page)
	}
}

func TestGetAgentVersion_FindsMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data":[
				{"type":"agent_version","agent_id":"agent_x","version":1,"name":"v1","model":{"id":"claude-opus-4-7"},"system":null,"description":null,"metadata":{},"created_at":"2026-05-13T00:00:00Z","updated_at":"2026-05-13T00:00:00Z"},
				{"type":"agent_version","agent_id":"agent_x","version":2,"name":"v2","model":{"id":"claude-opus-4-7"},"system":null,"description":null,"metadata":{},"created_at":"2026-05-13T00:00:00Z","updated_at":"2026-05-13T00:00:00Z"}
			],
			"has_more":false,"first_id":"","last_id":""
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	v, err := c.GetAgentVersion(context.Background(), "agent_x", 2)
	if err != nil {
		t.Fatalf("GetAgentVersion: %v", err)
	}
	if v.Version != 2 {
		t.Errorf("Version = %d", v.Version)
	}
}

func TestGetAgentVersion_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"has_more":false,"first_id":"","last_id":""}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetAgentVersion(context.Background(), "agent_x", 7)
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestGetAgentVersion_Validation(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.GetAgentVersion(context.Background(), "", 1); err == nil {
		t.Error("expected error for empty agent_id")
	}
	if _, err := c.GetAgentVersion(context.Background(), "x", 0); err == nil {
		t.Error("expected error for non-positive version")
	}
}
