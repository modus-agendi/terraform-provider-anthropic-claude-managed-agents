package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func vaultFixture() string {
	return `{
		"id":"vlt_FAKE0001",
		"type":"vault",
		"display_name":"Alice",
		"metadata":{"external_user_id":"usr_abc123"},
		"created_at":"2026-05-13T00:00:00Z",
		"updated_at":"2026-05-13T00:00:00Z",
		"archived_at":null
	}`
}

func TestCreateVault_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/vaults" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req VaultCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.DisplayName != "Alice" {
			t.Errorf("display_name = %q", req.DisplayName)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(vaultFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	v, err := c.CreateVault(context.Background(), VaultCreateRequest{
		DisplayName: "Alice",
		Metadata:    map[string]string{"external_user_id": "usr_abc123"},
	})
	if err != nil {
		t.Fatalf("CreateVault: %v", err)
	}
	if v.ID != "vlt_FAKE0001" {
		t.Errorf("ID = %q", v.ID)
	}
}

func TestCreateVault_RequiresDisplayName(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.CreateVault(context.Background(), VaultCreateRequest{}); err == nil {
		t.Error("expected error for empty display_name")
	}
}

func TestGetVault_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetVault(context.Background(), "nope")
	if !IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestUpdateVault_NameAndMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vaults/vlt_FAKE0001" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(vaultFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	name := "Alice (renamed)"
	_, err := c.UpdateVault(context.Background(), "vlt_FAKE0001", VaultUpdateRequest{
		DisplayName: &name,
		Metadata:    map[string]any{"new": "value"},
	})
	if err != nil {
		t.Fatalf("UpdateVault: %v", err)
	}
}

func TestArchiveDeleteVault_RequireID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.ArchiveVault(context.Background(), ""); err == nil {
		t.Error("ArchiveVault: expected error for empty id")
	}
	if err := c.DeleteVault(context.Background(), ""); err == nil {
		t.Error("DeleteVault: expected error for empty id")
	}
	if _, err := c.GetVault(context.Background(), ""); err == nil {
		t.Error("GetVault: expected error for empty id")
	}
	if _, err := c.UpdateVault(context.Background(), "", VaultUpdateRequest{}); err == nil {
		t.Error("UpdateVault: expected error for empty id")
	}
}

func TestArchiveVault_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/vaults/vlt_x/archive" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveVault(context.Background(), "vlt_x"); err != nil {
		t.Fatalf("ArchiveVault: %v", err)
	}
}

func TestDeleteVault_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/vaults/vlt_x" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteVault(context.Background(), "vlt_x"); err != nil {
		t.Fatalf("DeleteVault: %v", err)
	}
}

func TestListVaults_PassesQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "3" {
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
	_, err := c.ListVaults(context.Background(), ListVaultsParams{Limit: 3, IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListVaults: %v", err)
	}
}

func TestCreateVaultCredential_StaticBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vaults/vlt_x/credentials" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		if raw["display_name"] != "Linear API key" {
			t.Errorf("display_name = %v", raw["display_name"])
		}
		auth, _ := raw["auth"].(map[string]any)
		if auth["type"] != "static_bearer" {
			t.Errorf("auth.type = %v", auth["type"])
		}
		if auth["token"] != "lin_api_secret" {
			t.Errorf("auth.token not propagated")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"cred_FAKE0001",
			"type":"vault_credential",
			"vault_id":"vlt_x",
			"display_name":"Linear API key",
			"auth":{"type":"static_bearer","mcp_server_url":"https://mcp.linear.app/mcp"},
			"created_at":"2026-05-13T00:00:00Z",
			"updated_at":"2026-05-13T00:00:00Z",
			"archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	cred, err := c.CreateVaultCredential(context.Background(), "vlt_x", VaultCredentialCreateRequest{
		DisplayName: "Linear API key",
		Auth: map[string]any{
			"type":           "static_bearer",
			"mcp_server_url": "https://mcp.linear.app/mcp",
			"token":          "lin_api_secret",
		},
	})
	if err != nil {
		t.Fatalf("CreateVaultCredential: %v", err)
	}
	if cred.Auth.Type != "static_bearer" {
		t.Errorf("Auth.Type = %q", cred.Auth.Type)
	}
	if cred.Auth.McpServerURL != "https://mcp.linear.app/mcp" {
		t.Errorf("Auth.McpServerURL = %q", cred.Auth.McpServerURL)
	}
}

func TestVaultCredential_RequiresFields(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.CreateVaultCredential(context.Background(), "", VaultCredentialCreateRequest{}); err == nil {
		t.Error("expected error for empty vault_id")
	}
	if _, err := c.CreateVaultCredential(context.Background(), "vlt_x", VaultCredentialCreateRequest{}); err == nil {
		t.Error("expected error for empty display_name")
	}
	if _, err := c.CreateVaultCredential(context.Background(), "vlt_x", VaultCredentialCreateRequest{DisplayName: "x"}); err == nil {
		t.Error("expected error for nil auth")
	}
	if _, err := c.GetVaultCredential(context.Background(), "", "y"); err == nil {
		t.Error("expected error for empty vault_id")
	}
	if _, err := c.GetVaultCredential(context.Background(), "x", ""); err == nil {
		t.Error("expected error for empty credential id")
	}
	if _, err := c.UpdateVaultCredential(context.Background(), "x", "", VaultCredentialUpdateRequest{}); err == nil {
		t.Error("expected error for empty credential id on update")
	}
	if err := c.ArchiveVaultCredential(context.Background(), "", ""); err == nil {
		t.Error("expected error for empty ids on archive")
	}
	if err := c.DeleteVaultCredential(context.Background(), "", ""); err == nil {
		t.Error("expected error for empty ids on delete")
	}
	if _, err := c.ListVaultCredentials(context.Background(), "", ListVaultCredentialsParams{}); err == nil {
		t.Error("expected error for empty vault_id on list")
	}
}

func TestUpdateVaultCredential_Secret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vaults/vlt_x/credentials/cred_y" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"cred_y","type":"vault_credential","vault_id":"vlt_x",
			"display_name":"x",
			"auth":{"type":"static_bearer","mcp_server_url":"https://mcp.linear.app/mcp"},
			"created_at":"2026-05-13T00:00:00Z","updated_at":"2026-05-13T00:00:00Z","archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.UpdateVaultCredential(context.Background(), "vlt_x", "cred_y", VaultCredentialUpdateRequest{
		Auth: map[string]any{"type": "static_bearer", "token": "new"},
	})
	if err != nil {
		t.Fatalf("UpdateVaultCredential: %v", err)
	}
}

func TestArchiveDeleteVaultCredential_HappyPaths(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/vaults/vlt_x/credentials/cred_y/archive", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/v1/vaults/vlt_x/credentials/cred_y", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveVaultCredential(context.Background(), "vlt_x", "cred_y"); err != nil {
		t.Fatalf("ArchiveVaultCredential: %v", err)
	}
	if err := c.DeleteVaultCredential(context.Background(), "vlt_x", "cred_y"); err != nil {
		t.Fatalf("DeleteVaultCredential: %v", err)
	}
}

func TestListVaultCredentials_PassesQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vaults/vlt_x/credentials" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("limit = %q", r.URL.Query().Get("limit"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"has_more":false,"first_id":"","last_id":""}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.ListVaultCredentials(context.Background(), "vlt_x", ListVaultCredentialsParams{Limit: 5})
	if err != nil {
		t.Fatalf("ListVaultCredentials: %v", err)
	}
}
