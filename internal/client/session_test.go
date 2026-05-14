package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateSession_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sessions" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("anthropic-beta"); got != "managed-agents-2026-04-01" {
			t.Errorf("anthropic-beta = %q, want managed-agents-2026-04-01", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["agent"] != "agent_01ABC" {
			t.Errorf("body.agent = %v", body["agent"])
		}
		if _, ok := body["environment_id"]; ok {
			t.Errorf("environment_id should be omitted when empty, got %v", body["environment_id"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"sesn_01XYZ","type":"session","agent_id":"agent_01ABC",
			"status":"idle",
			"created_at":"2026-05-14T10:00:00Z",
			"updated_at":"2026-05-14T10:00:00Z",
			"archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	sess, err := c.CreateSession(context.Background(), SessionCreateRequest{AgentID: "agent_01ABC"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID != "sesn_01XYZ" {
		t.Errorf("ID = %q", sess.ID)
	}
	if sess.Status != "idle" {
		t.Errorf("Status = %q", sess.Status)
	}
}

func TestCreateSession_WithEnvironmentAndVaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["agent"] != "agent_01ABC" {
			t.Errorf("agent = %v", body["agent"])
		}
		if body["environment_id"] != "env_01ENV" {
			t.Errorf("environment_id = %v", body["environment_id"])
		}
		vaultIDs, ok := body["vault_ids"].([]any)
		if !ok || len(vaultIDs) != 2 || vaultIDs[0] != "vault_01A" || vaultIDs[1] != "vault_01B" {
			t.Errorf("vault_ids = %v", body["vault_ids"])
		}
		if body["title"] != "trial run" {
			t.Errorf("title = %v", body["title"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"sesn_01XYZ","type":"session","agent_id":"agent_01ABC",
			"environment_id":"env_01ENV","status":"idle","title":"trial run",
			"created_at":"2026-05-14T10:00:00Z",
			"updated_at":"2026-05-14T10:00:00Z",
			"archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	sess, err := c.CreateSession(context.Background(), SessionCreateRequest{
		AgentID:       "agent_01ABC",
		EnvironmentID: "env_01ENV",
		VaultIDs:      []string{"vault_01A", "vault_01B"},
		Title:         "trial run",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.EnvironmentID == nil || *sess.EnvironmentID != "env_01ENV" {
		t.Errorf("EnvironmentID = %v", sess.EnvironmentID)
	}
	if sess.Title == nil || *sess.Title != "trial run" {
		t.Errorf("Title = %v", sess.Title)
	}
}

func TestCreateSession_ValidatesAgentID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.CreateSession(context.Background(), SessionCreateRequest{}); err == nil {
		t.Error("expected error for empty AgentID")
	}
}

func TestGetSession_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/sessions/sesn_01XYZ" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("anthropic-beta"); got != "managed-agents-2026-04-01" {
			t.Errorf("anthropic-beta = %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"sesn_01XYZ","type":"session","agent_id":"agent_01ABC",
			"status":"idle",
			"usage":{"input_tokens":120,"output_tokens":340,"cache_creation_input_tokens":50,"cache_read_input_tokens":1000},
			"created_at":"2026-05-14T10:00:00Z",
			"updated_at":"2026-05-14T10:05:00Z",
			"archived_at":null
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	sess, err := c.GetSession(context.Background(), "sesn_01XYZ")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.Usage == nil || sess.Usage.OutputTokens != 340 {
		t.Errorf("Usage = %+v", sess.Usage)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetSession(context.Background(), "sesn_missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestGetSession_ValidatesID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.GetSession(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
}

func TestArchiveSession_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sessions/sesn_01XYZ/archive" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("anthropic-beta"); got != "managed-agents-2026-04-01" {
			t.Errorf("anthropic-beta = %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveSession(context.Background(), "sesn_01XYZ"); err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}
}

func TestArchiveSession_ValidatesID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.ArchiveSession(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
}

func TestPostSessionEvents_WrapsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sessions/sesn_01XYZ/events" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("anthropic-beta"); got != "managed-agents-2026-04-01" {
			t.Errorf("anthropic-beta = %q", got)
		}
		var body struct {
			Events []map[string]any `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(body.Events) != 1 {
			t.Fatalf("len(events) = %d", len(body.Events))
		}
		ev := body.Events[0]
		if ev["type"] != "user.message" {
			t.Errorf("event.type = %v", ev["type"])
		}
		content, ok := ev["content"].([]any)
		if !ok || len(content) != 1 {
			t.Fatalf("content = %v", ev["content"])
		}
		block, _ := content[0].(map[string]any)
		if block["type"] != "text" || block["text"] != "hello" {
			t.Errorf("content[0] = %v", block)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.PostSessionEvents(context.Background(), "sesn_01XYZ", []UserEvent{{
		Type: "user.message",
		Content: []EventContent{{
			Type: "text",
			Text: "hello",
		}},
	}})
	if err != nil {
		t.Fatalf("PostSessionEvents: %v", err)
	}
}

func TestPostSessionEvents_Validates(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.PostSessionEvents(context.Background(), "", []UserEvent{{Type: "user.message"}}); err == nil {
		t.Error("expected error for empty session_id")
	}
	if err := c.PostSessionEvents(context.Background(), "sesn_x", nil); err == nil {
		t.Error("expected error for empty events")
	}
}

func TestListSessionEvents_NoParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/sessions/sesn_01XYZ/events" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("RawQuery = %q, want empty", r.URL.RawQuery)
		}
		if got := r.Header.Get("anthropic-beta"); got != "managed-agents-2026-04-01" {
			t.Errorf("anthropic-beta = %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"evt_01","type":"session.status_running","processed_at":"2026-05-14T10:00:01Z"},
				{"id":"evt_02","type":"agent.message","processed_at":"2026-05-14T10:00:02Z","content":[{"type":"text","text":"hi"}]}
			],
			"has_more":true,"first_id":"evt_01","last_id":"evt_02"
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.ListSessionEvents(context.Background(), "sesn_01XYZ", ListSessionEventsParams{})
	if err != nil {
		t.Fatalf("ListSessionEvents: %v", err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("len(Data) = %d", len(page.Data))
	}
	if !page.HasMore || page.FirstID != "evt_01" || page.LastID != "evt_02" {
		t.Errorf("pagination = %+v", page)
	}
	if page.Data[0].Type != "session.status_running" {
		t.Errorf("Data[0].Type = %q", page.Data[0].Type)
	}
}

func TestListSessionEvents_CreatedAfterAndTypes(t *testing.T) {
	// CreatedAfter is encoded as created_at[gt]=<RFC3339Nano>; the
	// upstream events endpoint does NOT support an id-based cursor.
	want := time.Date(2026, 5, 14, 10, 0, 5, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("created_at[gt]"); got != want.Format(time.RFC3339Nano) {
			t.Errorf("created_at[gt] = %q, want %q", got, want.Format(time.RFC3339Nano))
		}
		if got := q.Get("limit"); got != "50" {
			t.Errorf("limit = %q, want 50", got)
		}
		types := q["types[]"]
		if len(types) != 2 || types[0] != "agent.message" || types[1] != "session.status_idle" {
			t.Errorf("types[] = %v", types)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"has_more":false,"first_id":"","last_id":""}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.ListSessionEvents(context.Background(), "sesn_01XYZ", ListSessionEventsParams{
		CreatedAfter: want,
		Types:        []string{"agent.message", "session.status_idle"},
		Limit:        50,
	})
	if err != nil {
		t.Fatalf("ListSessionEvents: %v", err)
	}
	if len(page.Data) != 0 {
		t.Errorf("Data = %v", page.Data)
	}
}

func TestListSessionEvents_ValidatesID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.ListSessionEvents(context.Background(), "", ListSessionEventsParams{}); err == nil {
		t.Error("expected error for empty session_id")
	}
}

func TestSessionEvent_UnmarshalJSON_PreservesRawData(t *testing.T) {
	body := []byte(`{
		"id":"evt_10",
		"type":"agent.message",
		"processed_at":"2026-05-14T10:00:05Z",
		"content":[{"type":"text","text":"hello world"}]
	}`)
	var ev SessionEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.ID != "evt_10" || ev.Type != "agent.message" {
		t.Errorf("ev = %+v", ev)
	}
	if ev.ProcessedAt == nil {
		t.Fatal("ProcessedAt is nil")
	}
	want := time.Date(2026, 5, 14, 10, 0, 5, 0, time.UTC)
	if !ev.ProcessedAt.Equal(want) {
		t.Errorf("ProcessedAt = %v, want %v", ev.ProcessedAt, want)
	}
	// RawData should preserve the original bytes (re-unmarshal should work).
	var roundtrip map[string]any
	if err := json.Unmarshal(ev.RawData, &roundtrip); err != nil {
		t.Fatalf("roundtrip RawData: %v", err)
	}
	if roundtrip["id"] != "evt_10" {
		t.Errorf("roundtrip id = %v", roundtrip["id"])
	}
}

func TestSessionEvent_UnmarshalJSON_InvalidReturnsError(t *testing.T) {
	var ev SessionEvent
	if err := ev.UnmarshalJSON([]byte("not json")); err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestSessionEvent_AgentMessageText_ConcatenatesTextBlocks(t *testing.T) {
	ev := mustEvent(t, `{
		"id":"evt_1","type":"agent.message",
		"content":[
			{"type":"text","text":"Hello "},
			{"type":"tool_use","name":"bash"},
			{"type":"text","text":"world."}
		]
	}`)
	got, err := ev.AgentMessageText()
	if err != nil {
		t.Fatalf("AgentMessageText: %v", err)
	}
	if got != "Hello world." {
		t.Errorf("got %q", got)
	}
}

func TestSessionEvent_AgentMessageText_MissingContent(t *testing.T) {
	ev := mustEvent(t, `{"id":"evt_1","type":"agent.message"}`)
	got, err := ev.AgentMessageText()
	if err != nil {
		t.Fatalf("AgentMessageText: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSessionEvent_AgentMessageText_MalformedRawData(t *testing.T) {
	ev := SessionEvent{ID: "evt_1", Type: "agent.message", RawData: []byte("not json")}
	if _, err := ev.AgentMessageText(); err == nil {
		t.Error("expected error on malformed RawData")
	}
}

func TestSessionEvent_ToolUseName(t *testing.T) {
	ev := mustEvent(t, `{"id":"evt_1","type":"agent.tool_use","name":"code_execution"}`)
	name, err := ev.ToolUseName()
	if err != nil {
		t.Fatalf("ToolUseName: %v", err)
	}
	if name != "code_execution" {
		t.Errorf("got %q", name)
	}
}

func TestSessionEvent_ToolUseName_MalformedRawData(t *testing.T) {
	ev := SessionEvent{RawData: []byte("not json")}
	if _, err := ev.ToolUseName(); err == nil {
		t.Error("expected error")
	}
}

func TestSessionEvent_StopReasonType(t *testing.T) {
	ev := mustEvent(t, `{
		"id":"evt_1","type":"session.status_idle",
		"stop_reason":{"type":"end_turn","event_ids":["evt_a","evt_b"]}
	}`)
	got, err := ev.StopReasonType()
	if err != nil {
		t.Fatalf("StopReasonType: %v", err)
	}
	if got != "end_turn" {
		t.Errorf("got %q", got)
	}
}

func TestSessionEvent_StopReasonType_Missing(t *testing.T) {
	ev := mustEvent(t, `{"id":"evt_1","type":"session.status_idle"}`)
	got, err := ev.StopReasonType()
	if err != nil {
		t.Fatalf("StopReasonType: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSessionEvent_StopReasonType_MalformedRawData(t *testing.T) {
	ev := SessionEvent{RawData: []byte("not json")}
	if _, err := ev.StopReasonType(); err == nil {
		t.Error("expected error")
	}
}

func TestSessionEvent_ErrorMessage(t *testing.T) {
	ev := mustEvent(t, `{
		"id":"evt_1","type":"session.error",
		"error":{"type":"transient","message":"upstream timed out"}
	}`)
	got, err := ev.ErrorMessage()
	if err != nil {
		t.Fatalf("ErrorMessage: %v", err)
	}
	if got != "upstream timed out" {
		t.Errorf("got %q", got)
	}
}

func TestSessionEvent_ErrorMessage_MalformedRawData(t *testing.T) {
	ev := SessionEvent{RawData: []byte("not json")}
	if _, err := ev.ErrorMessage(); err == nil {
		t.Error("expected error")
	}
}

func TestPostSessionEvents_BetaHeaderPresent(t *testing.T) {
	// Already covered transitively above; this dedicated test asserts the
	// beta header for the events endpoint specifically.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("anthropic-beta"); got != "managed-agents-2026-04-01" {
			t.Errorf("anthropic-beta = %q", got)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/sessions/") {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.PostSessionEvents(context.Background(), "sesn_x", []UserEvent{{Type: "user.message"}}); err != nil {
		t.Fatalf("PostSessionEvents: %v", err)
	}
}

func TestSession_ContextCancellation(t *testing.T) {
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

	_, err := c.GetSession(ctx, "sesn_slow")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

// mustEvent decodes a JSON literal as a SessionEvent for use in helper tests.
func mustEvent(t *testing.T, payload string) SessionEvent {
	t.Helper()
	var ev SessionEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatalf("mustEvent: %v", err)
	}
	return ev
}
