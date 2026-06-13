package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ModelConfig is the agent's model setting. The API accepts a bare string on
// create but always returns an object on read.
type ModelConfig struct {
	ID    string `json:"id"`
	Speed string `json:"speed,omitempty"`
	Type  string `json:"type,omitempty"`
}

// Agent is the read shape returned by GET /v1/agents/{id}.
//
// All four nested-config fields (tools, mcp_servers, skills, multiagent)
// round-trip through typed structs as of v0.2.
type Agent struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Model       ModelConfig       `json:"model"`
	System      *string           `json:"system"`
	Description *string           `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ArchivedAt  *time.Time        `json:"archived_at"`

	Tools      []Tool          `json:"tools,omitempty"`
	McpServers []McpServer     `json:"mcp_servers,omitempty"`
	Skills     []AgentSkillRef `json:"skills,omitempty"`
	Multiagent *Multiagent     `json:"multiagent,omitempty"`
}

// Tool is one entry of the agent's `tools` list. The shape is a
// discriminated union on `type`:
//
//   - "agent_toolset_20260401" — the bundled Anthropic toolset (bash, edit,
//     web_fetch, etc.). DefaultConfig applies to every tool; Configs
//     overrides per-tool settings by `name`.
//   - "mcp_toolset" — exposes the tools of an MCP server. McpServerName
//     must match the `name` of an entry in the agent's `mcp_servers`.
//     DefaultConfig / Configs follow the same shape but Configs `name`
//     refers to individual tool names exposed by the MCP server.
//   - "custom" — a user-defined tool. Name + Description are required;
//     InputSchema is a JSON Schema describing the tool's argument shape.
//
// Only fields relevant to the variant are populated by the API on read.
type Tool struct {
	Type string `json:"type"`

	// agent_toolset_20260401 + mcp_toolset
	DefaultConfig *ToolConfig  `json:"default_config,omitempty"`
	Configs       []ToolConfig `json:"configs,omitempty"`

	// mcp_toolset
	McpServerName string `json:"mcp_server_name,omitempty"`

	// custom
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ToolConfig is one entry of `configs` (Name set) or the value of
// `default_config` (Name empty).
type ToolConfig struct {
	Name             string            `json:"name,omitempty"`
	Enabled          *bool             `json:"enabled,omitempty"`
	PermissionPolicy *PermissionPolicy `json:"permission_policy,omitempty"`
}

// PermissionPolicy gates whether a server-executed tool runs automatically
// (`always_allow`) or waits for explicit approval (`always_ask`).
type PermissionPolicy struct {
	Type string `json:"type"`
}

// McpServer is one entry of the agent's `mcp_servers` list.
//
// The only documented `type` value is "url"; other variants would need new
// fields here.
type McpServer struct {
	Type string `json:"type"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// AgentSkillRef is one entry of the agent's `skills` list — a reference to
// an existing skill, not the skill itself. The skill's content (files
// under SKILL.md) is managed via the separate Skills API and the
// `claude-managed-agents_skill` resource introduced in v0.3.
//
// Discriminated on `type`:
//   - "anthropic" — pre-built skills (`skill_id` is a short name like "xlsx")
//   - "custom"    — user-uploaded skills (`skill_id` is `skill_*` and
//     `version` is optional, defaulting to "latest")
type AgentSkillRef struct {
	Type    string `json:"type"`
	SkillID string `json:"skill_id"`
	Version string `json:"version,omitempty"`
}

// Skill is the read shape returned by GET /v1/skills/{id} and POST
// /v1/skills. The Skills API is a beta surface gated by the
// `anthropic-beta: skills-2025-10-02` header (see `skillsBeta`); the client
// applies it automatically on every skill method.
//
// `Source` is "custom" for user-uploaded skills and "anthropic" for
// pre-built skills (e.g. `pptx`, `xlsx`). `LatestVersion` is an epoch
// timestamp string for custom skills and an ISO date for Anthropic
// prebuilts.
type Skill struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Source        string    `json:"source"`
	DisplayTitle  string    `json:"display_title"`
	LatestVersion string    `json:"latest_version"`
	CreatedAt     time.Time `json:"created_at"`
}

