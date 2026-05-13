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
// Nested config fields that the v0.1 provider does not yet expose as HCL
// (tools, mcp_servers, skills, multiagent) are decoded into json.RawMessage
// so the provider can round-trip them on update without losing user
// configuration set via the API directly.
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

	Tools      json.RawMessage `json:"tools,omitempty"`
	McpServers json.RawMessage `json:"mcp_servers,omitempty"`
	Skills     json.RawMessage `json:"skills,omitempty"`
	Multiagent json.RawMessage `json:"multiagent,omitempty"`
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

	Tools      json.RawMessage `json:"tools,omitempty"`
	McpServers json.RawMessage `json:"mcp_servers,omitempty"`
	Skills     json.RawMessage `json:"skills,omitempty"`
	Multiagent json.RawMessage `json:"multiagent,omitempty"`
}

// AgentUpdateRequest is the body for POST /v1/agents/{id}.
//
// Version is required (optimistic concurrency). Use pointer fields for
// scalars so the marshalled JSON can distinguish "leave unchanged" (nil)
// from "set to null" (a typed nil string pointer is impossible in JSON, so
// we wrap with json.RawMessage for the null-clear case).
type AgentUpdateRequest struct {
	Version     int               `json:"version"`
	Name        *string           `json:"name,omitempty"`
	Model       *string           `json:"model,omitempty"`
	System      json.RawMessage   `json:"system,omitempty"`
	Description json.RawMessage   `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	Tools      json.RawMessage `json:"tools,omitempty"`
	McpServers json.RawMessage `json:"mcp_servers,omitempty"`
	Skills     json.RawMessage `json:"skills,omitempty"`
	Multiagent json.RawMessage `json:"multiagent,omitempty"`
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

// MemoryStoreUpdateRequest is the body for POST /v1/memory_stores/{id}.
//
// Name and Description use pointer / raw-message semantics matching agent
// updates: a nil Name leaves it unchanged; a non-nil Description that is
// the JSON literal `null` clears the field.
type MemoryStoreUpdateRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
}
