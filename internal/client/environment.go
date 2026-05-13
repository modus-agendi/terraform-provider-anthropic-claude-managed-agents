package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CreateEnvironment issues POST /v1/environments.
func (c *Client) CreateEnvironment(ctx context.Context, req EnvironmentCreateRequest) (*Environment, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("client.CreateEnvironment: name is required")
	}
	if req.Config.Type == "" {
		return nil, fmt.Errorf("client.CreateEnvironment: config.type is required")
	}
	if req.Config.Networking.Type == "" {
		return nil, fmt.Errorf("client.CreateEnvironment: config.networking.type is required")
	}
	var out Environment
	if err := c.do(ctx, http.MethodPost, "/v1/environments", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEnvironment issues GET /v1/environments/{id}.
func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetEnvironment: id is required")
	}
	var out Environment
	path := "/v1/environments/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveEnvironment issues POST /v1/environments/{id}/archive.
//
// Archive succeeds even when active sessions reference the environment;
// running sessions continue to use the archived config until they finish.
func (c *Client) ArchiveEnvironment(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.ArchiveEnvironment: id is required")
	}
	path := "/v1/environments/" + url.PathEscape(id) + "/archive"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// DeleteEnvironment issues DELETE /v1/environments/{id}.
//
// Delete is rejected with 409 Conflict if any session currently references
// the environment. Callers that want a "best effort" cleanup should call
// DeleteEnvironment first and fall back to ArchiveEnvironment on conflict.
func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.DeleteEnvironment: id is required")
	}
	path := "/v1/environments/" + url.PathEscape(id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// ListEnvironments issues GET /v1/environments.
func (c *Client) ListEnvironments(ctx context.Context, params ListEnvironmentsParams) (*ListResponse[Environment], error) {
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
	path := "/v1/environments"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[Environment]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListEnvironmentsParams holds query parameters for ListEnvironments.
type ListEnvironmentsParams struct {
	Limit           int
	BeforeID        string
	AfterID         string
	IncludeArchived bool
}