// SkillVersion is one entry in GET /v1/skills/{id}/versions.
type SkillVersion struct {
	Type      string    `json:"type"`
	SkillID   string    `json:"skill_id"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

// SkillFile is one file in the multipart upload. Path is the POSIX-style
// relative path within the skill (SKILL.md must be at the root, no leading
// slash). Content is the raw bytes.
type SkillFile struct {
	Path    string
	Content []byte
}

// SkillCreateRequest is the body for POST /v1/skills (multipart/form-data).
// The client converts this to a multipart payload via buildSkillMultipart.
type SkillCreateRequest struct {
	DisplayTitle string
	Files        []SkillFile
}

// SkillVersionCreateRequest is the body for POST /v1/skills/{id}/versions
// (multipart/form-data). Like SkillCreateRequest, Files must contain a
// `SKILL.md` at the root.
type SkillVersionCreateRequest struct {
	Files []SkillFile
}

// ListSkillsParams holds query parameters for ListSkills.
type ListSkillsParams struct {
	Limit    int
	BeforeID string
	AfterID  string
	Source   string // "" | "custom" | "anthropic"
}

// ListSkillVersionsParams holds query parameters for ListSkillVersions.
type ListSkillVersionsParams struct {
	Limit    int
	BeforeID string
	AfterID  string
}

// Multiagent is the agent's optional coordinator config.
type Multiagent struct {
	Type   string             `json:"type"`
	Agents []MultiagentMember `json:"agents,omitempty"`
}

// MultiagentMember is one entry of the coordinator's `agents` list.
// Discriminated on `type`:
//   - "agent" — references another agent by `id`
//   - "self"  — the coordinator may invoke itself; no `id`
type MultiagentMember struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

// AgentCreateRequest is the body for POST /v1/agents.
//
// The fields Model is sent as a bare string because that is what the upstream
// API accepts and what most users will write in HCL.
type AgentCreateRequest struct {
	Name        string            `json:"name"`
	Model       string            `json:"model"`
	System      *string           `json:"system,omitempty"`
	Description *string           `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	Tools      []Tool          `json:"tools,omitempty"`
	McpServers []McpServer     `json:"mcp_servers,omitempty"`
	Skills     []AgentSkillRef `json:"skills,omitempty"`
	Multiagent *Multiagent     `json:"multiagent,omitempty"`
}

// AgentUpdateRequest is the body for POST /v1/agents/{id}.
//
// Version is required (optimistic concurrency). Use pointer fields for
// scalars so the marshalled JSON can distinguish "leave unchanged" (nil)
// from "set to null" (a typed nil string pointer is impossible in JSON, so
// we wrap with json.RawMessage for the null-clear case).
type AgentUpdateRequest struct {
	Version     int             `json:"version"`
	Name        *string         `json:"name,omitempty"`
	Model       *string         `json:"model,omitempty"`
	System      json.RawMessage `json:"system,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	// Metadata uses map[string]any so the provider can send JSON null
	// values for keys it wants to delete; a string value updates/creates
	// the key. The upstream API uses merge semantics (not full-replace),
	// so unset keys are left alone.
	Metadata map[string]any `json:"metadata,omitempty"`

	// Pointer slices distinguish "leave unchanged" (nil) from "replace
	// with this exact list" (non-nil — including an explicit empty list).
	Tools      *[]Tool          `json:"tools,omitempty"`
	McpServers *[]McpServer     `json:"mcp_servers,omitempty"`
	Skills     *[]AgentSkillRef `json:"skills,omitempty"`
	Multiagent *Multiagent      `json:"multiagent,omitempty"`
}

// ListResponse is the cursor pagination envelope shared by all list endpoints.
type ListResponse[T any] struct {
	Data    []T    `json:"data"`
	HasMore bool   `json:"has_more"`
	FirstID string `json:"first_id"`
	LastID  string `json:"last_id"`
}

// Environment is the read shape returned by GET /v1/environments/{id}.
//
// Environments are immutable post-creation upstream — there is no update
// endpoint. Treat every field as ForceNew in the Terraform resource.
type Environment struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	Name       string      `json:"name"`
	Config     CloudConfig `json:"config"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	ArchivedAt *time.Time  `json:"archived_at"`
}

