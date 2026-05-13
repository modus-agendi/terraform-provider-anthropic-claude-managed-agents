package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func envResponseFixture() string {
	return `{
		"id":"env_FAKE0001",
		"type":"environment",
		"name":"python-dev",
		"config": {
			"type":"cloud",
			"packages": {"pip":["pandas==2.2.0"]},
			"networking": {"type":"unrestricted"}
		},
		"created_at":"2026-05-13T00:00:00Z",
		"updated_at":"2026-05-13T00:00:00Z",
		"archived_at":null
	}`
}

func TestCreateEnvironment_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/environments" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req EnvironmentCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Name != "python-dev" {
			t.Errorf("name = %q", req.Name)
		}
		if req.Config.Networking.Type != "unrestricted" {
			t.Errorf("networking.type = %q", req.Config.Networking.Type)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(envResponseFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.CreateEnvironment(context.Background(), EnvironmentCreateRequest{
		Name: "python-dev",
		Config: CloudConfig{
			Type:       "cloud",
			Packages:   &Packages{Pip: []string{"pandas==2.2.0"}},
			Networking: Networking{Type: "unrestricted"},
		},
	})
	if err != nil {
		t.Fatalf("CreateEnvironment: %v", err)
	}
	if env.ID != "env_FAKE0001" {
		t.Errorf("ID = %q", env.ID)
	}
	if env.Config.Networking.Type != "unrestricted" {
		t.Errorf("Networking.Type = %q", env.Config.Networking.Type)
	}
	if env.Config.Packages == nil || len(env.Config.Packages.Pip) != 1 {
		t.Errorf("Packages.Pip not preserved")
	}
}

func TestCreateEnvironment_LimitedNetworking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req EnvironmentCreateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Config.Networking.Type != "limited" {
			t.Errorf("expected limited, got %q", req.Config.Networking.Type)
		}
		if len(req.Config.Networking.AllowedHosts) != 1 {
			t.Errorf("expected 1 allowed host, got %d", len(req.Config.Networking.AllowedHosts))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"env_LIMIT",
			"type":"environment",
			"name":"locked",
			"config":{"type":"cloud","networking":{"type":"limited","allowed_hosts":["https://api.example.com"],"allow_mcp_servers":false,"allow_package_managers":false}},
			"created_at":"2026-05-13T00:00:00Z","updated_at":"2026-05-13T00:00:00Z","archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	allowFalse := false
	_, err := c.CreateEnvironment(context.Background(), EnvironmentCreateRequest{
		Name: "locked",
		Config: CloudConfig{
			Type: "cloud",
			Networking: Networking{
				Type:                 "limited",
				AllowedHosts:         []string{"https://api.example.com"},
				AllowMcpServers:      &allowFalse,
				AllowPackageManagers: &allowFalse,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateEnvironment: %v", err)
	}
}

func TestCreateEnvironment_ValidatesRequired(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := map[string]EnvironmentCreateRequest{
		"empty name":            {Config: CloudConfig{Type: "cloud", Networking: Networking{Type: "unrestricted"}}},
		"empty config.type":     {Name: "x", Config: CloudConfig{Networking: Networking{Type: "unrestricted"}}},
		"empty networking.type": {Name: "x", Config: CloudConfig{Type: "cloud"}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := c.CreateEnvironment(context.Background(), req); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestGetEnvironment_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/environments/env_FAKE0001" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(envResponseFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.GetEnvironment(context.Background(), "env_FAKE0001")
	if err != nil {
		t.Fatalf("GetEnvironment: %v", err)
	}
	if env.Name != "python-dev" {
		t.Errorf("Name = %q", env.Name)
	}
}

func TestGetEnvironment_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetEnvironment(context.Background(), "env_nope")
	if !IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestArchiveEnvironment_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/environments/env_FAKE0001/archive" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveEnvironment(context.Background(), "env_FAKE0001"); err != nil {
		t.Fatalf("ArchiveEnvironment: %v", err)
	}
}

func TestDeleteEnvironment_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/environments/env_FAKE0001" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteEnvironment(context.Background(), "env_FAKE0001"); err != nil {
		t.Fatalf("DeleteEnvironment: %v", err)
	}
}

func TestDeleteEnvironment_ConflictWhenActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"conflict_error","message":"environment is referenced by active sessions"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.DeleteEnvironment(context.Background(), "env_x")
	if !IsConflict(err) {
		t.Errorf("want IsConflict, got %v", err)
	}
}

func TestArchiveDeleteEnvironment_RequireID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.ArchiveEnvironment(context.Background(), ""); err == nil {
		t.Error("ArchiveEnvironment: expected error for empty id")
	}
	if err := c.DeleteEnvironment(context.Background(), ""); err == nil {
		t.Error("DeleteEnvironment: expected error for empty id")
	}
	if _, err := c.GetEnvironment(context.Background(), ""); err == nil {
		t.Error("GetEnvironment: expected error for empty id")
	}
}

func TestListEnvironments_PassesQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "5" {
			t.Errorf("limit = %q", q.Get("limit"))
		}
		if q.Get("include_archived") != "true" {
			t.Errorf("include_archived = %q", q.Get("include_archived"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"has_more":false,"first_id":"","last_id":""}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.ListEnvironments(context.Background(), ListEnvironmentsParams{Limit: 5, IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	if resp.HasMore {
		t.Errorf("HasMore = true, want false")
	}
}
