package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// agentModel is the Terraform schema representation of a Claude Managed Agents
// agent.
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

	Tools      types.List   `tfsdk:"tools"`
	McpServers types.List   `tfsdk:"mcp_servers"`
	Skills     types.List   `tfsdk:"skills"`
	Multiagent types.Object `tfsdk:"multiagent"`
}

// deploymentModel is the Terraform schema representation of a Claude Managed
// Agents deployment.
type deploymentModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Agent         types.String `tfsdk:"agent"`
	AgentVersion  types.Int64  `tfsdk:"agent_version"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Description   types.String `tfsdk:"description"`
	Metadata      types.Map    `tfsdk:"metadata"`
	VaultIDs      types.List   `tfsdk:"vault_ids"`
	DesiredStatus types.String `tfsdk:"desired_status"`
	Status        types.String `tfsdk:"status"`
	CreatedAt     types.String `tfsdk:"created_at"`
	ArchivedAt    types.String `tfsdk:"archived_at"`

	InitialEvents types.List   `tfsdk:"initial_events"`
	Resources     types.List   `tfsdk:"resources"`
	Schedule      types.Object `tfsdk:"schedule"`
	PausedReason  types.Object `tfsdk:"paused_reason"`
}

// deploymentRunsModel is the Terraform schema representation of the
// deployment_runs data source: input filters plus the computed `runs` list.
type deploymentRunsModel struct {
	DeploymentID types.String `tfsdk:"deployment_id"`
	TriggerType  types.String `tfsdk:"trigger_type"`
	HasError     types.Bool   `tfsdk:"has_error"`
	Limit        types.Int64  `tfsdk:"limit"`
	Runs         types.List   `tfsdk:"runs"`
}

// providerModel is the Terraform schema representation of the provider block.
type providerModel struct {
	APIKey     types.String `tfsdk:"api_key"`
	BaseURL    types.String `tfsdk:"base_url"`
	MaxRetries types.Int64  `tfsdk:"max_retries"`
}
