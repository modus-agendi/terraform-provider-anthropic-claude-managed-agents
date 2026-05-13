package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CreateMemoryStore issues POST /v1/memory_stores.
func (c *Client) CreateMemoryStore(ctx context.Context, req MemoryStoreCreateRequest) (*MemoryStore, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("client.CreateMemoryStore: name is required")
	}
	var out MemoryStore
	if err := c.do(ctx, http.MethodPost, "/v1/memory_stores", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMemoryStore issues GET /v1/memory_stores/{id}.
func (c *Client) GetMemoryStore(ctx context.Context, id string) (*MemoryStore, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetMemoryStore: id is required")
	}
	var out MemoryStore
	path := "/v1/memory_stores/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateMemoryStore issues POST /v1/memory_stores/{id}.
func (c *Client) UpdateMemoryStore(ctx context.Context, id string, req MemoryStoreUpdateRequest) (*MemoryStore, error) {
	if id == "" {
		return nil, fmt.Errorf("client.UpdateMemoryStore: id is required")
	}
	var out MemoryStore
	path := "/v1/memory_stores/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveMemoryStore issues POST /v1/memory_stores/{id}/archive.
//
// Archive is the safe, audit-preserving disposal: existing memories remain
// queryable but the store no longer accepts new attachments.
func (c *Client) ArchiveMemoryStore(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.ArchiveMemoryStore: id is required")
	}
	path := "/v1/memory_stores/" + url.PathEscape(id) + "/archive"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// DeleteMemoryStore issues DELETE /v1/memory_stores/{id}.
//
// Delete is hard and cascades: every memory and every memory version inside
// the store is removed. Use only when the audit trail is unwanted.
func (c *Client) DeleteMemoryStore(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.DeleteMemoryStore: id is required")
	}
	path := "/v1/memory_stores/" + url.PathEscape(id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// ListMemoryStores issues GET /v1/memory_stores.
func (c *Client) ListMemoryStores(ctx context.Context, params ListMemoryStoresParams) (*ListResponse[MemoryStore], error) {
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
	path := "/v1/memory_stores"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[MemoryStore]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListMemoryStoresParams holds query parameters for ListMemoryStores.
type ListMemoryStoresParams struct {
	Limit           int
	BeforeID        string
	AfterID         string
	IncludeArchived bool
}
