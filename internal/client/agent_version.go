package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ListAgentVersions issues GET /v1/agents/{agent_id}/versions.
func (c *Client) ListAgentVersions(ctx context.Context, agentID string, params ListAgentVersionsParams) (*ListResponse[AgentVersion], error) {
	if agentID == "" {
		return nil, fmt.Errorf("client.ListAgentVersions: agent_id is required")
	}
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
	path := "/v1/agents/" + url.PathEscape(agentID) + "/versions"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[AgentVersion]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgentVersion looks up a specific version of an agent by paging
// through ListAgentVersions and selecting the matching entry. The upstream
// API does not expose a single-version retrieve endpoint, so this method
// is a convenience wrapper over the list endpoint.
func (c *Client) GetAgentVersion(ctx context.Context, agentID string, version int) (*AgentVersion, error) {
	if agentID == "" {
		return nil, fmt.Errorf("client.GetAgentVersion: agent_id is required")
	}
	if version <= 0 {
		return nil, fmt.Errorf("client.GetAgentVersion: version must be > 0")
	}
	cursor := ""
	for {
		page, err := c.ListAgentVersions(ctx, agentID, ListAgentVersionsParams{Limit: 100, AfterID: cursor})
		if err != nil {
			return nil, err
		}
		for i := range page.Data {
			if page.Data[i].Version == version {
				return &page.Data[i], nil
			}
		}
		if !page.HasMore {
			break
		}
		cursor = page.LastID
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Type: "not_found_error", Message: fmt.Sprintf("agent %s has no version %d", agentID, version)}
}

// ListAgentVersionsParams holds query parameters for ListAgentVersions.
type ListAgentVersionsParams struct {
	Limit    int
	BeforeID string
	AfterID  string
}
