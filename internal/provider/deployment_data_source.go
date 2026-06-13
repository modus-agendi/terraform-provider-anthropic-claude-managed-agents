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
	_ datasource.DataSource              = (*deploymentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*deploymentDataSource)(nil)
)

type deploymentDataSource struct {
	client *client.Client
}

func newDeploymentDataSource() datasource.DataSource {
	return &deploymentDataSource{}
}

func (d *deploymentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (d *deploymentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Claude Managed Agents deployment by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `depl_01ABC...`).",
				Required:            true,
			},
			"name":           schema.StringAttribute{Computed: true, MarkdownDescription: "Deployment name."},
			"agent":          schema.StringAttribute{Computed: true, MarkdownDescription: "Agent id."},
			"agent_version":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Resolved agent version."},
			"environment_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Environment id."},
			"description":    schema.StringAttribute{Computed: true, MarkdownDescription: "Description, or null."},
			"metadata": schema.MapAttribute{
				Computed: true, ElementType: types.StringType, MarkdownDescription: "Metadata map.",
			},
			"vault_ids": schema.ListAttribute{
				Computed: true, ElementType: types.StringType, MarkdownDescription: "Mounted vault ids.",
			},
			"desired_status": schema.StringAttribute{Computed: true, MarkdownDescription: "Intended run state (mirrors `status` when read via the data source)."},
			"status":         schema.StringAttribute{Computed: true, MarkdownDescription: "Observed run state: `active` or `paused`."},
			"created_at":     schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"archived_at":    schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 archive timestamp, or null."},
			"initial_events": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Events sent to each session on start.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":        schema.StringAttribute{Computed: true},
						"content":     schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"rubric": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"type":    schema.StringAttribute{Computed: true},
								"file_id": schema.StringAttribute{Computed: true},
								"content": schema.StringAttribute{Computed: true},
							},
						},
						"max_iterations": schema.Int64Attribute{Computed: true},
					},
				},
			},
			"resources": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Mounted resources. `authorization_token` is write-only upstream and always null here.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":                           schema.StringAttribute{Computed: true},
						"url":                            schema.StringAttribute{Computed: true},
						"authorization_token":            schema.StringAttribute{Computed: true},
						"authorization_token_wo_version": schema.Int64Attribute{Computed: true},
						"checkout": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{Computed: true},
								"name": schema.StringAttribute{Computed: true},
								"sha":  schema.StringAttribute{Computed: true},
							},
						},
						"mount_path":      schema.StringAttribute{Computed: true},
						"file_id":         schema.StringAttribute{Computed: true},
						"memory_store_id": schema.StringAttribute{Computed: true},
						"access":          schema.StringAttribute{Computed: true},
						"instructions":    schema.StringAttribute{Computed: true},
					},
				},
			},
			"schedule": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Cron schedule, or null.",
				Attributes: map[string]schema.Attribute{
					"type":       schema.StringAttribute{Computed: true},
					"expression": schema.StringAttribute{Computed: true},
					"timezone":   schema.StringAttribute{Computed: true},
				},
			},
			"paused_reason": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Why the deployment is paused, or null.",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{Computed: true},
					"error": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"type":    schema.StringAttribute{Computed: true},
							"message": schema.StringAttribute{Computed: true},
						},
					},
				},
			},
		},
	}
}

func (d *deploymentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *deploymentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg deploymentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dep, err := d.client.GetDeployment(ctx, cfg.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Deployment not found", fmt.Sprintf("no deployment with id %q", cfg.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read deployment", err.Error())
		return
	}

	// No prior state in a data source: pass null prior resources and
	// desired_status (the helper seeds desired_status from actual status).
	state := deploymentFromAPI(ctx, dep, types.ListNull(types.ObjectType{AttrTypes: deploymentResourceObjectAttrTypes()}), types.StringNull(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
