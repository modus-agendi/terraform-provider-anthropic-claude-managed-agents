package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CreateAgent issues POST /v1/agents.
func (c *Client) CreateAgent(ctx context.Context, req AgentCreateRequest) (*Agent, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("client.CreateAgent: name is required")
	}
	if req.Model == "" {
		return nil, fmt.Errorf("client.CreateAgent: model is required")
	}
	var out Agent
	if err := c.do(ctx, http.MethodPost, "/v1/agents", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgent issues GET /v1/agents/{id}.
func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetAgent: id is required")
	}
	var out Agent
	path := "/v1/agents/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAgent issues POST /v1/agents/{id} with optimistic concurrency control.
// The caller must provide req.Version (the current server version).
func (c *Client) UpdateAgent(ctx context.Context, id string, req AgentUpdateRequest) (*Agent, error) {
	if id == "" {
		return nil, fmt.Errorf("client.UpdateAgent: id is required")
	}
	if req.Version <= 0 {
		return nil, fmt.Errorf("client.UpdateAgent: version is required and must be > 0")
	}
	var out Agent
	path := "/v1/agents/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveAgent issues POST /v1/agents/{id}/archive. The API has no DELETE
// endpoint for agents; archive is the terminal lifecycle operation.
func (c *Client) ArchiveAgent(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.ArchiveAgent: id is required")
	}
	path := "/v1/agents/" + url.PathEscape(id) + "/archive"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// ListAgents issues GET /v1/agents.
func (c *Client) ListAgents(ctx context.Context, params ListAgentsParams) (*ListResponse[Agent], error) {
	q := url.Values{}
	if params.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.BeforeID != "" {
		q.Set("before_id", params.BeforeID)
	}
	if params.AfterID != "" {
		q.Set("after_id", params.AfterID)
	}
	if params.IncludeArchived {
		q.Set("include_archived", "true")
	}
	path := "/v1/agents"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[Agent]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAgentsParams holds query parameters for ListAgents.
type ListAgentsParams struct {
	Limit           int
	BeforeID        string
	AfterID         string
	IncludeArchived bool
}