// CloudConfig is the only documented value of `config.type`. Future config
// types would need new variants here.
type CloudConfig struct {
	Type       string     `json:"type"` // currently only "cloud"
	Packages   *Packages  `json:"packages,omitempty"`
	Networking Networking `json:"networking"`
}

// Packages is the per-package-manager install list. All fields optional.
type Packages struct {
	Apt   []string `json:"apt,omitempty"`
	Cargo []string `json:"cargo,omitempty"`
	Gem   []string `json:"gem,omitempty"`
	Go    []string `json:"go,omitempty"`
	Npm   []string `json:"npm,omitempty"`
	Pip   []string `json:"pip,omitempty"`
}

// Networking is the discriminated-union outbound policy.
//
// Type values:
//   - "unrestricted" — agent can reach any host. AllowedHosts and the two
//     Allow* booleans must be empty/nil.
//   - "limited"      — agent restricted to AllowedHosts. The two Allow*
//     booleans gate MCP servers and package managers respectively.
type Networking struct {
	Type                 string   `json:"type"`
	AllowedHosts         []string `json:"allowed_hosts,omitempty"`
	AllowMcpServers      *bool    `json:"allow_mcp_servers,omitempty"`
	AllowPackageManagers *bool    `json:"allow_package_managers,omitempty"`
}

// EnvironmentCreateRequest is the body for POST /v1/environments.
type EnvironmentCreateRequest struct {
	Name   string      `json:"name"`
	Config CloudConfig `json:"config"`
}

// MemoryStore is the read shape returned by GET /v1/memory_stores/{id}.
//
// Memory stores have a `name` and a `description` that are mutable post
// create. The description is surfaced in the agent's system prompt, so
// changing it changes how the agent talks about the store.
type MemoryStore struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
}

// MemoryStoreCreateRequest is the body for POST /v1/memory_stores.
type MemoryStoreCreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// Vault is the read shape returned by GET /v1/vaults/{id}.
//
// Vaults are workspace-scoped containers of credentials, typically used to
// model one end-user. Both display_name and metadata are mutable.
type Vault struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ArchivedAt  *time.Time        `json:"archived_at"`
}

