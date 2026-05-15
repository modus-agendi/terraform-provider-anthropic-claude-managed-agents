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
	_ datasource.DataSource              = (*memoryStoreDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*memoryStoreDataSource)(nil)
)

type memoryStoreDataSource struct {
	client *client.Client
}

func newMemoryStoreDataSource() datasource.DataSource {
	return &memoryStoreDataSource{}
}

func (d *memoryStoreDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory_store"
}

func (d *memoryStoreDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Claude Managed Agents memory store by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `memstore_01ABC...`).",
				Required:            true,
			},
			"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Memory store name."},
			"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Description, or null."},
			"created_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"updated_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 last-modified timestamp."},
			"archived_at": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 archive timestamp, or null if active."},
		},
	}
}

func (d *memoryStoreDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *memoryStoreDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg memoryStoreDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s, err := d.client.GetMemoryStore(ctx, cfg.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Memory store not found", fmt.Sprintf("no memory_store with id %q", cfg.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read memory store", err.Error())
		return
	}

	state := memoryStoreDataSourceModel{
		ID:        types.StringValue(s.ID),
		Name:      types.StringValue(s.Name),
		CreatedAt: types.StringValue(s.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt: types.StringValue(s.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if s.Description != nil && *s.Description != "" {
		state.Description = types.StringValue(*s.Description)
	} else {
		state.Description = types.StringNull()
	}
	if s.ArchivedAt != nil {
		state.ArchivedAt = types.StringValue(s.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		state.ArchivedAt = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// memoryStoreDataSourceModel mirrors the data-source schema. It omits the
// resource-only attribute `delete_on_destroy`.
type memoryStoreDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	ArchivedAt  types.String `tfsdk:"archived_at"`
}
