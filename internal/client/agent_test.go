package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateAgent_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/agents" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req AgentCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Name != "Test" || req.Model != "claude-opus-4-7" {
			t.Errorf("body = %+v", req)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"agent_01ABC","type":"agent","name":"Test",
			"model":{"id":"claude-opus-4-7","speed":"standard"},
			"system":null,"description":null,
			"metadata":{},"version":1,
			"created_at":"2026-04-03T18:24:10.412Z",
			"updated_at":"2026-04-03T18:24:10.412Z",
			"archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	agent, err := c.CreateAgent(context.Background(), AgentCreateRequest{Name: "Test", Model: "claude-opus-4-7"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if agent.ID != "agent_01ABC" {
		t.Errorf("ID = %q", agent.ID)
	}
	if agent.Model.ID != "claude-opus-4-7" {
		t.Errorf("Model.ID = %q", agent.Model.ID)
	}
	if agent.Version != 1 {
		t.Errorf("Version = %d", agent.Version)
	}
}

func TestCreateAgent_ValidatesRequired(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.CreateAgent(context.Background(), AgentCreateRequest{Model: "x"}); err == nil {
		t.Error("expected error for empty Name")
	}
	if _, err := c.CreateAgent(context.Background(), AgentCreateRequest{Name: "x"}); err == nil {
		t.Error("expected error for empty Model")
	}
}

func TestGetAgent_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/agents/agent_01ABC" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"agent_01ABC","type":"agent","name":"Test",
			"model":{"id":"claude-opus-4-7","speed":"standard"},
			"system":"Be helpful.","description":null,
			"metadata":{"team":"platform"},"version":3,
			"created_at":"2026-04-03T18:24:10.412Z",
			"updated_at":"2026-04-04T11:00:00.000Z",
			"archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	agent, err := c.GetAgent(context.Background(), "agent_01ABC")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.System == nil || *agent.System != "Be helpful." {
		t.Errorf("System = %v", agent.System)
	}
	if agent.Metadata["team"] != "platform" {
		t.Errorf("Metadata = %v", agent.Metadata)
	}
	if agent.Version != 3 {
		t.Errorf("Version = %d", agent.Version)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetAgent(context.Background(), "agent_nope")
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestUpdateAgent_SendsVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/agents/agent_01ABC" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if v, _ := body["version"].(float64); v != 3 {
			t.Errorf("version = %v, want 3", body["version"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"agent_01ABC","type":"agent","name":"Test",
			"model":{"id":"claude-opus-4-7","speed":"standard"},
			"system":null,"description":null,
			"metadata":{},"version":4,
			"created_at":"2026-04-03T18:24:10.412Z",
			"updated_at":"2026-04-05T11:00:00.000Z",
			"archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	name := "Renamed"
	agent, err := c.UpdateAgent(context.Background(), "agent_01ABC", AgentUpdateRequest{Version: 3, Name: &name})
	if err != nil {
		t.Fatalf("UpdateAgent: %v", err)
	}
	if agent.Version != 4 {
		t.Errorf("Version = %d, want 4 (incremented)", agent.Version)
	}
}

func TestUpdateAgent_ValidatesRequired(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.UpdateAgent(context.Background(), "", AgentUpdateRequest{Version: 1}); err == nil {
		t.Error("expected error for empty id")
	}
	if _, err := c.UpdateAgent(context.Background(), "agent_x", AgentUpdateRequest{Version: 0}); err == nil {
		t.Error("expected error for zero version")
	}
}

func TestArchiveAgent_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/agents/agent_01ABC/archive" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveAgent(context.Background(), "agent_01ABC"); err != nil {
		t.Fatalf("ArchiveAgent: %v", err)
	}
}

func TestListAgents_PassesQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "10" {
			t.Errorf("limit = %q", q.Get("limit"))
		}
		if q.Get("after_id") != "agent_cursor" {
			t.Errorf("after_id = %q", q.Get("after_id"))
		}
		if q.Get("include_archived") != "true" {
			t.Errorf("include_archived = %q", q.Get("include_archived"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"has_more":false,"first_id":"","last_id":""}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.ListAgents(context.Background(), ListAgentsParams{Limit: 10, AfterID: "agent_cursor", IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if resp.HasMore {
		t.Errorf("HasMore = true, want false")
	}
}

func TestListAgents_PaginationLoop(t *testing.T) {
	// Server returns two pages: the first sets has_more=true with last_id="agent_FIRST10",
	// then on the cursor-second call returns has_more=false. Verifies the caller can
	// loop until exhaustion using the LastID field.
	page1 := `{
		"data": [{"id":"agent_FIRST1","name":"a","model":{"id":"x"},"version":1,"metadata":{}, "created_at":"2026-05-13T00:00:00Z", "updated_at":"2026-05-13T00:00:00Z", "archived_at":null}],
		"has_more": true,
		"first_id": "agent_FIRST1",
		"last_id":  "agent_FIRST10"
	}`
	page2 := `{
		"data": [{"id":"agent_SECOND1","name":"b","model":{"id":"x"},"version":1,"metadata":{}, "created_at":"2026-05-13T00:00:00Z", "updated_at":"2026-05-13T00:00:00Z", "archived_at":null}],
		"has_more": false,
		"first_id": "agent_SECOND1",
		"last_id":  "agent_SECOND10"
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("after_id") == "agent_FIRST10" {
			_, _ = w.Write([]byte(page2))
			return
		}
		_, _ = w.Write([]byte(page1))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var all []Agent
	cursor := ""
	for {
		page, err := c.ListAgents(context.Background(), ListAgentsParams{Limit: 100, AfterID: cursor})
		if err != nil {
			t.Fatalf("ListAgents: %v", err)
		}
		all = append(all, page.Data...)
		if !page.HasMore {
			break
		}
		cursor = page.LastID
	}

	if len(all) != 2 {
		t.Fatalf("got %d agents, want 2", len(all))
	}
	if all[0].ID != "agent_FIRST1" || all[1].ID != "agent_SECOND1" {
		t.Errorf("ids = [%s, %s]", all[0].ID, all[1].ID)
	}
}

func TestArchiveAgent_404TreatedAsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.ArchiveAgent(context.Background(), "agent_gone")
	if !IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestArchiveAgent_RequiresID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.ArchiveAgent(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
}
