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
	_ datasource.DataSource              = (*skillDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*skillDataSource)(nil)
)

type skillDataSource struct {
	client *client.Client
}

func newSkillDataSource() datasource.DataSource {
	return &skillDataSource{}
}

func (d *skillDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (d *skillDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing skill — either an Anthropic prebuilt skill (e.g. `xlsx`, `pptx`, `docx`, `pdf`) or a custom `skill_*` you previously created. The data source does not upload anything, and works for both `custom` and `anthropic` source types.",
		Attributes: map[string]schema.Attribute{
			"skill_id": schema.StringAttribute{
				MarkdownDescription: "Either a prebuilt short name (e.g. `xlsx`) or a `skill_*` id.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Canonical server-side id. For prebuilts this equals `skill_id`; for custom skills it is the `skill_*` form.",
				Computed:            true,
			},
			"display_title":  schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable skill title."},
			"latest_version": schema.StringAttribute{Computed: true, MarkdownDescription: "Most recent version string. Date form for prebuilts, epoch form for custom skills."},
			"source":         schema.StringAttribute{Computed: true, MarkdownDescription: "Either `anthropic` (prebuilt) or `custom` (user-uploaded)."},
			"created_at":     schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
		},
	}
}

func (d *skillDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *skillDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg skillDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s, err := d.client.GetSkill(ctx, cfg.SkillID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Skill not found", fmt.Sprintf("no skill with id %q", cfg.SkillID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read skill", err.Error())
		return
	}

	state := skillDataSourceModel{
		SkillID:       cfg.SkillID,
		ID:            types.StringValue(s.ID),
		DisplayTitle:  types.StringValue(s.DisplayTitle),
		LatestVersion: types.StringValue(s.LatestVersion),
		Source:        types.StringValue(s.Source),
		CreatedAt:     types.StringValue(s.CreatedAt.Format(timeFormatRFC3339)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type skillDataSourceModel struct {
	SkillID       types.String `tfsdk:"skill_id"`
	ID            types.String `tfsdk:"id"`
	DisplayTitle  types.String `tfsdk:"display_title"`
	LatestVersion types.String `tfsdk:"latest_version"`
	Source        types.String `tfsdk:"source"`
	CreatedAt     types.String `tfsdk:"created_at"`
}
