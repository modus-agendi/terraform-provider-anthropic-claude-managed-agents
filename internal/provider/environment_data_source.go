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
	_ datasource.DataSource              = (*environmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*environmentDataSource)(nil)
)

type environmentDataSource struct {
	client *client.Client
}

func newEnvironmentDataSource() datasource.DataSource {
	return &environmentDataSource{}
}

func (d *environmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *environmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Claude Managed Agents environment by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `env_01ABC...`).",
				Required:            true,
			},
			"name": schema.StringAttribute{Computed: true, MarkdownDescription: "Environment name."},
			"config": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Sandbox configuration.",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{Computed: true, MarkdownDescription: "Config discriminator (currently always `cloud`)."},
					"packages": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Per-package-manager install lists.",
						Attributes: map[string]schema.Attribute{
							"apt":   schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "APT packages."},
							"cargo": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Cargo crates."},
							"gem":   schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "RubyGems."},
							"go":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Go modules."},
							"npm":   schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "npm packages."},
							"pip":   schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "pip packages."},
						},
					},
					"networking": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Outbound networking policy.",
						Attributes: map[string]schema.Attribute{
							"type":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Either `unrestricted` or `limited`."},
							"allowed_hosts":          schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Hostnames the agent may reach."},
							"allow_mcp_servers":      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether MCP-server traffic is allowed under `limited` networking."},
							"allow_package_managers": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether package-manager installs are allowed under `limited` networking."},
						},
					},
				},
			},
			"created_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"updated_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 last-modified timestamp."},
			"archived_at": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 archive timestamp, or null if active."},
		},
	}
}

func (d *environmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *environmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg environmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := d.client.GetEnvironment(ctx, cfg.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Environment not found", fmt.Sprintf("no environment with id %q", cfg.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}

	state, diags := environmentFromAPI(ctx, env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
