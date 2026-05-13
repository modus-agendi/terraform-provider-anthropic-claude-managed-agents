package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GetFile issues GET /v1/files/{id}.
//
// This returns file metadata only. The provider does not model the
// binary `/files/{id}/content` endpoint.
func (c *Client) GetFile(ctx context.Context, id string) (*File, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetFile: id is required")
	}
	var out File
	path := "/v1/files/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
