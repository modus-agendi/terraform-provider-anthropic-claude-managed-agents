package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ datasource.DataSource              = (*agentVersionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*agentVersionDataSource)(nil)
)

type agentVersionDataSource struct {
	client *client.Client
}

func newAgentVersionDataSource() datasource.DataSource {
	return &agentVersionDataSource{}
}

func (d *agentVersionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_version"
}

func (d *agentVersionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a specific historical version of an agent. The upstream API only exposes a list endpoint for versions, so this data source pages through the version history and returns the entry whose `version` field matches.",
		Attributes: map[string]schema.Attribute{
			"agent_id":    schema.StringAttribute{Required: true, MarkdownDescription: "Agent id (`agent_*`)."},
			"version":     schema.Int64Attribute{Required: true, MarkdownDescription: "Version number to look up. Server-managed monotonic counter starting at 1."},
			"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Agent name at this version."},
			"model":       schema.StringAttribute{Computed: true, MarkdownDescription: "Model identifier at this version."},
			"system":      schema.StringAttribute{Computed: true, MarkdownDescription: "System prompt at this version, or null."},
			"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Description at this version, or null."},
			"metadata":    schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Metadata at this version."},
			"created_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"updated_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 last-modified timestamp."},
		},
	}
}

func (d *agentVersionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected ProviderData type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *agentVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg agentVersionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := d.client.GetAgentVersion(ctx, cfg.AgentID.ValueString(), int(cfg.Version.ValueInt64()))
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Agent version not found", fmt.Sprintf("agent %q version %d not found", cfg.AgentID.ValueString(), cfg.Version.ValueInt64()))
			return
		}
		resp.Diagnostics.AddError("Failed to read agent version", err.Error())
		return
	}

	state := agentVersionDataSourceModel{
		AgentID:   types.StringValue(v.AgentID),
		Version:   types.Int64Value(int64(v.Version)),
		Name:      types.StringValue(v.Name),
		Model:     types.StringValue(v.Model.ID),
		CreatedAt: types.StringValue(v.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt: types.StringValue(v.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if v.System != nil {
		state.System = types.StringValue(*v.System)
	} else {
		state.System = types.StringNull()
	}
	if v.Description != nil {
		state.Description = types.StringValue(*v.Description)
	} else {
		state.Description = types.StringNull()
	}
	mdMap, d2 := stringMapToMap(ctx, v.Metadata)
	resp.Diagnostics.Append(d2...)
	state.Metadata = mdMap

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type agentVersionDataSourceModel struct {
	AgentID     types.String `tfsdk:"agent_id"`
	Version     types.Int64  `tfsdk:"version"`
	Name        types.String `tfsdk:"name"`
	Model       types.String `tfsdk:"model"`
	System      types.String `tfsdk:"system"`
	Description types.String `tfsdk:"description"`
	Metadata    types.Map    `tfsdk:"metadata"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}
