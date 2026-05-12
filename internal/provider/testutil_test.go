package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories returns provider factories for use with the
// terraform-plugin-testing framework. The factory always constructs a fresh
// provider instance for each test step.
// init sets the namespace plugin-testing uses to construct the implicit
// required_providers source for each test factory. Without this, the
// framework would build `registry.terraform.io/hashicorp/<key>` and the test
// configuration wouldn't match our actual provider address.
func init() {
	_ = os.Setenv("TF_ACC_PROVIDER_NAMESPACE", "andasv")
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"claude-managed-agents": providerserver.NewProtocol6WithError(New("test", "test")()),
}

// fakeAPI is a tiny in-memory stand-in for the Managed Agents API. It is good
// enough to exercise CRUD + drift + import behavior without network access.
type fakeAPI struct {
	mu      sync.Mutex
	agents  map[string]*fakeAgent
	counter int
}

type fakeAgent struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Model       map[string]string `json:"model"`
	System      *string           `json:"system"`
	Description *string           `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Version     int               `json:"version"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{agents: map[string]*fakeAgent{}}
}

// MutateAgent runs f under lock so tests can simulate out-of-band edits to
// drive drift-detection scenarios.
func (f *fakeAPI) MutateAgent(id string, mutate func(a *fakeAgent)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.agents[id]; ok {
		mutate(a)
		a.Version++
		a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
}

// Snapshot returns a copy of the agent with id, or nil. For test assertions.
func (f *fakeAPI) Snapshot(id string) *fakeAgent {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.agents[id]
	if !ok {
		return nil
	}
	cp := *a
	return &cp
}

func (f *fakeAPI) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			f.create(w, r)
		case http.MethodGet:
			f.list(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	archiveRe := regexp.MustCompile(`^/v1/agents/([^/]+)/archive$`)
	itemRe := regexp.MustCompile(`^/v1/agents/([^/]+)$`)

	mux.HandleFunc("/v1/agents/", func(w http.ResponseWriter, r *http.Request) {
		if m := archiveRe.FindStringSubmatch(r.URL.Path); m != nil {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			f.archive(w, m[1])
			return
		}
		if m := itemRe.FindStringSubmatch(r.URL.Path); m != nil {
			switch r.Method {
			case http.MethodGet:
				f.get(w, m[1])
			case http.MethodPost:
				f.update(w, r, m[1])
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	return mux
}

func (f *fakeAPI) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string            `json:"name"`
		Model       any               `json:"model"`
		System      *string           `json:"system"`
		Description *string           `json:"description"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if body.Name == "" || body.Model == nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "name and model are required")
		return
	}
	modelID, ok := modelStringOf(body.Model)
	if !ok {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "model must be a string or {id} object")
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.counter++
	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("agent_FAKE%04d", f.counter)
	agent := &fakeAgent{
		ID:          id,
		Type:        "agent",
		Name:        body.Name,
		Model:       map[string]string{"id": modelID, "speed": "standard"},
		System:      body.System,
		Description: body.Description,
		Metadata:    body.Metadata,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if agent.Metadata == nil {
		agent.Metadata = map[string]string{}
	}
	f.agents[id] = agent

	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(agent)
}

func (f *fakeAPI) get(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.agents[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such agent")
		return
	}
	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(a)
}

func (f *fakeAPI) update(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Version     int                `json:"version"`
		Name        *string            `json:"name"`
		Model       any                `json:"model"`
		System      json.RawMessage    `json:"system"`
		Description json.RawMessage    `json:"description"`
		Metadata    map[string]string  `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.agents[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such agent")
		return
	}
	if a.ArchivedAt != nil {
		writeAPIErr(w, http.StatusConflict, "conflict_error", "agent is archived")
		return
	}
	if body.Version != a.Version {
		writeAPIErr(w, http.StatusConflict, "conflict_error", "version mismatch")
		return
	}

	if body.Name != nil {
		a.Name = *body.Name
	}
	if body.Model != nil {
		if id, ok := modelStringOf(body.Model); ok {
			a.Model["id"] = id
		}
	}
	if body.System != nil {
		a.System = decodeNullableString(body.System)
	}
	if body.Description != nil {
		a.Description = decodeNullableString(body.Description)
	}
	for k, v := range body.Metadata {
		if v == "" {
			delete(a.Metadata, k)
		} else {
			a.Metadata[k] = v
		}
	}

	a.Version++
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(a)
}

func (f *fakeAPI) archive(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.agents[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such agent")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	a.ArchivedAt = &now
	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(a)
}

func (f *fakeAPI) list(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	out := struct {
		Data    []*fakeAgent `json:"data"`
		HasMore bool         `json:"has_more"`
		FirstID string       `json:"first_id"`
		LastID  string       `json:"last_id"`
	}{}
	for _, a := range f.agents {
		if a.ArchivedAt != nil && !includeArchived {
			continue
		}
		out.Data = append(out.Data, a)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func writeAPIErr(w http.ResponseWriter, status int, typ, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "error",
		"error": map[string]any{"type": typ, "message": msg},
	})
}

func modelStringOf(v any) (string, bool) {
	switch m := v.(type) {
	case string:
		return m, true
	case map[string]any:
		if id, ok := m["id"].(string); ok {
			return id, true
		}
	}
	return "", false
}

func decodeNullableString(raw json.RawMessage) *string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return &s
}

// startFakeAPI starts the in-process fake server, sets the env vars the
// provider Configure reads, and returns a cleanup function plus the API
// handle for direct test manipulation.
//
// When TF_ACC_LIVE is set together with ANTHROPIC_API_KEY, the fixture is
// bypassed and the provider talks to the real Anthropic API.
func startFakeAPI(t *testing.T) (*fakeAPI, func()) {
	t.Helper()
	live := os.Getenv("TF_ACC_LIVE") == "1" && os.Getenv("ANTHROPIC_API_KEY") != ""
	if live {
		// Use the real API. No fake server. ANTHROPIC_API_KEY is already set.
		return nil, func() {}
	}

	api := newFakeAPI()
	srv := httptest.NewServer(api.handler())
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("CLAUDE_MANAGED_AGENTS_BASE_URL", srv.URL)
	return api, srv.Close
}

// providerConfig returns the HCL `provider { }` block used in every test step.
// The base_url is set from the env var that startFakeAPI exports.
//
// terraform-plugin-testing auto-injects a `terraform { required_providers {} }`
// block per factory using the namespace from TF_ACC_PROVIDER_NAMESPACE (set
// in this package's init), so we don't write one ourselves.
func providerConfig() string {
	base := os.Getenv("CLAUDE_MANAGED_AGENTS_BASE_URL")
	if base == "" {
		return `
provider "claude-managed-agents" {}
`
	}
	return fmt.Sprintf(`
provider "claude-managed-agents" {
  base_url = %q
}
`, base)
}
