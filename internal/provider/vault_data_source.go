package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ datasource.DataSource              = (*vaultDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*vaultDataSource)(nil)
)

type vaultDataSource struct {
	client *client.Client
}

func newVaultDataSource() datasource.DataSource {
	return &vaultDataSource{}
}

func (d *vaultDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

func (d *vaultDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Claude Managed Agents vault by id.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Required: true, MarkdownDescription: "Server-assigned identifier (e.g. `vlt_01ABC...`)."},
			"display_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Vault display name."},
			"metadata":     schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Vault metadata map."},
			"created_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"updated_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 last-modified timestamp."},
			"archived_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 archive timestamp, or null."},
		},
	}
}

func (d *vaultDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vaultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg vaultDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := d.client.GetVault(ctx, cfg.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Vault not found", fmt.Sprintf("no vault with id %q", cfg.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read vault", err.Error())
		return
	}

	state := vaultDataSourceModel{
		ID:          types.StringValue(v.ID),
		DisplayName: types.StringValue(v.DisplayName),
		CreatedAt:   types.StringValue(v.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt:   types.StringValue(v.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if v.ArchivedAt != nil {
		state.ArchivedAt = types.StringValue(v.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		state.ArchivedAt = types.StringNull()
	}
	mdMap, d2 := stringMapToMap(ctx, v.Metadata)
	resp.Diagnostics.Append(d2...)
	state.Metadata = mdMap
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type vaultDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Metadata    types.Map    `tfsdk:"metadata"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	ArchivedAt  types.String `tfsdk:"archived_at"`
}