// VaultCreateRequest is the body for POST /v1/vaults.
type VaultCreateRequest struct {
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// VaultUpdateRequest is the body for POST /v1/vaults/{id}. nil DisplayName
// means leave unchanged. Metadata uses merge semantics (not full-replace):
// the upstream API merges the supplied map on top of stored values. To
// delete a key, send JSON null for it — modeled here as map[string]any so
// nil values marshal as null.
type VaultUpdateRequest struct {
	DisplayName *string        `json:"display_name,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// VaultCredential is the read shape returned by GET
// /v1/vaults/{vault_id}/credentials/{credential_id}. Secret payloads are
// never returned by the API; only metadata and non-secret config fields
// (such as `auth.mcp_server_url`) appear on read.
type VaultCredential struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	VaultID     string              `json:"vault_id"`
	DisplayName string              `json:"display_name"`
	Auth        VaultCredentialAuth `json:"auth"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	ArchivedAt  *time.Time          `json:"archived_at"`
}

// VaultCredentialAuth is the union of the two `auth` shapes the API returns
// on read. Secrets are never present; the type discriminator drives
// interpretation.
//
// For `mcp_oauth`, the `expires_at` and the `refresh` block (sans secrets)
// are returned. For `static_bearer`, only `mcp_server_url` is meaningful.
type VaultCredentialAuth struct {
	Type         string                      `json:"type"`
	McpServerURL string                      `json:"mcp_server_url"`
	ExpiresAt    *time.Time                  `json:"expires_at,omitempty"`
	Refresh      *VaultCredentialAuthRefresh `json:"refresh,omitempty"`
}

// VaultCredentialAuthRefresh mirrors the read shape of the OAuth refresh
// sub-object. The secret-bearing fields (`refresh_token`, `client_secret`)
// are not present on read.
type VaultCredentialAuthRefresh struct {
	TokenEndpoint     string                                 `json:"token_endpoint"`
	ClientID          string                                 `json:"client_id"`
	Scope             string                                 `json:"scope,omitempty"`
	TokenEndpointAuth VaultCredentialAuthRefreshEndpointAuth `json:"token_endpoint_auth"`
}

// VaultCredentialAuthRefreshEndpointAuth carries only `type` on read; the
// `client_secret` is purged.
type VaultCredentialAuthRefreshEndpointAuth struct {
	Type string `json:"type"`
}

// VaultCredentialCreateRequest is the body for POST
// /v1/vaults/{vault_id}/credentials. The Auth field is a free-form map
// because the union shape varies by auth.type; callers build the map
// directly to keep the client thin and avoid leaking secrets into typed
// fields that might accidentally end up in logs.
type VaultCredentialCreateRequest struct {
	DisplayName string         `json:"display_name"`
	Auth        map[string]any `json:"auth"`
}

// VaultCredentialUpdateRequest is the body for POST
// /v1/vaults/{vault_id}/credentials/{credential_id}. Only the secret payload
// and a few metadata fields are mutable; `mcp_server_url`,
// `token_endpoint`, and `client_id` are locked.
type VaultCredentialUpdateRequest struct {
	DisplayName *string        `json:"display_name,omitempty"`
	Auth        map[string]any `json:"auth,omitempty"`
}

// AgentVersion is one entry in the agent's version history. Returned by
// GET /v1/agents/{id}/versions. The provider exposes a data source that
// looks up a specific version number by listing and filtering.
type AgentVersion struct {
	Type        string            `json:"type"`
	AgentID     string            `json:"agent_id"`
	Version     int               `json:"version"`
	Name        string            `json:"name"`
	Model       ModelConfig       `json:"model"`
	System      *string           `json:"system"`
	Description *string           `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// File is the read shape returned by GET /v1/files/{id}.
//
// Files are session-scoped artifacts. Only metadata is exposed by the
// provider data source; the binary content endpoint is not modeled.
type File struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	MimeType  string    `json:"mime_type"`
	ScopeID   string    `json:"scope_id"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryStoreUpdateRequest is the body for POST /v1/memory_stores/{id}.
//
// Name and Description use pointer / raw-message semantics matching agent
// updates: a nil Name leaves it unchanged; a non-nil Description that is
// the JSON literal `null` clears the field.
type MemoryStoreUpdateRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
}

// Session is the read shape returned by POST /v1/sessions and GET
// /v1/sessions/{id}. Sessions are gated by the standard managed-agents
// beta header (`managed-agents-2026-04-01`); no new beta override is
// required.
//
// A session is a running instance of an agent inside an environment. The
// `Status` field is one of "idle" | "running" | "rescheduling" |
// "terminated"; the lifecycle is driven by user / agent events sent
// through /events.
type Session struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"`
	AgentID            string            `json:"agent_id"`
	EnvironmentID      *string           `json:"environment_id,omitempty"`
	Status             string            `json:"status"`
	Title              *string           `json:"title,omitempty"`
	Usage              *SessionUsage     `json:"usage,omitempty"`
	OutcomeEvaluations []json.RawMessage `json:"outcome_evaluations,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	ArchivedAt         *time.Time        `json:"archived_at"`
}

// SessionUsage is the cumulative token-usage tally returned on Session.
// Reported after the session goes idle.
type SessionUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// SessionResource is one entry in SessionCreateRequest.Resources. It
// attaches an external object (memory store, file, or repository) to
// the session at create time; the upstream API does not support
// attaching additional resources to a running session, so callers must
// know up front what the session needs.
//
// Type is the discriminator and selects which of the *ID fields is
// honored. Instructions is optional session-specific guidance shown to
// the agent alongside the resource's name and description; capped at
// 4096 characters server-side.
type SessionResource struct {
	Type          string `json:"type"`
	MemoryStoreID string `json:"memory_store_id,omitempty"`
	FileID        string `json:"file_id,omitempty"`
	RepositoryID  string `json:"repository_id,omitempty"`
	Instructions  string `json:"instructions,omitempty"`
}

// SessionCreateRequest is the body for POST /v1/sessions. AgentID is sent
// as the bare string form (latest agent version); pinning to a specific
// version is not currently exposed by this client. EnvironmentID is
// required by the upstream API in practice; VaultIDs, Resources, and
// Title are optional.
type SessionCreateRequest struct {
	AgentID       string            `json:"agent"`
	EnvironmentID string            `json:"environment_id,omitempty"`
	VaultIDs      []string          `json:"vault_ids,omitempty"`
	Resources     []SessionResource `json:"resources,omitempty"`
	Title         string            `json:"title,omitempty"`
}

// EventContent is one content block inside a user event. Today only the
// "text" type is sent by this client; the struct is open-ended so other
// types can be added without breaking the API.
type EventContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// UserEvent is one entry of the `events` array on POST
// /v1/sessions/{id}/events. The harness only needs `user.message` today;
// other discriminator fields (custom_tool_use_id, tool_use_id, result,
// deny_message) are intentionally omitted and will be added when the
// harness gains tool-confirmation support.
type UserEvent struct {
	Type    string         `json:"type"`
	Content []EventContent `json:"content,omitempty"`
}

// SessionEvent is one entry in the session's read-side event log.
//
// The session event stream is a discriminated union with ~25 variants
// (see events-and-streaming.md). Rather than model every variant
// statically, this struct keeps the discriminator fields (ID, Type,
// ProcessedAt) and stashes the full event JSON in RawData. Per-event
// extraction lives on dedicated helpers (AgentMessageText, ToolUseName,
// StopReasonType, ErrorMessage); callers that need richer access can
// unmarshal RawData themselves.
type SessionEvent struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	ProcessedAt *time.Time      `json:"processed_at,omitempty"`
	RawData     json.RawMessage `json:"-"`
}

// UnmarshalJSON captures the full event body so per-variant helpers can
// re-parse on demand, while still populating the common ID / Type /
// ProcessedAt discriminator fields.
func (e *SessionEvent) UnmarshalJSON(data []byte) error {
	var meta struct {
		ID          string     `json:"id"`
		Type        string     `json:"type"`
		ProcessedAt *time.Time `json:"processed_at"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	e.ID = meta.ID
	e.Type = meta.Type
	e.ProcessedAt = meta.ProcessedAt
	e.RawData = append(e.RawData[:0], data...)
	return nil
}

