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
	mu             sync.Mutex
	agents         map[string]*fakeAgent
	envs           map[string]*fakeEnvironment
	envBlockDelete map[string]bool // ids for which env DELETE should 409
	stores         map[string]*fakeMemoryStore
	vaults         map[string]*fakeVault
	creds          map[string]*fakeVaultCredential // keyed by credential id
	files          map[string]*fakeFile
	agentVersions  map[string][]map[string]any // agent_id → snapshots
	counter        int
	envCounter     int
	storeCounter   int
	vaultCounter   int
	credCounter    int
}

type fakeVault struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

type fakeVaultCredential struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	VaultID     string         `json:"vault_id"`
	DisplayName string         `json:"display_name"`
	Auth        map[string]any `json:"auth"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	ArchivedAt  *string        `json:"archived_at"`
}

type fakeMemoryStore struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	ArchivedAt  *string `json:"archived_at"`
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

	Tools      []map[string]any `json:"tools,omitempty"`
	McpServers []map[string]any `json:"mcp_servers,omitempty"`
	Skills     []map[string]any `json:"skills,omitempty"`
	Multiagent map[string]any   `json:"multiagent,omitempty"`
}

type fakeEnvironment struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Config     map[string]any `json:"config"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
	ArchivedAt *string        `json:"archived_at"`
}

type fakeFile struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	MimeType  string `json:"mime_type"`
	ScopeID   string `json:"scope_id"`
	CreatedAt string `json:"created_at"`
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		agents:         map[string]*fakeAgent{},
		envs:           map[string]*fakeEnvironment{},
		envBlockDelete: map[string]bool{},
		stores:         map[string]*fakeMemoryStore{},
		vaults:         map[string]*fakeVault{},
		creds:          map[string]*fakeVaultCredential{},
		files:          map[string]*fakeFile{},
		agentVersions:  map[string][]map[string]any{},
	}
}

// SeedFile installs a fake file record so the file data source can resolve it.
func (f *fakeAPI) SeedFile(file *fakeFile) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[file.ID] = file
}

// SeedAgentVersion adds a version snapshot under agentID. The order of
// snapshots is significant — list returns them in insertion order.
func (f *fakeAPI) SeedAgentVersion(agentID string, snapshot map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.agentVersions[agentID] = append(f.agentVersions[agentID], snapshot)
}

// DeleteAllStores wipes the memory store map. Use to simulate an
// out-of-band deletion before a Read step.
func (f *fakeAPI) DeleteAllStores() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stores = map[string]*fakeMemoryStore{}
}

// DeleteAllVaults wipes the vault and credential maps. Use to simulate an
// out-of-band deletion before a Read step.
func (f *fakeAPI) DeleteAllVaults() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vaults = map[string]*fakeVault{}
	f.creds = map[string]*fakeVaultCredential{}
}

// BlockEnvDelete makes DELETE /v1/environments/{id} return 409 on the next
// call, simulating "active sessions reference the environment".
func (f *fakeAPI) BlockEnvDelete(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envBlockDelete[id] = true
}

// SnapshotEnv returns a copy of the environment with id, or nil.
func (f *fakeAPI) SnapshotEnv(id string) *fakeEnvironment {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[id]
	if !ok {
		return nil
	}
	cp := *e
	return &cp
}

// DeleteAllEnvs wipes the environment map. Use to simulate an out-of-band
// deletion before a Read step.
func (f *fakeAPI) DeleteAllEnvs() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envs = map[string]*fakeEnvironment{}
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

