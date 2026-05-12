package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

var (
	_ datasource.DataSource              = (*agentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*agentDataSource)(nil)
)

type agentDataSource struct {
	client *client.Client
}

func newAgentDataSource() datasource.DataSource {
	return &agentDataSource{}
}

func (d *agentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (d *agentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Claude Managed Agents agent by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `agent_01ABC...`).",
				Required:            true,
			},
			"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Agent name."},
			"model":       schema.StringAttribute{Computed: true, MarkdownDescription: "Model identifier."},
			"system":      schema.StringAttribute{Computed: true, MarkdownDescription: "System prompt, or null."},
			"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Description, or null."},
			"metadata": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType, MarkdownDescription: "Metadata map.",
			},
			"version":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Server-managed version number."},
			"created_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"updated_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 last-modified timestamp."},
			"archived_at": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 archive timestamp, or null if active."},
		},
	}
}

func (d *agentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *agentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg agentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agent, err := d.client.GetAgent(ctx, cfg.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Agent not found", fmt.Sprintf("no agent with id %q", cfg.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read agent", err.Error())
		return
	}

	state := agentFromAPI(ctx, agent, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
