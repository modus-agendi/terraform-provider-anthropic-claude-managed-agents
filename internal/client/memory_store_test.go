package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func memoryStoreFixture() string {
	return `{
		"id":"memstore_FAKE0001",
		"type":"memory_store",
		"name":"prefs",
		"description":"Per-user preferences",
		"created_at":"2026-05-13T00:00:00Z",
		"updated_at":"2026-05-13T00:00:00Z",
		"archived_at":null
	}`
}

func TestCreateMemoryStore_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/memory_stores" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req MemoryStoreCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Name != "prefs" {
			t.Errorf("name = %q", req.Name)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(memoryStoreFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	desc := "Per-user preferences"
	ms, err := c.CreateMemoryStore(context.Background(), MemoryStoreCreateRequest{
		Name:        "prefs",
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("CreateMemoryStore: %v", err)
	}
	if ms.ID != "memstore_FAKE0001" {
		t.Errorf("ID = %q", ms.ID)
	}
	if ms.Description == nil || *ms.Description != "Per-user preferences" {
		t.Errorf("Description = %v", ms.Description)
	}
}

func TestCreateMemoryStore_RequiresName(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.CreateMemoryStore(context.Background(), MemoryStoreCreateRequest{}); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestGetMemoryStore_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/memory_stores/memstore_FAKE0001" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(memoryStoreFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ms, err := c.GetMemoryStore(context.Background(), "memstore_FAKE0001")
	if err != nil {
		t.Fatalf("GetMemoryStore: %v", err)
	}
	if ms.Name != "prefs" {
		t.Errorf("Name = %q", ms.Name)
	}
}

func TestGetMemoryStore_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetMemoryStore(context.Background(), "nope")
	if !IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestUpdateMemoryStore_NameAndDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memory_stores/memstore_FAKE0001" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		if raw["name"] != "renamed" {
			t.Errorf("name = %v", raw["name"])
		}
		if raw["description"] != "new desc" {
			t.Errorf("description = %v", raw["description"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(memoryStoreFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	name := "renamed"
	_, err := c.UpdateMemoryStore(context.Background(), "memstore_FAKE0001", MemoryStoreUpdateRequest{
		Name:        &name,
		Description: json.RawMessage(`"new desc"`),
	})
	if err != nil {
		t.Fatalf("UpdateMemoryStore: %v", err)
	}
}

func TestUpdateMemoryStore_RequiresID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.UpdateMemoryStore(context.Background(), "", MemoryStoreUpdateRequest{}); err == nil {
		t.Error("expected error for empty id")
	}
}

func TestArchiveMemoryStore_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/memory_stores/memstore_FAKE0001/archive" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveMemoryStore(context.Background(), "memstore_FAKE0001"); err != nil {
		t.Fatalf("ArchiveMemoryStore: %v", err)
	}
}

func TestDeleteMemoryStore_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/memory_stores/memstore_FAKE0001" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.DeleteMemoryStore(context.Background(), "memstore_FAKE0001"); err != nil {
		t.Fatalf("DeleteMemoryStore: %v", err)
	}
}

func TestArchiveDeleteMemoryStore_RequireID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.ArchiveMemoryStore(context.Background(), ""); err == nil {
		t.Error("ArchiveMemoryStore: expected error for empty id")
	}
	if err := c.DeleteMemoryStore(context.Background(), ""); err == nil {
		t.Error("DeleteMemoryStore: expected error for empty id")
	}
	if _, err := c.GetMemoryStore(context.Background(), ""); err == nil {
		t.Error("GetMemoryStore: expected error for empty id")
	}
}

func TestListMemoryStores_PassesQueryParams(t *testing.T) {
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
	resp, err := c.ListMemoryStores(context.Background(), ListMemoryStoresParams{Limit: 5, IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListMemoryStores: %v", err)
	}
	if resp.HasMore {
		t.Errorf("HasMore = true, want false")
	}
}