// AgentMessageText concatenates the `text` field of every "text" block in
// an `agent.message` event's `content[]`. Returns an empty string with no
// error if there are no text blocks (e.g. a message with only tool-use
// content). Returns an error only if RawData itself is malformed.
func (e *SessionEvent) AgentMessageText() (string, error) {
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(e.RawData, &payload); err != nil {
		return "", fmt.Errorf("session event %s: unmarshal agent.message: %w", e.ID, err)
	}
	var sb strings.Builder
	for _, block := range payload.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String(), nil
}

// ToolUseName extracts the `name` field of an `agent.tool_use` (or
// `agent.mcp_tool_use` / `agent.custom_tool_use`) event. Returns an empty
// string if the field is absent.
func (e *SessionEvent) ToolUseName() (string, error) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(e.RawData, &payload); err != nil {
		return "", fmt.Errorf("session event %s: unmarshal tool_use: %w", e.ID, err)
	}
	return payload.Name, nil
}

// StopReasonType extracts `stop_reason.type` from a `session.status_idle`
// event (e.g. "end_turn", "requires_action"). Returns an empty string if
// no stop_reason is set on the event.
func (e *SessionEvent) StopReasonType() (string, error) {
	var payload struct {
		StopReason *struct {
			Type string `json:"type"`
		} `json:"stop_reason"`
	}
	if err := json.Unmarshal(e.RawData, &payload); err != nil {
		return "", fmt.Errorf("session event %s: unmarshal status_idle: %w", e.ID, err)
	}
	if payload.StopReason == nil {
		return "", nil
	}
	return payload.StopReason.Type, nil
}

