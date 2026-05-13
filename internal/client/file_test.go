package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFile_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/files/file_abc" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"file_abc","type":"file","filename":"report.csv","size_bytes":1234,
			"mime_type":"text/csv","scope_id":"sesn_abc","created_at":"2026-05-13T00:00:00Z"
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	f, err := c.GetFile(context.Background(), "file_abc")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if f.Filename != "report.csv" || f.SizeBytes != 1234 {
		t.Errorf("unexpected file: %+v", f)
	}
}

func TestGetFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetFile(context.Background(), "x")
	if !IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestGetFile_RequiresID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.GetFile(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
}
