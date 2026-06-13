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

// deploymentResponseFixture is an active deployment with a schedule, vaults,
// a github_repository resource (note: NO authorization_token — it is
// write-only and never returned), and a user.message initial event.
func deploymentResponseFixture() string {
	return `{
		"id":"deployment_FAKE01",
		"type":"deployment",
		"name":"nightly-digest",
		"agent":{"id":"agent_01ABC","type":"agent","version":7},
		"environment_id":"env_01",
		"description":"Runs nightly",
		"metadata":{"team":"platform"},
		"initial_events":[
			{"type":"user.message","content":[{"type":"text","text":"Run the digest."}]}
		],
		"resources":[
			{"type":"github_repository","url":"https://github.com/x/y","checkout":{"type":"branch","name":"main"},"mount_path":"/workspace/y"}
		],
		"schedule":{"type":"cron","expression":"0 3 * * *","timezone":"UTC","last_run_at":"2026-06-11T03:00:00Z","upcoming_runs_at":["2026-06-12T03:00:00Z"]},
		"vault_ids":["vault_01"],
		"status":"active",
		"paused_reason":null,
		"created_at":"2026-06-10T00:00:00Z",
		"updated_at":"2026-06-10T00:00:00Z",
		"archived_at":null
	}`
}

func TestCreateDeployment_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/deployments" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req DeploymentCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// agent is sent as a bare JSON string (latest version pin).
		if string(req.Agent) != `"agent_01ABC"` {
			t.Errorf("agent = %s, want \"agent_01ABC\"", req.Agent)
		}
		if req.Name != "nightly-digest" || req.EnvironmentID != "env_01" {
			t.Errorf("body = %+v", req)
		}
		if len(req.Resources) != 1 || req.Resources[0].AuthorizationToken != "ghp_secret" {
			t.Errorf("write-only token not forwarded in request: %+v", req.Resources)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(deploymentResponseFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	desc := "Runs nightly"
	dep, err := c.CreateDeployment(context.Background(), DeploymentCreateRequest{
		Name:          "nightly-digest",
		Agent:         json.RawMessage(`"agent_01ABC"`),
		EnvironmentID: "env_01",
		Description:   &desc,
		InitialEvents: []DeploymentInitialEvent{
			{Type: "user.message", Content: []json.RawMessage{json.RawMessage(`{"type":"text","text":"Run the digest."}`)}},
		},
		Resources: []DeploymentResource{
			{Type: "github_repository", URL: "https://github.com/x/y", AuthorizationToken: "ghp_secret", Checkout: &DeploymentCheckout{Type: "branch", Name: "main"}},
		},
		Schedule: &DeploymentSchedule{Type: "cron", Expression: "0 3 * * *", Timezone: "UTC"},
		VaultIDs: []string{"vault_01"},
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if dep.ID != "deployment_FAKE01" {
		t.Errorf("ID = %q", dep.ID)
	}
	// Resolved agent version comes back even though we sent a bare id.
	if dep.Agent.ID != "agent_01ABC" || dep.Agent.Version != 7 {
		t.Errorf("Agent = %+v, want {agent_01ABC v7}", dep.Agent)
	}
	if dep.Status != "active" {
		t.Errorf("Status = %q", dep.Status)
	}
	// Write-only token is never returned by the API.
	if dep.Resources[0].AuthorizationToken != "" {
		t.Errorf("AuthorizationToken leaked into read: %q", dep.Resources[0].AuthorizationToken)
	}
	// Schedule enrichment round-trips.
	if dep.Schedule == nil || dep.Schedule.LastRunAt == nil || len(dep.Schedule.UpcomingRunsAt) != 1 {
		t.Errorf("Schedule enrichment not parsed: %+v", dep.Schedule)
	}
	// Content blocks are preserved as raw JSON.
	if len(dep.InitialEvents) != 1 || !strings.Contains(string(dep.InitialEvents[0].Content[0]), "Run the digest.") {
		t.Errorf("InitialEvents content not preserved: %+v", dep.InitialEvents)
	}
}

func TestCreateDeployment_ValidatesRequired(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	oneEvent := []DeploymentInitialEvent{{Type: "user.message"}}
	tooMany := make([]DeploymentInitialEvent, 51)
	cases := map[string]DeploymentCreateRequest{
		"empty name":      {Agent: json.RawMessage(`"a"`), EnvironmentID: "e", InitialEvents: oneEvent},
		"empty agent":     {Name: "n", EnvironmentID: "e", InitialEvents: oneEvent},
		"empty env":       {Name: "n", Agent: json.RawMessage(`"a"`), InitialEvents: oneEvent},
		"no events":       {Name: "n", Agent: json.RawMessage(`"a"`), EnvironmentID: "e"},
		"too many events": {Name: "n", Agent: json.RawMessage(`"a"`), EnvironmentID: "e", InitialEvents: tooMany},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := c.CreateDeployment(context.Background(), req); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestGetDeployment_HappyPath_PausedOnError(t *testing.T) {
	// A deployment auto-paused because a vault was deleted.
	body := `{
		"id":"deployment_FAKE01","type":"deployment","name":"x",
		"agent":{"id":"agent_01ABC","type":"agent","version":7},
		"environment_id":"env_01","description":null,"metadata":{},
		"initial_events":[{"type":"system.message","content":[{"type":"text","text":"ctx"}]}],
		"resources":[],"schedule":null,"vault_ids":["vault_gone"],
		"status":"paused",
		"paused_reason":{"type":"error","error":{"type":"vault_not_found_error","message":"vault_gone is gone"}},
		"created_at":"2026-06-10T00:00:00Z","updated_at":"2026-06-10T00:00:00Z","archived_at":null
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/deployments/deployment_FAKE01" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	dep, err := c.GetDeployment(context.Background(), "deployment_FAKE01")
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.Status != "paused" {
		t.Errorf("Status = %q", dep.Status)
	}
	if dep.PausedReason == nil || dep.PausedReason.Type != "error" || dep.PausedReason.Error == nil || dep.PausedReason.Error.Type != "vault_not_found_error" {
		t.Errorf("PausedReason = %+v", dep.PausedReason)
	}
}

func TestGetDeployment_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.GetDeployment(context.Background(), "deployment_nope"); !IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestUpdateDeployment_PatchSemantics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Update is POST, not PATCH (the real endpoint 405s on PATCH).
		if r.Method != http.MethodPost || r.URL.Path != "/v1/deployments/deployment_FAKE01" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// description explicitly cleared via JSON null.
		if v, ok := body["description"]; !ok || v != nil {
			t.Errorf("description = %v (present=%v), want explicit null", v, ok)
		}
		// metadata key delete: null value.
		md, _ := body["metadata"].(map[string]any)
		if v, ok := md["old"]; !ok || v != nil {
			t.Errorf("metadata.old = %v, want null (delete)", md["old"])
		}
		// vault_ids cleared via empty list (not null = "unchanged").
		if vs, ok := body["vault_ids"].([]any); !ok || len(vs) != 0 {
			t.Errorf("vault_ids = %v, want []", body["vault_ids"])
		}
		// name not sent → field omitted entirely.
		if _, ok := body["name"]; ok {
			t.Errorf("name should be omitted when unchanged")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(deploymentResponseFixture()))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.UpdateDeployment(context.Background(), "deployment_FAKE01", DeploymentUpdateRequest{
		Description: json.RawMessage("null"),
		Metadata:    map[string]*string{"old": nil},
		VaultIDs:    &[]string{},
	})
	if err != nil {
		t.Fatalf("UpdateDeployment: %v", err)
	}
}

func TestUpdateDeployment_PropagatesConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"conflict_error","message":"archived"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	name := "x"
	if _, err := c.UpdateDeployment(context.Background(), "deployment_x", DeploymentUpdateRequest{Name: &name}); !IsConflict(err) {
		t.Errorf("want IsConflict, got %v", err)
	}
}

func TestArchiveDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/deployments/deployment_FAKE01/archive" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveDeployment(context.Background(), "deployment_FAKE01"); err != nil {
		t.Fatalf("ArchiveDeployment: %v", err)
	}
}

func TestArchiveDeployment_404IsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.ArchiveDeployment(context.Background(), "deployment_gone"); !IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestPauseResumeDeployment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/v1/deployments/deployment_FAKE01/pause":
			_, _ = w.Write([]byte(`{"id":"deployment_FAKE01","type":"deployment","name":"x","agent":{"id":"a","type":"agent","version":1},"environment_id":"e","metadata":{},"initial_events":[],"resources":[],"vault_ids":[],"status":"paused","paused_reason":{"type":"manual"},"created_at":"2026-06-10T00:00:00Z","updated_at":"2026-06-10T00:00:00Z","archived_at":null}`))
		case "/v1/deployments/deployment_FAKE01/unpause":
			_, _ = w.Write([]byte(`{"id":"deployment_FAKE01","type":"deployment","name":"x","agent":{"id":"a","type":"agent","version":1},"environment_id":"e","metadata":{},"initial_events":[],"resources":[],"vault_ids":[],"status":"active","paused_reason":null,"created_at":"2026-06-10T00:00:00Z","updated_at":"2026-06-10T00:00:00Z","archived_at":null}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	paused, err := c.PauseDeployment(context.Background(), "deployment_FAKE01")
	if err != nil {
		t.Fatalf("PauseDeployment: %v", err)
	}
	if paused.Status != "paused" || paused.PausedReason == nil || paused.PausedReason.Type != "manual" {
		t.Errorf("pause result = %+v / %+v", paused.Status, paused.PausedReason)
	}
	resumed, err := c.ResumeDeployment(context.Background(), "deployment_FAKE01")
	if err != nil {
		t.Fatalf("ResumeDeployment: %v", err)
	}
	if resumed.Status != "active" || resumed.PausedReason != nil {
		t.Errorf("resume result = %+v / %+v", resumed.Status, resumed.PausedReason)
	}
}

func TestDeployment_MethodsRequireID(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.GetDeployment(context.Background(), ""); err == nil {
		t.Error("GetDeployment: expected error for empty id")
	}
	if _, err := c.UpdateDeployment(context.Background(), "", DeploymentUpdateRequest{}); err == nil {
		t.Error("UpdateDeployment: expected error for empty id")
	}
	if err := c.ArchiveDeployment(context.Background(), ""); err == nil {
		t.Error("ArchiveDeployment: expected error for empty id")
	}
	if _, err := c.PauseDeployment(context.Background(), ""); err == nil {
		t.Error("PauseDeployment: expected error for empty id")
	}
	if _, err := c.ResumeDeployment(context.Background(), ""); err == nil {
		t.Error("ResumeDeployment: expected error for empty id")
	}
	if _, err := c.GetDeploymentRun(context.Background(), ""); err == nil {
		t.Error("GetDeploymentRun: expected error for empty id")
	}
}

func TestListDeployments_CursorPagination(t *testing.T) {
	page1 := `{"data":[{"id":"deployment_1","type":"deployment","name":"a","agent":{"id":"a","type":"agent","version":1},"environment_id":"e","metadata":{},"initial_events":[],"resources":[],"vault_ids":[],"status":"active","created_at":"2026-06-10T00:00:00Z","updated_at":"2026-06-10T00:00:00Z","archived_at":null}],"next_page":"CURSOR2"}`
	page2 := `{"data":[{"id":"deployment_2","type":"deployment","name":"b","agent":{"id":"a","type":"agent","version":1},"environment_id":"e","metadata":{},"initial_events":[],"resources":[],"vault_ids":[],"status":"active","created_at":"2026-06-10T00:00:00Z","updated_at":"2026-06-10T00:00:00Z","archived_at":null}],"next_page":null}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "50" {
			t.Errorf("limit = %q", r.URL.Query().Get("limit"))
		}
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("page") == "CURSOR2" {
			_, _ = w.Write([]byte(page2))
			return
		}
		_, _ = w.Write([]byte(page1))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var all []Deployment
	cursor := ""
	for {
		page, err := c.ListDeployments(context.Background(), ListDeploymentsParams{Limit: 50, Page: cursor})
		if err != nil {
			t.Fatalf("ListDeployments: %v", err)
		}
		all = append(all, page.Data...)
		if page.NextPage == nil {
			break
		}
		cursor = *page.NextPage
	}
	if len(all) != 2 || all[0].ID != "deployment_1" || all[1].ID != "deployment_2" {
		t.Errorf("paginated ids = %+v", all)
	}
}

func TestListDeploymentRuns_FiltersAndParse(t *testing.T) {
	hasError := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/deployment_runs" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("deployment_id") != "deployment_FAKE01" {
			t.Errorf("deployment_id = %q", q.Get("deployment_id"))
		}
		if q.Get("trigger_type") != "schedule" {
			t.Errorf("trigger_type = %q", q.Get("trigger_type"))
		}
		if q.Get("has_error") != "true" {
			t.Errorf("has_error = %q", q.Get("has_error"))
		}
		if q.Get("created_at_gte") != "2026-06-01T00:00:00Z" {
			t.Errorf("created_at_gte = %q", q.Get("created_at_gte"))
		}
		if q.Get("limit") != "100" {
			t.Errorf("limit = %q", q.Get("limit"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[
			{"id":"drun_1","type":"deployment_run","agent":{"id":"a","type":"agent","version":1},"deployment_id":"deployment_FAKE01","created_at":"2026-06-11T03:00:00Z","session_id":null,"error":{"type":"vault_not_found_error","message":"gone"},"trigger_context":{"type":"schedule","scheduled_at":"2026-06-11T03:00:00Z"}}
		],"next_page":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	runs, err := c.ListDeploymentRuns(context.Background(), ListDeploymentRunsParams{
		DeploymentID: "deployment_FAKE01",
		TriggerType:  "schedule",
		HasError:     &hasError,
		CreatedAtGte: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Limit:        100,
	})
	if err != nil {
		t.Fatalf("ListDeploymentRuns: %v", err)
	}
	if len(runs.Data) != 1 {
		t.Fatalf("got %d runs, want 1", len(runs.Data))
	}
	run := runs.Data[0]
	if run.SessionID != nil {
		t.Errorf("SessionID = %v, want nil on failed run", run.SessionID)
	}
	if run.Error == nil || run.Error.Type != "vault_not_found_error" {
		t.Errorf("Error = %+v", run.Error)
	}
	if run.TriggerContext.Type != "schedule" || run.TriggerContext.ScheduledAt == nil {
		t.Errorf("TriggerContext = %+v", run.TriggerContext)
	}
}

func TestListDeploymentRuns_RemainingFiltersAndCursor(t *testing.T) {
	// Exercises the created_at_gt/lt/lte, page cursor, has_error=false, and
	// trigger_type=manual query-builder branches not hit by the filter test.
	hasError := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for k, want := range map[string]string{
			"created_at_gt":  "2026-06-01T00:00:00Z",
			"created_at_lt":  "2026-06-30T00:00:00Z",
			"created_at_lte": "2026-06-29T00:00:00Z",
			"has_error":      "false",
			"trigger_type":   "manual",
			"page":           "CURSOR",
		} {
			if q.Get(k) != want {
				t.Errorf("%s = %q, want %q", k, q.Get(k), want)
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"next_page":null}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.ListDeploymentRuns(context.Background(), ListDeploymentRunsParams{
		TriggerType:  "manual",
		HasError:     &hasError,
		CreatedAtGt:  time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		CreatedAtLt:  time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		CreatedAtLte: time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC),
		Page:         "CURSOR",
	})
	if err != nil {
		t.Fatalf("ListDeploymentRuns: %v", err)
	}
}

func TestGetDeploymentRun_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/deployment_runs/drun_1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"drun_1","type":"deployment_run","agent":{"id":"a","type":"agent","version":1},"deployment_id":"deployment_FAKE01","created_at":"2026-06-11T03:00:00Z","session_id":"session_9","error":null,"trigger_context":{"type":"manual"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	run, err := c.GetDeploymentRun(context.Background(), "drun_1")
	if err != nil {
		t.Fatalf("GetDeploymentRun: %v", err)
	}
	if run.SessionID == nil || *run.SessionID != "session_9" {
		t.Errorf("SessionID = %v, want session_9", run.SessionID)
	}
	if run.Error != nil {
		t.Errorf("Error = %+v, want nil on success", run.Error)
	}
	if run.TriggerContext.Type != "manual" || run.TriggerContext.ScheduledAt != nil {
		t.Errorf("TriggerContext = %+v", run.TriggerContext)
	}
}