// ErrorMessage extracts the inner error message from a `session.error`
// event. Returns an empty string if the field is absent.
func (e *SessionEvent) ErrorMessage() (string, error) {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(e.RawData, &payload); err != nil {
		return "", fmt.Errorf("session event %s: unmarshal session.error: %w", e.ID, err)
	}
	return payload.Error.Message, nil
}

// OutcomeResult extracts the top-level `result` field from a
// `span.outcome_evaluation_end` event — the verdict of one define_outcome
// grading iteration (e.g. "satisfied", "max_iterations_reached", "failed").
// Returns an empty string if the field is absent.
func (e *SessionEvent) OutcomeResult() (string, error) {
	var payload struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(e.RawData, &payload); err != nil {
		return "", fmt.Errorf("session event %s: unmarshal outcome_evaluation_end: %w", e.ID, err)
	}
	return payload.Result, nil
}

// JudgeRequest is the input to JudgeVerdict. SystemPrompt + UserPrompt
// are mapped onto the Messages API as `system` + a single user message.
type JudgeRequest struct {
	Model        string
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
}

// JudgeResult is the structured JSON the judge model is expected to
// produce as its only content block, plus the Messages API's billing
// usage block from the same response. Verdict must be exactly "PASS"
// or "FAIL"; anything else is treated as a malformed response.
//
// Usage is populated from the Messages API response envelope, NOT from
// the judge model's own content. It is nil if the API did not return
// a usage block (an extremely unlikely upstream regression — kept as a
// pointer so callers can detect that case).
type JudgeResult struct {
	Verdict string      `json:"verdict"`
	Reason  string      `json:"reason"`
	Usage   *JudgeUsage `json:"-"`
}

