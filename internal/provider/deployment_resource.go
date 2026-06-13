package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ resource.Resource                = (*deploymentResource)(nil)
	_ resource.ResourceWithConfigure   = (*deploymentResource)(nil)
	_ resource.ResourceWithImportState = (*deploymentResource)(nil)
)

type deploymentResource struct {
	client *client.Client
}

func newDeploymentResource() resource.Resource {
	return &deploymentResource{}
}

func (r *deploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (r *deploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: deploymentResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `depl_01ABC...`). Use with `terraform import`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable deployment name. Mutable.",
				Required:            true,
			},
			"agent": schema.StringAttribute{
				MarkdownDescription: "Agent id (e.g. `agent_01ABC...`) to deploy. The deployment pins the agent's latest version at create/update time; the resolved version is exposed as `agent_version`. Mutable.",
				Required:            true,
			},
			"agent_version": schema.Int64Attribute{
				MarkdownDescription: "The concrete agent version the API resolved `agent` to. Computed.",
				Computed:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Container environment id the agent runs in. Mutable.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description. Optional. Set to `null` to clear.",
				Optional:            true,
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Arbitrary string-string labels (max 16 keys). Merge semantics: removing a key from HCL deletes it server-side. Omit the attribute to leave it unset; an explicit empty map (`{}`) is rejected.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Map{nonEmptyMap()},
			},
			"vault_ids": schema.ListAttribute{
				MarkdownDescription: "Vault ids whose credentials are mounted into each session (max 50). Mutable. Omit the attribute to leave it unset; an explicit empty list (`[]`) is rejected.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.List{nonEmptyList()},
			},
			"desired_status": schema.StringAttribute{
				MarkdownDescription: "Intended run state: `active` or `paused`. Defaults to `active`. Changing it pauses or resumes the deployment via the pause/resume endpoints. This is your INTENT — it is independent of the observed `status`, so an automatic error-pause (which moves `status` to `paused` while `desired_status` stays `active`) does NOT cause Terraform to fight the API. To resume after an error-pause, run `terraform apply` once the underlying cause is fixed.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("active"),
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Observed run state reported by the API: `active` or `paused`. Read-only. Compare against `desired_status` and `paused_reason` to detect error-pauses.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 archive timestamp, or `null` if active. `terraform destroy` archives the deployment.",
				Computed:            true,
			},
			"initial_events": schema.ListNestedAttribute{
				MarkdownDescription: deploymentInitialEventsMarkdown,
				Required:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "`user.message`, `system.message`, or `user.define_outcome`.",
						},
						"content": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `user.message`/`system.message`: a JSON-encoded array of content blocks. Use `jsonencode([{ type = \"text\", text = \"...\" }])`. Kept as a JSON string (not nested HCL) because content blocks are a deep union (text/image/document with nested sources); this round-trips any block the API accepts.",
						},
						"description": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `user.define_outcome`: the task specification.",
						},
						"rubric": schema.SingleNestedAttribute{
							Optional:            true,
							MarkdownDescription: "For `user.define_outcome`: success rubric, either an uploaded file or inline text.",
							Attributes: map[string]schema.Attribute{
								"type":    schema.StringAttribute{Required: true, MarkdownDescription: "`file` or `text`."},
								"file_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Required when `type = \"file\"`."},
								"content": schema.StringAttribute{Optional: true, MarkdownDescription: "Required when `type = \"text\"` (max 262144 chars)."},
							},
						},
						"max_iterations": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "For `user.define_outcome`: max refine iterations (default 3, max 20).",
						},
					},
				},
			},
			"resources": schema.ListNestedAttribute{
				MarkdownDescription: deploymentResourcesMarkdown,
				Optional:            true,
				Validators:          []validator.List{nonEmptyList()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "`github_repository`, `file`, or `memory_store`.",
						},
						"url": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `github_repository`: the repository URL.",
						},
						"authorization_token": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							WriteOnly:           true,
							MarkdownDescription: "For `github_repository`: access token for cloning. Write-only — sent to the API but never persisted to state or returned on read. Pair with `authorization_token_wo_version` to trigger re-send on rotation.",
						},
						"authorization_token_wo_version": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "Increment to signal that the write-only `authorization_token` was rotated and must be re-sent. Stored in state (the token itself is not), so changing it produces a diff that re-sends the whole `resources` list from config.",
						},
						"checkout": schema.SingleNestedAttribute{
							Optional:            true,
							MarkdownDescription: "For `github_repository`: pin to a branch or commit. Defaults to the repo's default branch.",
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{Required: true, MarkdownDescription: "`branch` or `commit`."},
								"name": schema.StringAttribute{Optional: true, MarkdownDescription: "Branch name (when `type = \"branch\"`)."},
								"sha":  schema.StringAttribute{Optional: true, MarkdownDescription: "Full commit SHA (when `type = \"commit\"`)."},
							},
						},
						"mount_path": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `github_repository`/`file`: where to mount. Defaults are server-assigned.",
						},
						"file_id": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `file`: the uploaded file id.",
						},
						"memory_store_id": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `memory_store`: the memory store id.",
						},
						"access": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `memory_store`: `read_write` or `read_only`.",
						},
						"instructions": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "For `memory_store`: usage instructions for the agent (max 4096 chars).",
						},
					},
				},
			},
			"schedule": schema.SingleNestedAttribute{
				MarkdownDescription: "Optional cron schedule. Omit for a manually-triggered deployment. The API's read-only next-run enrichment is not surfaced in state (to keep plans clean); query it via the API/console.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"type":       schema.StringAttribute{Required: true, MarkdownDescription: "Currently only `cron`."},
					"expression": schema.StringAttribute{Required: true, MarkdownDescription: "5-field POSIX cron (e.g. `0 3 * * *`). No `@daily`-style shortcuts.", Validators: []validator.String{fiveFieldCron()}},
					"timezone":   schema.StringAttribute{Required: true, MarkdownDescription: "IANA timezone (e.g. `UTC`, `America/Los_Angeles`)."},
				},
			},
			"paused_reason": schema.SingleNestedAttribute{
				MarkdownDescription: "Why the deployment is paused, or `null` when active. `type` is `manual` (paused via `desired_status`) or `error` (auto-paused), with the typed error in `error`.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{Computed: true, MarkdownDescription: "`manual` or `error`."},
					"error": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Typed error that caused an auto-pause (present only when `type = \"error\"`).",
						Attributes: map[string]schema.Attribute{
							"type":    schema.StringAttribute{Computed: true, MarkdownDescription: "Error type (e.g. `vault_not_found_error`)."},
							"message": schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable error detail."},
						},
					},
				},
			},
		},
	}
}

