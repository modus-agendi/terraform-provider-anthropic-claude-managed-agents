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
	_ datasource.DataSource              = (*fileDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*fileDataSource)(nil)
)

type fileDataSource struct {
	client *client.Client
}

func newFileDataSource() datasource.DataSource {
	return &fileDataSource{}
}

func (d *fileDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file"
}

func (d *fileDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up metadata for a file uploaded to the Managed Agents Files API. Only metadata is exposed; binary content download is not modeled in this provider.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Required: true, MarkdownDescription: "File id (`file_*`)."},
			"filename":   schema.StringAttribute{Computed: true, MarkdownDescription: "Original filename at upload time."},
			"size_bytes": schema.Int64Attribute{Computed: true, MarkdownDescription: "File size in bytes."},
			"mime_type":  schema.StringAttribute{Computed: true, MarkdownDescription: "Detected MIME type."},
			"scope_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Identifier of the resource (typically a session) the file is scoped to."},
			"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
		},
	}
}

func (d *fileDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *fileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg fileDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	f, err := d.client.GetFile(ctx, cfg.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("File not found", fmt.Sprintf("no file with id %q", cfg.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read file", err.Error())
		return
	}

	state := fileDataSourceModel{
		ID:        types.StringValue(f.ID),
		Filename:  types.StringValue(f.Filename),
		SizeBytes: types.Int64Value(f.SizeBytes),
		MimeType:  types.StringValue(f.MimeType),
		ScopeID:   types.StringValue(f.ScopeID),
		CreatedAt: types.StringValue(f.CreatedAt.Format(timeFormatRFC3339)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type fileDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Filename  types.String `tfsdk:"filename"`
	SizeBytes types.Int64  `tfsdk:"size_bytes"`
	MimeType  types.String `tfsdk:"mime_type"`
	ScopeID   types.String `tfsdk:"scope_id"`
	CreatedAt types.String `tfsdk:"created_at"`
}
