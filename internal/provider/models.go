package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// agentModel is the Terraform schema representation of a Claude Managed Agents
// agent. Mirrors the fields the v0.1 provider exposes in HCL.
type agentModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Model       types.String `tfsdk:"model"`
	System      types.String `tfsdk:"system"`
	Description types.String `tfsdk:"description"`
	Metadata    types.Map    `tfsdk:"metadata"`
	Version     types.Int64  `tfsdk:"version"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	ArchivedAt  types.String `tfsdk:"archived_at"`
}

// providerModel is the Terraform schema representation of the provider block.
type providerModel struct {
	APIKey     types.String `tfsdk:"api_key"`
	BaseURL    types.String `tfsdk:"base_url"`
	MaxRetries types.Int64  `tfsdk:"max_retries"`
}
