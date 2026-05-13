package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CreateVault issues POST /v1/vaults.
func (c *Client) CreateVault(ctx context.Context, req VaultCreateRequest) (*Vault, error) {
	if req.DisplayName == "" {
		return nil, fmt.Errorf("client.CreateVault: display_name is required")
	}
	var out Vault
	if err := c.do(ctx, http.MethodPost, "/v1/vaults", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVault issues GET /v1/vaults/{id}.
func (c *Client) GetVault(ctx context.Context, id string) (*Vault, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetVault: id is required")
	}
	var out Vault
	path := "/v1/vaults/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateVault issues POST /v1/vaults/{id}.
func (c *Client) UpdateVault(ctx context.Context, id string, req VaultUpdateRequest) (*Vault, error) {
	if id == "" {
		return nil, fmt.Errorf("client.UpdateVault: id is required")
	}
	var out Vault
	path := "/v1/vaults/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveVault issues POST /v1/vaults/{id}/archive. Cascades to credentials.
func (c *Client) ArchiveVault(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.ArchiveVault: id is required")
	}
	path := "/v1/vaults/" + url.PathEscape(id) + "/archive"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// DeleteVault issues DELETE /v1/vaults/{id}. Hard delete; cascades.
func (c *Client) DeleteVault(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.DeleteVault: id is required")
	}
	path := "/v1/vaults/" + url.PathEscape(id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// ListVaults issues GET /v1/vaults.
func (c *Client) ListVaults(ctx context.Context, params ListVaultsParams) (*ListResponse[Vault], error) {
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
	path := "/v1/vaults"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[Vault]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListVaultsParams holds query parameters for ListVaults.
type ListVaultsParams struct {
	Limit           int
	BeforeID        string
	AfterID         string
	IncludeArchived bool
}

// CreateVaultCredential issues POST /v1/vaults/{vault_id}/credentials.
func (c *Client) CreateVaultCredential(ctx context.Context, vaultID string, req VaultCredentialCreateRequest) (*VaultCredential, error) {
	if vaultID == "" {
		return nil, fmt.Errorf("client.CreateVaultCredential: vault_id is required")
	}
	if req.DisplayName == "" {
		return nil, fmt.Errorf("client.CreateVaultCredential: display_name is required")
	}
	if req.Auth == nil {
		return nil, fmt.Errorf("client.CreateVaultCredential: auth is required")
	}
	var out VaultCredential
	path := "/v1/vaults/" + url.PathEscape(vaultID) + "/credentials"
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVaultCredential issues GET /v1/vaults/{vault_id}/credentials/{cred_id}.
func (c *Client) GetVaultCredential(ctx context.Context, vaultID, credID string) (*VaultCredential, error) {
	if vaultID == "" {
		return nil, fmt.Errorf("client.GetVaultCredential: vault_id is required")
	}
	if credID == "" {
		return nil, fmt.Errorf("client.GetVaultCredential: credential id is required")
	}
	var out VaultCredential
	path := "/v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credID)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateVaultCredential issues POST /v1/vaults/{vault_id}/credentials/{cred_id}.
func (c *Client) UpdateVaultCredential(ctx context.Context, vaultID, credID string, req VaultCredentialUpdateRequest) (*VaultCredential, error) {
	if vaultID == "" {
		return nil, fmt.Errorf("client.UpdateVaultCredential: vault_id is required")
	}
	if credID == "" {
		return nil, fmt.Errorf("client.UpdateVaultCredential: credential id is required")
	}
	var out VaultCredential
	path := "/v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credID)
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveVaultCredential issues POST .../archive. Purges the secret payload.
func (c *Client) ArchiveVaultCredential(ctx context.Context, vaultID, credID string) error {
	if vaultID == "" || credID == "" {
		return fmt.Errorf("client.ArchiveVaultCredential: vault_id and credential id are required")
	}
	path := "/v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credID) + "/archive"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// DeleteVaultCredential issues DELETE .../credentials/{cred_id}. Hard delete.
func (c *Client) DeleteVaultCredential(ctx context.Context, vaultID, credID string) error {
	if vaultID == "" || credID == "" {
		return fmt.Errorf("client.DeleteVaultCredential: vault_id and credential id are required")
	}
	path := "/v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(credID)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// ListVaultCredentials issues GET /v1/vaults/{vault_id}/credentials.
func (c *Client) ListVaultCredentials(ctx context.Context, vaultID string, params ListVaultCredentialsParams) (*ListResponse[VaultCredential], error) {
	if vaultID == "" {
		return nil, fmt.Errorf("client.ListVaultCredentials: vault_id is required")
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
	if params.IncludeArchived {
		q.Set("include_archived", "true")
	}
	path := "/v1/vaults/" + url.PathEscape(vaultID) + "/credentials"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[VaultCredential]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListVaultCredentialsParams holds query parameters for ListVaultCredentials.
type ListVaultCredentialsParams struct {
	Limit           int
	BeforeID        string
	AfterID         string
	IncludeArchived bool
}
