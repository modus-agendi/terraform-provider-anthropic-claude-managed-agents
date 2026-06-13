package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ datasource.DataSource              = (*deploymentRunsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*deploymentRunsDataSource)(nil)
)

type deploymentRunsDataSource struct {
	client *client.Client
}

func newDeploymentRunsDataSource() datasource.DataSource {
	return &deploymentRunsDataSource{}
}

func (d *deploymentRunsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_runs"
}

func deploymentRunObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":            types.StringType,
		"deployment_id": types.StringType,
		"agent_id":      types.StringType,
		"agent_version": types.Int64Type,
		"session_id":    types.StringType,
		"trigger_type":  types.StringType,
		"scheduled_at":  types.StringType,
		"error_type":    types.StringType,
		"error_message": types.StringType,
		"created_at":    types.StringType,
	}
}

func (d *deploymentRunsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List append-only run records for deployments. Each scheduled or manual fire produces one run; exactly one of `session_id` (success) or `error_type` (failure) is set.",
		Attributes: map[string]schema.Attribute{
			"deployment_id": schema.StringAttribute{
				MarkdownDescription: "Filter to a single deployment. Omit to list runs across all deployments in the workspace.",
				Optional:            true,
			},
			"trigger_type": schema.StringAttribute{
				MarkdownDescription: "Filter by trigger: `schedule` or `manual`. Omit for all.",
				Optional:            true,
			},
			"has_error": schema.BoolAttribute{
				MarkdownDescription: "Filter: `true` for failed runs, `false` for successful runs. Omit for all.",
				Optional:            true,
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of runs to return (default 20, max 1000). Only the first page is fetched.",
				Optional:            true,
			},
			"runs": schema.ListNestedAttribute{
				MarkdownDescription: "The matching run records, newest first.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "Run id (`drun_...`)."},
						"deployment_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Owning deployment id."},
						"agent_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Resolved agent id."},
						"agent_version": schema.Int64Attribute{Computed: true, MarkdownDescription: "Resolved agent version."},
						"session_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Created session id on success, else null."},
						"trigger_type":  schema.StringAttribute{Computed: true, MarkdownDescription: "`schedule` or `manual`."},
						"scheduled_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "Scheduled fire time (schedule trigger only), else null."},
						"error_type":    schema.StringAttribute{Computed: true, MarkdownDescription: "Typed failure reason on failure, else null."},
						"error_message": schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable failure detail, else null."},
						"created_at":    schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 run timestamp."},
					},
				},
			},
		},
	}
}

func (d *deploymentRunsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *deploymentRunsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg deploymentRunsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := client.ListDeploymentRunsParams{
		DeploymentID: cfg.DeploymentID.ValueString(),
		TriggerType:  cfg.TriggerType.ValueString(),
	}
	if !cfg.HasError.IsNull() && !cfg.HasError.IsUnknown() {
		v := cfg.HasError.ValueBool()
		params.HasError = &v
	}
	if !cfg.Limit.IsNull() && !cfg.Limit.IsUnknown() {
		params.Limit = int(cfg.Limit.ValueInt64())
	}

	page, err := d.client.ListDeploymentRuns(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list deployment runs", err.Error())
		return
	}

	objType := types.ObjectType{AttrTypes: deploymentRunObjectAttrTypes()}
	items := make([]attr.Value, 0, len(page.Data))
	for _, run := range page.Data {
		sessionID := types.StringNull()
		if run.SessionID != nil {
			sessionID = types.StringValue(*run.SessionID)
		}
		scheduledAt := types.StringNull()
		if run.TriggerContext.ScheduledAt != nil {
			scheduledAt = types.StringValue(run.TriggerContext.ScheduledAt.Format(timeFormatRFC3339))
		}
		errType, errMsg := types.StringNull(), types.StringNull()
		if run.Error != nil {
			errType = types.StringValue(run.Error.Type)
			errMsg = stringOrNull(run.Error.Message)
		}
		obj, dg := types.ObjectValue(deploymentRunObjectAttrTypes(), map[string]attr.Value{
			"id":            types.StringValue(run.ID),
			"deployment_id": types.StringValue(run.DeploymentID),
			"agent_id":      types.StringValue(run.Agent.ID),
			"agent_version": types.Int64Value(int64(run.Agent.Version)),
			"session_id":    sessionID,
			"trigger_type":  types.StringValue(run.TriggerContext.Type),
			"scheduled_at":  scheduledAt,
			"error_type":    errType,
			"error_message": errMsg,
			"created_at":    types.StringValue(run.CreatedAt.Format(timeFormatRFC3339)),
		})
		resp.Diagnostics.Append(dg...)
		items = append(items, obj)
	}
	runsList, dg := types.ListValue(objType, items)
	resp.Diagnostics.Append(dg...)
	cfg.Runs = runsList

	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
