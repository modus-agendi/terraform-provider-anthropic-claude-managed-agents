package client

import (
	"encoding/json"
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