// DeleteAllAgents wipes the agent map. Use to simulate an out-of-band deletion
// before a Read or Delete step.
func (f *fakeAPI) DeleteAllAgents() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.agents = map[string]*fakeAgent{}
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
	versionsRe := regexp.MustCompile(`^/v1/agents/([^/]+)/versions$`)
	itemRe := regexp.MustCompile(`^/v1/agents/([^/]+)$`)

	mux.HandleFunc("/v1/agents/", func(w http.ResponseWriter, r *http.Request) {
		if m := versionsRe.FindStringSubmatch(r.URL.Path); m != nil {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			f.agentVersionsList(w, m[1])
			return
		}
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

	mux.HandleFunc("/v1/environments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			f.envCreate(w, r)
		case http.MethodGet:
			f.envList(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	envArchiveRe := regexp.MustCompile(`^/v1/environments/([^/]+)/archive$`)
	envItemRe := regexp.MustCompile(`^/v1/environments/([^/]+)$`)

	fileItemRe := regexp.MustCompile(`^/v1/files/([^/]+)$`)
	mux.HandleFunc("/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		if m := fileItemRe.FindStringSubmatch(r.URL.Path); m != nil {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			f.fileGet(w, m[1])
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	mux.HandleFunc("/v1/vaults", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			f.vaultCreate(w, r)
		case http.MethodGet:
			f.vaultList(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	vaultArchiveRe := regexp.MustCompile(`^/v1/vaults/([^/]+)/archive$`)
	credListRe := regexp.MustCompile(`^/v1/vaults/([^/]+)/credentials$`)
	credArchiveRe := regexp.MustCompile(`^/v1/vaults/([^/]+)/credentials/([^/]+)/archive$`)
	credItemRe := regexp.MustCompile(`^/v1/vaults/([^/]+)/credentials/([^/]+)$`)
	vaultItemRe := regexp.MustCompile(`^/v1/vaults/([^/]+)$`)

	mux.HandleFunc("/v1/vaults/", func(w http.ResponseWriter, r *http.Request) {
		if m := credArchiveRe.FindStringSubmatch(r.URL.Path); m != nil {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			f.credArchive(w, m[1], m[2])
			return
		}
		if m := credItemRe.FindStringSubmatch(r.URL.Path); m != nil {
			switch r.Method {
			case http.MethodGet:
				f.credGet(w, m[1], m[2])
			case http.MethodPost:
				f.credUpdate(w, r, m[1], m[2])
			case http.MethodDelete:
				f.credDelete(w, m[1], m[2])
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if m := credListRe.FindStringSubmatch(r.URL.Path); m != nil {
			switch r.Method {
			case http.MethodPost:
				f.credCreate(w, r, m[1])
			case http.MethodGet:
				f.credList(w, r, m[1])
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if m := vaultArchiveRe.FindStringSubmatch(r.URL.Path); m != nil {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			f.vaultArchive(w, m[1])
			return
		}
		if m := vaultItemRe.FindStringSubmatch(r.URL.Path); m != nil {
			switch r.Method {
			case http.MethodGet:
				f.vaultGet(w, m[1])
			case http.MethodPost:
				f.vaultUpdate(w, r, m[1])
			case http.MethodDelete:
				f.vaultDelete(w, m[1])
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	mux.HandleFunc("/v1/memory_stores", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			f.storeCreate(w, r)
		case http.MethodGet:
			f.storeList(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	storeArchiveRe := regexp.MustCompile(`^/v1/memory_stores/([^/]+)/archive$`)
	storeItemRe := regexp.MustCompile(`^/v1/memory_stores/([^/]+)$`)

	mux.HandleFunc("/v1/memory_stores/", func(w http.ResponseWriter, r *http.Request) {
		if m := storeArchiveRe.FindStringSubmatch(r.URL.Path); m != nil {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			f.storeArchive(w, m[1])
			return
		}
		if m := storeItemRe.FindStringSubmatch(r.URL.Path); m != nil {
			switch r.Method {
			case http.MethodGet:
				f.storeGet(w, m[1])
			case http.MethodPost:
				f.storeUpdate(w, r, m[1])
			case http.MethodDelete:
				f.storeDelete(w, m[1])
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	mux.HandleFunc("/v1/environments/", func(w http.ResponseWriter, r *http.Request) {
		if m := envArchiveRe.FindStringSubmatch(r.URL.Path); m != nil {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			f.envArchive(w, m[1])
			return
		}
		if m := envItemRe.FindStringSubmatch(r.URL.Path); m != nil {
			switch r.Method {
			case http.MethodGet:
				f.envGet(w, m[1])
			case http.MethodDelete:
				f.envDelete(w, m[1])
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	return mux
}

func (f *fakeAPI) envCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string         `json:"name"`
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if body.Name == "" {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "name is required")
		return
	}
	if body.Config == nil || body.Config["type"] == nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "config.type is required")
		return
	}
	net, _ := body.Config["networking"].(map[string]any)
	if net == nil || net["type"] == nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "config.networking.type is required")
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.envCounter++
	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("env_FAKE%04d", f.envCounter)
	env := &fakeEnvironment{
		ID:        id,
		Type:      "environment",
		Name:      body.Name,
		Config:    body.Config,
		CreatedAt: now,
		UpdatedAt: now,
	}
	f.envs[id] = env

	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(env)
}

func (f *fakeAPI) envGet(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such environment")
		return
	}
	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(e)
}

func (f *fakeAPI) envArchive(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such environment")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	e.ArchivedAt = &now
	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(e)
}

func (f *fakeAPI) envDelete(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.envs[id]; !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such environment")
		return
	}
	if f.envBlockDelete[id] {
		writeAPIErr(w, http.StatusConflict, "conflict_error", "environment is referenced by active sessions")
		return
	}
	delete(f.envs, id)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

func (f *fakeAPI) storeCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if body.Name == "" {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "name is required")
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.storeCounter++
	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("memstore_FAKE%04d", f.storeCounter)
	store := &fakeMemoryStore{
		ID:          id,
		Type:        "memory_store",
		Name:        body.Name,
		Description: body.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	f.stores[id] = store
	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(store)
}

func (f *fakeAPI) storeGet(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.stores[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such memory_store")
		return
	}
	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(s)
}

func (f *fakeAPI) storeUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name        *string         `json:"name"`
		Description json.RawMessage `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.stores[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such memory_store")
		return
	}
	if s.ArchivedAt != nil {
		writeAPIErr(w, http.StatusConflict, "conflict_error", "memory_store is archived")
		return
	}
	if body.Name != nil {
		s.Name = *body.Name
	}
	if body.Description != nil {
		s.Description = decodeNullableString(body.Description)
	}
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(s)
}

func (f *fakeAPI) storeArchive(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.stores[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such memory_store")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.ArchivedAt = &now
	w.Header().Set("request-id", "req_"+id)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(s)
}

func (f *fakeAPI) storeDelete(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.stores[id]; !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such memory_store")
		return
	}
	delete(f.stores, id)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

func (f *fakeAPI) storeList(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	out := struct {
		Data    []*fakeMemoryStore `json:"data"`
		HasMore bool               `json:"has_more"`
		FirstID string             `json:"first_id"`
		LastID  string             `json:"last_id"`
	}{Data: []*fakeMemoryStore{}}
	for _, s := range f.stores {
		if s.ArchivedAt != nil && !includeArchived {
			continue
		}
		out.Data = append(out.Data, s)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeAPI) envList(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	out := struct {
		Data    []*fakeEnvironment `json:"data"`
		HasMore bool               `json:"has_more"`
		FirstID string             `json:"first_id"`
		LastID  string             `json:"last_id"`
	}{Data: []*fakeEnvironment{}}
	for _, e := range f.envs {
		if e.ArchivedAt != nil && !includeArchived {
			continue
		}
		out.Data = append(out.Data, e)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeAPI) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string            `json:"name"`
		Model       any               `json:"model"`
		System      *string           `json:"system"`
		Description *string           `json:"description"`
		Metadata    map[string]string `json:"metadata"`
		Tools       []map[string]any  `json:"tools"`
		McpServers  []map[string]any  `json:"mcp_servers"`
		Skills      []map[string]any  `json:"skills"`
		Multiagent  map[string]any    `json:"multiagent"`
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
		Tools:       body.Tools,
		McpServers:  body.McpServers,
		Skills:      body.Skills,
		Multiagent:  body.Multiagent,
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
		Version     int               `json:"version"`
		Name        *string           `json:"name"`
		Model       any               `json:"model"`
		System      json.RawMessage   `json:"system"`
		Description json.RawMessage   `json:"description"`
		Metadata    map[string]any    `json:"metadata"`
		Tools       *[]map[string]any `json:"tools"`
		McpServers  *[]map[string]any `json:"mcp_servers"`
		Skills      *[]map[string]any `json:"skills"`
		Multiagent  *map[string]any   `json:"multiagent"`
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
	// Mirror real API merge semantics: a string value sets/updates the key;
	// a JSON null value (Go nil) deletes the key; keys not in the request
	// are left alone.
	for k, v := range body.Metadata {
		if v == nil {
			delete(a.Metadata, k)
			continue
		}
		if s, ok := v.(string); ok {
			a.Metadata[k] = s
		}
	}
	if body.Tools != nil {
		a.Tools = *body.Tools
	}
	if body.McpServers != nil {
		a.McpServers = *body.McpServers
	}
	if body.Skills != nil {
		a.Skills = *body.Skills
	}
	if body.Multiagent != nil {
		a.Multiagent = *body.Multiagent
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

// --- agent version + file handlers ---

func (f *fakeAPI) agentVersionsList(w http.ResponseWriter, agentID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	versions := f.agentVersions[agentID]
	out := struct {
		Data    []map[string]any `json:"data"`
		HasMore bool             `json:"has_more"`
		FirstID string           `json:"first_id"`
		LastID  string           `json:"last_id"`
	}{Data: versions}
	if out.Data == nil {
		out.Data = []map[string]any{}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func (f *fakeAPI) fileGet(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such file")
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(file)
}

// --- vault handlers ---

func (f *fakeAPI) vaultCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string            `json:"display_name"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if body.DisplayName == "" {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "display_name is required")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vaultCounter++
	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("vlt_FAKE%04d", f.vaultCounter)
	v := &fakeVault{
		ID: id, Type: "vault", DisplayName: body.DisplayName,
		Metadata: body.Metadata, CreatedAt: now, UpdatedAt: now,
	}
	if v.Metadata == nil {
		v.Metadata = map[string]string{}
	}
	f.vaults[id] = v
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAPI) vaultGet(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.vaults[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such vault")
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAPI) vaultUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		DisplayName *string        `json:"display_name"`
		Metadata    map[string]any `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.vaults[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such vault")
		return
	}
	if v.ArchivedAt != nil {
		writeAPIErr(w, http.StatusConflict, "conflict_error", "vault is archived")
		return
	}
	if body.DisplayName != nil {
		v.DisplayName = *body.DisplayName
	}
	// Mirror real API merge semantics; see fakeAgent.update().
	for k, val := range body.Metadata {
		if val == nil {
			delete(v.Metadata, k)
			continue
		}
		if s, ok := val.(string); ok {
			v.Metadata[k] = s
		}
	}
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAPI) vaultArchive(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.vaults[id]
	if !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such vault")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	v.ArchivedAt = &now
	// Cascade archive to credentials.
	for _, c := range f.creds {
		if c.VaultID == id && c.ArchivedAt == nil {
			c.ArchivedAt = &now
		}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAPI) vaultDelete(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.vaults[id]; !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such vault")
		return
	}
	delete(f.vaults, id)
	// Cascade delete to credentials.
	for cid, c := range f.creds {
		if c.VaultID == id {
			delete(f.creds, cid)
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

func (f *fakeAPI) vaultList(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	out := struct {
		Data    []*fakeVault `json:"data"`
		HasMore bool         `json:"has_more"`
		FirstID string       `json:"first_id"`
		LastID  string       `json:"last_id"`
	}{Data: []*fakeVault{}}
	for _, v := range f.vaults {
		if v.ArchivedAt != nil && !includeArchived {
			continue
		}
		out.Data = append(out.Data, v)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

// --- vault credential handlers ---

func (f *fakeAPI) credCreate(w http.ResponseWriter, r *http.Request, vaultID string) {
	var body struct {
		DisplayName string         `json:"display_name"`
		Auth        map[string]any `json:"auth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if body.DisplayName == "" || body.Auth == nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "display_name and auth are required")
		return
	}
	if body.Auth["type"] == nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "auth.type is required")
		return
	}
	if body.Auth["mcp_server_url"] == nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", "auth.mcp_server_url is required")
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.vaults[vaultID]; !ok {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such vault")
		return
	}
	// Reject duplicate mcp_server_url within the same active vault.
	for _, c := range f.creds {
		if c.VaultID == vaultID && c.ArchivedAt == nil {
			if existingURL, _ := c.Auth["mcp_server_url"].(string); existingURL != "" {
				if existingURL == body.Auth["mcp_server_url"] {
					writeAPIErr(w, http.StatusConflict, "conflict_error", "duplicate mcp_server_url within vault")
					return
				}
			}
		}
	}

	f.credCounter++
	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("cred_FAKE%04d", f.credCounter)
	storedAuth := scrubCredentialSecrets(body.Auth)
	c := &fakeVaultCredential{
		ID: id, Type: "vault_credential", VaultID: vaultID,
		DisplayName: body.DisplayName, Auth: storedAuth,
		CreatedAt: now, UpdatedAt: now,
	}
	f.creds[id] = c
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(c)
}

func (f *fakeAPI) credGet(w http.ResponseWriter, vaultID, credID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.creds[credID]
	if !ok || c.VaultID != vaultID {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such credential")
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(c)
}

func (f *fakeAPI) credUpdate(w http.ResponseWriter, r *http.Request, vaultID, credID string) {
	var body struct {
		DisplayName *string        `json:"display_name"`
		Auth        map[string]any `json:"auth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIErr(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.creds[credID]
	if !ok || c.VaultID != vaultID {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such credential")
		return
	}
	if c.ArchivedAt != nil {
		writeAPIErr(w, http.StatusConflict, "conflict_error", "credential is archived")
		return
	}
	if body.DisplayName != nil {
		c.DisplayName = *body.DisplayName
	}
	if body.Auth != nil {
		// Merge by overwriting non-locked fields and scrubbing secrets.
		for k, v := range scrubCredentialSecrets(body.Auth) {
			c.Auth[k] = v
		}
	}
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(c)
}

func (f *fakeAPI) credArchive(w http.ResponseWriter, vaultID, credID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.creds[credID]
	if !ok || c.VaultID != vaultID {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such credential")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	c.ArchivedAt = &now
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(c)
}

func (f *fakeAPI) credDelete(w http.ResponseWriter, vaultID, credID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.creds[credID]
	if !ok || c.VaultID != vaultID {
		writeAPIErr(w, http.StatusNotFound, "not_found_error", "no such credential")
		return
	}
	delete(f.creds, c.ID)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

func (f *fakeAPI) credList(w http.ResponseWriter, r *http.Request, vaultID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	out := struct {
		Data    []*fakeVaultCredential `json:"data"`
		HasMore bool                   `json:"has_more"`
		FirstID string                 `json:"first_id"`
		LastID  string                 `json:"last_id"`
	}{Data: []*fakeVaultCredential{}}
	for _, c := range f.creds {
		if c.VaultID != vaultID {
			continue
		}
		if c.ArchivedAt != nil && !includeArchived {
			continue
		}
		out.Data = append(out.Data, c)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

// scrubCredentialSecrets emulates the API's behavior of never returning
// secret values on read. We strip the four secret keys and any nested
// equivalents from the auth object before storing it.
func scrubCredentialSecrets(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch k {
		case "token", "access_token":
			continue
		case "refresh":
			if nested, ok := v.(map[string]any); ok {
				clean := make(map[string]any, len(nested))
				for nk, nv := range nested {
					if nk == "refresh_token" {
						continue
					}
					if nk == "token_endpoint_auth" {
						if auth, ok := nv.(map[string]any); ok {
							authClean := make(map[string]any, len(auth))
							for ak, av := range auth {
								if ak == "client_secret" {
									continue
								}
								authClean[ak] = av
							}
							clean[nk] = authClean
							continue
						}
					}
					clean[nk] = nv
				}
				out[k] = clean
				continue
			}
		}
		out[k] = v
	}
	return out
}

func writeAPIErr(w http.ResponseWriter, status int, typ, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
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