func (r *deploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected ProviderData type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *deploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// agent is sent as a bare JSON string (pins latest version).
	agentJSON, err := json.Marshal(plan.Agent.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to encode agent", err.Error())
		return
	}

	apiReq := client.DeploymentCreateRequest{
		Name:          plan.Name.ValueString(),
		Agent:         agentJSON,
		EnvironmentID: plan.EnvironmentID.ValueString(),
		InitialEvents: initialEventsListToAPI(ctx, plan.InitialEvents, &resp.Diagnostics),
		Schedule:      scheduleToAPI(ctx, plan.Schedule, &resp.Diagnostics),
		VaultIDs:      listToStringSlice(ctx, plan.VaultIDs, &resp.Diagnostics),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		apiReq.Description = &v
	}
	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		md, d := mapToStringMap(ctx, plan.Metadata)
		resp.Diagnostics.Append(d...)
		apiReq.Metadata = md
	}
	// resources must be read from config so write-only tokens are present.
	apiReq.Resources = r.resourcesFromConfig(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	dep, err := r.client.CreateDeployment(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create deployment", err.Error())
		return
	}

	// A deployment is created active. If the user wants it paused, pause now.
	if plan.DesiredStatus.ValueString() == "paused" {
		dep, err = r.client.PauseDeployment(ctx, dep.ID)
		if err != nil {
			resp.Diagnostics.AddError("Deployment created but pause failed", err.Error())
			return
		}
	}

	state := deploymentFromAPI(ctx, dep, plan.Resources, plan.DesiredStatus, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *deploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dep, err := r.client.GetDeployment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read deployment", err.Error())
		return
	}

	fresh := deploymentFromAPI(ctx, dep, state.Resources, state.DesiredStatus, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *deploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state deploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var updateReq client.DeploymentUpdateRequest
	changed := false

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
		changed = true
	}
	if !plan.Agent.Equal(state.Agent) {
		agentJSON, err := json.Marshal(plan.Agent.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to encode agent", err.Error())
			return
		}
		updateReq.Agent = agentJSON
		changed = true
	}
	if !plan.EnvironmentID.Equal(state.EnvironmentID) {
		v := plan.EnvironmentID.ValueString()
		updateReq.EnvironmentID = &v
		changed = true
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = nullableStringPayload(plan.Description)
		changed = true
	}
	if !plan.Metadata.Equal(state.Metadata) {
		md, d := metadataMergePtr(ctx, plan.Metadata, state.Metadata)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Metadata = md
		changed = true
	}
	if !plan.VaultIDs.Equal(state.VaultIDs) {
		vs := listToStringSlice(ctx, plan.VaultIDs, &resp.Diagnostics)
		if vs == nil {
			vs = []string{}
		}
		updateReq.VaultIDs = &vs
		changed = true
	}
	if !plan.Schedule.Equal(state.Schedule) {
		updateReq.Schedule = r.schedulePayload(ctx, plan.Schedule, &resp.Diagnostics)
		changed = true
	}
	if !plan.Resources.Equal(state.Resources) {
		res := r.resourcesFromConfig(ctx, req.Config, &resp.Diagnostics)
		if res == nil {
			res = []client.DeploymentResource{}
		}
		updateReq.Resources = &res
		changed = true
	}
	if resp.Diagnostics.HasError() {
		return
	}

	dep := &client.Deployment{}
	if changed {
		var err error
		dep, err = r.client.UpdateDeployment(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update deployment", err.Error())
			return
		}
	}

	// Reconcile pause/resume from desired_status intent.
	if !plan.DesiredStatus.Equal(state.DesiredStatus) {
		var err error
		switch plan.DesiredStatus.ValueString() {
		case "paused":
			dep, err = r.client.PauseDeployment(ctx, state.ID.ValueString())
		case "active":
			dep, err = r.client.ResumeDeployment(ctx, state.ID.ValueString())
		default:
			resp.Diagnostics.AddError("Invalid desired_status", fmt.Sprintf("must be \"active\" or \"paused\", got %q", plan.DesiredStatus.ValueString()))
			return
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to reconcile desired_status", err.Error())
			return
		}
	} else if !changed {
		// Nothing changed that hit the API; refresh to get a coherent object.
		var err error
		dep, err = r.client.GetDeployment(ctx, state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to read deployment", err.Error())
			return
		}
	}

	fresh := deploymentFromAPI(ctx, dep, plan.Resources, plan.DesiredStatus, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *deploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.ArchiveDeployment(ctx, state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to archive deployment", err.Error())
		return
	}
}

func (r *deploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// resourcesFromConfig reads the `resources` list from config (NOT plan/state)
// so write-only authorization_token values are included, then flattens it into
// client structs.
func (r *deploymentResource) resourcesFromConfig(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) []client.DeploymentResource {
	var resourcesList types.List
	diags.Append(cfg.GetAttribute(ctx, path.Root("resources"), &resourcesList)...)
	if diags.HasError() {
		return nil
	}
	return deploymentResourcesListToAPI(ctx, resourcesList, diags)
}

// schedulePayload builds the PATCH `schedule` field: the literal JSON null to
// clear the schedule, or the marshaled schedule object to set it.
func (r *deploymentResource) schedulePayload(ctx context.Context, obj types.Object, diags *diag.Diagnostics) json.RawMessage {
	s := scheduleToAPI(ctx, obj, diags)
	if s == nil {
		return json.RawMessage("null")
	}
	b, err := json.Marshal(s)
	if err != nil {
		diags.AddError("Failed to encode schedule", err.Error())
		return nil
	}
	return b
}

const deploymentInitialEventsMarkdown = "Events sent to each session when the deployment fires (1-50 entries). " +
	"Changing this list forces replacement of the deployment (the API does not " +
	"patch initial events in place). Three variants discriminated by `type`:\n\n" +
	"  - `user.message` — set `content` to a `jsonencode`d array of content blocks.\n" +
	"  - `system.message` — privileged context; set `content` likewise. Must be the " +
	"**last** event in the list and follow a `user.message`; supported only on " +
	"models that accept system messages.\n" +
	"  - `user.define_outcome` — set `description` (task), optional `rubric`, optional `max_iterations`."

const deploymentResourcesMarkdown = "Resources mounted into each session (max 500). Three variants discriminated " +
	"by `type`: `github_repository` (with a write-only `authorization_token`), " +
	"`file`, and `memory_store`. The whole list is replaced on any change."

const deploymentResourceMarkdown = "Manages a Claude Managed Agents deployment: a configured, persistent " +
	"instance of an agent bound to an environment, vaults, resources, initial " +
	"events, and an optional cron schedule.\n\n" +
	"### Lifecycle\n\n" +
	"`terraform destroy` issues `POST /v1/deployments/{id}/archive` (one-way). " +
	"There is no DELETE endpoint.\n\n" +
	"### Pause / resume\n\n" +
	"Set `desired_status` to `active` or `paused`; the provider calls the " +
	"pause/resume endpoints to reconcile. `desired_status` is your intent and is " +
	"deliberately decoupled from the observed `status`: if the API auto-pauses " +
	"the deployment on an error, `status` becomes `paused` while `desired_status` " +
	"stays `active`, and Terraform will NOT fight it. Inspect `paused_reason` to " +
	"see why, fix the cause, then `terraform apply` to resume.\n\n" +
	"### Updates\n\n" +
	"The deployments API has no optimistic-concurrency version; updates are " +
	"last-write-wins `PATCH`. Changing `initial_events` forces replacement."
