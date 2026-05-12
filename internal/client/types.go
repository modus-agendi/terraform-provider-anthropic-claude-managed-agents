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
