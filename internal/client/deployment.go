package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// CreateDeployment issues POST /v1/deployments.
func (c *Client) CreateDeployment(ctx context.Context, req DeploymentCreateRequest) (*Deployment, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("client.CreateDeployment: name is required")
	}
	if len(req.Agent) == 0 {
		return nil, fmt.Errorf("client.CreateDeployment: agent is required")
	}
	if req.EnvironmentID == "" {
		return nil, fmt.Errorf("client.CreateDeployment: environment_id is required")
	}
	if n := len(req.InitialEvents); n < 1 || n > 50 {
		return nil, fmt.Errorf("client.CreateDeployment: initial_events must have 1-50 entries, got %d", n)
	}
	var out Deployment
	if err := c.do(ctx, http.MethodPost, "/v1/deployments", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDeployment issues GET /v1/deployments/{id}.
func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetDeployment: id is required")
	}
	var out Deployment
	path := "/v1/deployments/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDeployment issues POST /v1/deployments/{id}. The deployments API has
// no version/etag field; updates are last-write-wins. (The endpoint rejects
// PATCH with 405 — update is POST, mirroring the agents API.)
func (c *Client) UpdateDeployment(ctx context.Context, id string, req DeploymentUpdateRequest) (*Deployment, error) {
	if id == "" {
		return nil, fmt.Errorf("client.UpdateDeployment: id is required")
	}
	var out Deployment
	path := "/v1/deployments/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveDeployment issues POST /v1/deployments/{id}/archive. Archive is the
// terminal lifecycle operation (one-way); `terraform destroy` calls this.
func (c *Client) ArchiveDeployment(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.ArchiveDeployment: id is required")
	}
	path := "/v1/deployments/" + url.PathEscape(id) + "/archive"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// PauseDeployment issues POST /v1/deployments/{id}/pause and returns the
// updated deployment (status "paused", paused_reason type "manual").
func (c *Client) PauseDeployment(ctx context.Context, id string) (*Deployment, error) {
	if id == "" {
		return nil, fmt.Errorf("client.PauseDeployment: id is required")
	}
	var out Deployment
	path := "/v1/deployments/" + url.PathEscape(id) + "/pause"
	if err := c.do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResumeDeployment issues POST /v1/deployments/{id}/unpause and returns the
// updated deployment (status "active", paused_reason cleared). The endpoint is
// "/unpause", not "/resume" (the latter 404s).
func (c *Client) ResumeDeployment(ctx context.Context, id string) (*Deployment, error) {
	if id == "" {
		return nil, fmt.Errorf("client.ResumeDeployment: id is required")
	}
	var out Deployment
	path := "/v1/deployments/" + url.PathEscape(id) + "/unpause"
	if err := c.do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDeployments issues GET /v1/deployments. Pagination is cursor-based:
// pass the previous response's NextPage as params.Page to fetch the next page.
func (c *Client) ListDeployments(ctx context.Context, params ListDeploymentsParams) (*DeploymentList, error) {
	q := url.Values{}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Page != "" {
		q.Set("page", params.Page)
	}
	path := "/v1/deployments"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out DeploymentList
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDeploymentsParams holds query parameters for ListDeployments.
type ListDeploymentsParams struct {
	Limit int
	Page  string
}

// ListDeploymentRuns issues GET /v1/deployment_runs. All filters are optional;
// omit DeploymentID to list runs across every deployment in the workspace.
func (c *Client) ListDeploymentRuns(ctx context.Context, params ListDeploymentRunsParams) (*DeploymentRunList, error) {
	q := url.Values{}
	if params.DeploymentID != "" {
		q.Set("deployment_id", params.DeploymentID)
	}
	if params.TriggerType != "" {
		q.Set("trigger_type", params.TriggerType)
	}
	if params.HasError != nil {
		q.Set("has_error", strconv.FormatBool(*params.HasError))
	}
	if !params.CreatedAtGt.IsZero() {
		q.Set("created_at_gt", params.CreatedAtGt.Format(time.RFC3339))
	}
	if !params.CreatedAtGte.IsZero() {
		q.Set("created_at_gte", params.CreatedAtGte.Format(time.RFC3339))
	}
	if !params.CreatedAtLt.IsZero() {
		q.Set("created_at_lt", params.CreatedAtLt.Format(time.RFC3339))
	}
	if !params.CreatedAtLte.IsZero() {
		q.Set("created_at_lte", params.CreatedAtLte.Format(time.RFC3339))
	}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Page != "" {
		q.Set("page", params.Page)
	}
	path := "/v1/deployment_runs"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out DeploymentRunList
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDeploymentRunsParams holds query parameters for ListDeploymentRuns.
// TriggerType is "schedule" or "manual"; HasError filters to failed (true) or
// successful (false) runs; the CreatedAt* fields bound the time range.
type ListDeploymentRunsParams struct {
	DeploymentID string
	TriggerType  string
	HasError     *bool
	CreatedAtGt  time.Time
	CreatedAtGte time.Time
	CreatedAtLt  time.Time
	CreatedAtLte time.Time
	Limit        int
	Page         string
}

// GetDeploymentRun issues GET /v1/deployment_runs/{id}.
func (c *Client) GetDeploymentRun(ctx context.Context, id string) (*DeploymentRun, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetDeploymentRun: id is required")
	}
	var out DeploymentRun
	path := "/v1/deployment_runs/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