// JudgeUsage is the input/output token count reported by the Messages
// API for a JudgeVerdict call. Fields mirror the standard Messages API
// usage shape; cache-related fields are omitted because the judge
// requests are short, non-cached, and one-shot.
type JudgeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Deployment is the read shape returned by GET /v1/deployments/{id}.
//
// A deployment binds an agent to an environment, vaults, mounted resources,
// initial events, and an optional cron schedule. Each scheduled (or manual)
// fire produces a DeploymentRun audit record.
//
// `status` is `active` or `paused` ONLY — an archived deployment is identified
// by a non-null ArchivedAt, not a status value. A deployment auto-pauses on
// certain errors (see PausedReason); `session_rate_limited_error` is the
// notable exception that does NOT pause (the schedule keeps firing).
type Deployment struct {
	ID            string                   `json:"id"`
	Type          string                   `json:"type"`
	Name          string                   `json:"name"`
	Agent         DeploymentAgentRef       `json:"agent"`
	EnvironmentID string                   `json:"environment_id"`
	Description   *string                  `json:"description"`
	Metadata      map[string]string        `json:"metadata"`
	InitialEvents []DeploymentInitialEvent `json:"initial_events"`
	Resources     []DeploymentResource     `json:"resources"`
	Schedule      *DeploymentSchedule      `json:"schedule"`
	VaultIDs      []string                 `json:"vault_ids"`
	Status        string                   `json:"status"`
	PausedReason  *DeploymentPausedReason  `json:"paused_reason"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
	ArchivedAt    *time.Time               `json:"archived_at"`
}

// DeploymentAgentRef is the resolved agent reference the API always returns
// for a deployment's `agent` field: id plus the concrete version it pinned.
// On create/update the API also accepts a bare id string; this provider sends
// the string form (see DeploymentCreateRequest.Agent).
type DeploymentAgentRef struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Version int    `json:"version"`
}

// DeploymentInitialEvent is one entry of `initial_events` (1-50 per
// deployment). Discriminated union on Type:
//
//   - "user.message"        — Content carries text/image/document blocks.
//   - "system.message"      — Content carries text blocks (privileged context).
//   - "user.define_outcome" — Description (task), Rubric, MaxIterations.
//
// Content blocks are kept as raw JSON: the block source shapes are themselves
// nested unions (base64/url/file image and document sources) that the client
// passes through verbatim rather than enumerating.
type DeploymentInitialEvent struct {
	Type string `json:"type"`

	// user.message + system.message
	Content []json.RawMessage `json:"content,omitempty"`

	// user.define_outcome
	Description   string            `json:"description,omitempty"`
	Rubric        *DeploymentRubric `json:"rubric,omitempty"`
	MaxIterations *int              `json:"max_iterations,omitempty"`
}

// DeploymentRubric is the `rubric` of a user.define_outcome event: either an
// uploaded file (Type "file", FileID) or inline text (Type "text", Content).
type DeploymentRubric struct {
	Type    string `json:"type"`
	FileID  string `json:"file_id,omitempty"`
	Content string `json:"content,omitempty"`
}

// DeploymentResource is one entry of `resources` (max 500). Discriminated
// union on Type:
//
//   - "github_repository" — URL + write-only AuthorizationToken + optional
//     Checkout + MountPath.
//   - "file"              — FileID + optional MountPath.
//   - "memory_store"      — MemoryStoreID + optional Access + Instructions.
//
// AuthorizationToken is write-only upstream: it is accepted on create/update
// but NEVER returned on read. The Terraform resource must treat it as a
// write-only secret (never persisted to state).
type DeploymentResource struct {
	Type string `json:"type"`

	// github_repository
	URL                string              `json:"url,omitempty"`
	AuthorizationToken string              `json:"authorization_token,omitempty"`
	Checkout           *DeploymentCheckout `json:"checkout,omitempty"`
	MountPath          string              `json:"mount_path,omitempty"`

	// file
	FileID string `json:"file_id,omitempty"`

	// memory_store
	MemoryStoreID string `json:"memory_store_id,omitempty"`
	Access        string `json:"access,omitempty"`
	Instructions  string `json:"instructions,omitempty"`
}

// DeploymentCheckout pins a github_repository resource to a branch (Type
// "branch", Name) or a commit (Type "commit", SHA).
type DeploymentCheckout struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	SHA  string `json:"sha,omitempty"`
}

// DeploymentSchedule is the optional cron schedule. Expression is a 5-field
// POSIX cron string (no "@daily"-style shortcuts); Timezone is an IANA name.
// LastRunAt and UpcomingRunsAt are read-only enrichment the API adds on read.
type DeploymentSchedule struct {
	Type           string      `json:"type"`
	Expression     string      `json:"expression"`
	Timezone       string      `json:"timezone"`
	LastRunAt      *time.Time  `json:"last_run_at,omitempty"`
	UpcomingRunsAt []time.Time `json:"upcoming_runs_at,omitempty"`
}

// DeploymentPausedReason explains why a deployment is paused: a manual pause
// (Type "manual") or an automatic error pause (Type "error", Error set).
type DeploymentPausedReason struct {
	Type  string                 `json:"type"`
	Error *DeploymentPausedError `json:"error,omitempty"`
}

// DeploymentPausedError is the typed error that auto-paused a deployment. Type
// is one of the run-error taxonomy values (e.g. "vault_not_found_error").
type DeploymentPausedError struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

// DeploymentCreateRequest is the POST /v1/deployments body.
//
// Agent is raw JSON so the caller controls the string-vs-object form; this
// provider sends a bare id string (JSON-encoded), pinning the agent's latest
// version. Required: Name, Agent, EnvironmentID, InitialEvents (1-50).
type DeploymentCreateRequest struct {
	Name          string                   `json:"name"`
	Agent         json.RawMessage          `json:"agent"`
	EnvironmentID string                   `json:"environment_id"`
	InitialEvents []DeploymentInitialEvent `json:"initial_events"`
	Description   *string                  `json:"description,omitempty"`
	Metadata      map[string]string        `json:"metadata,omitempty"`
	Resources     []DeploymentResource     `json:"resources,omitempty"`
	Schedule      *DeploymentSchedule      `json:"schedule,omitempty"`
	VaultIDs      []string                 `json:"vault_ids,omitempty"`
}

// DeploymentUpdateRequest is the POST /v1/deployments/{id} body. Every field
// is optional; an omitted field is left unchanged. There is no version/etag —
// updates are last-write-wins.
//
// Nullable-clear semantics use json.RawMessage so the caller can send the
// literal `null` to clear (Description, Schedule). Metadata uses
// map[string]*string: a nil value for a key deletes that key server-side.
// Slice pointers (InitialEvents, Resources, VaultIDs) distinguish "unchanged"
// (nil) from "replace with this exact list" (non-nil, possibly empty).
type DeploymentUpdateRequest struct {
	Name          *string                   `json:"name,omitempty"`
	Agent         json.RawMessage           `json:"agent,omitempty"`
	EnvironmentID *string                   `json:"environment_id,omitempty"`
	InitialEvents *[]DeploymentInitialEvent `json:"initial_events,omitempty"`
	Description   json.RawMessage           `json:"description,omitempty"`
	Metadata      map[string]*string        `json:"metadata,omitempty"`
	Resources     *[]DeploymentResource     `json:"resources,omitempty"`
	Schedule      json.RawMessage           `json:"schedule,omitempty"`
	VaultIDs      *[]string                 `json:"vault_ids,omitempty"`
}

// DeploymentList is the cursor-paginated envelope for GET /v1/deployments.
// Unlike the before_id/after_id envelope (ListResponse) used by agents and
// environments, deployments and deployment_runs paginate with an opaque
// `page` cursor echoed back as `next_page`.
type DeploymentList struct {
	Data     []Deployment `json:"data"`
	NextPage *string      `json:"next_page"`
}

// DeploymentRun is one append-only audit record of a deployment fire. Exactly
// one of SessionID (success) or Error (failure) is non-null.
type DeploymentRun struct {
	ID             string                   `json:"id"`
	Type           string                   `json:"type"`
	Agent          DeploymentAgentRef       `json:"agent"`
	DeploymentID   string                   `json:"deployment_id"`
	CreatedAt      time.Time                `json:"created_at"`
	SessionID      *string                  `json:"session_id"`
	Error          *DeploymentRunError      `json:"error"`
	TriggerContext DeploymentTriggerContext `json:"trigger_context"`
}

// DeploymentRunError is the typed failure reason on a run. Type is one of the
// 15 run-error taxonomy values; Message is human-readable detail.
type DeploymentRunError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// DeploymentTriggerContext records what fired a run: the cron schedule (Type
// "schedule", ScheduledAt set) or a manual session create (Type "manual").
type DeploymentTriggerContext struct {
	Type        string     `json:"type"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
}

// DeploymentRunList is the cursor-paginated envelope for GET /v1/deployment_runs.
type DeploymentRunList struct {
	Data     []DeploymentRun `json:"data"`
	NextPage *string         `json:"next_page"`
}
