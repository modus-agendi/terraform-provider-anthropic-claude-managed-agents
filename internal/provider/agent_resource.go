package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

var (
	_ resource.Resource                = (*agentResource)(nil)
	_ resource.ResourceWithConfigure   = (*agentResource)(nil)
	_ resource.ResourceWithImportState = (*agentResource)(nil)
)

type agentResource struct {
	client *client.Client
}

func newAgentResource() resource.Resource {
	return &agentResource{}
}

func (r *agentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *agentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: agentResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `agent_01ABC...`). Use this value with `terraform import`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable agent name. Mutable.",
				Required:            true,
			},
			"model": schema.StringAttribute{
				MarkdownDescription: "Model identifier (e.g. `claude-opus-4-7`). Mutable. The API also accepts an object form with `speed`; this provider exposes only the bare string in v0.1.",
				Required:            true,
			},
			"system": schema.StringAttribute{
				MarkdownDescription: "System prompt for the agent. Optional. Set to `null` to clear.",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description. Optional. Set to `null` to clear.",
				Optional:            true,
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Arbitrary string-string labels. Merged at the key level on update: removing a key from your HCL causes the provider to send an empty-string value for that key, which the API treats as a delete.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "Server-managed monotonic version. Used internally for optimistic concurrency on update.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the agent was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp of the most recent change.",
				Computed:            true,
			},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the agent was archived, or `null` if active. `terraform destroy` issues an archive call against this resource.",
				Computed:            true,
			},
			"mcp_servers": schema.ListNestedAttribute{
				MarkdownDescription: "MCP servers the agent may connect to at session runtime. Mutable. Sending an empty list clears server-side state.",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{Required: true, MarkdownDescription: "Currently only `url`."},
						"name": schema.StringAttribute{Required: true, MarkdownDescription: "Logical name. Referenced by `tools[mcp_toolset].mcp_server_name`."},
						"url":  schema.StringAttribute{Required: true, MarkdownDescription: "Server URL."},
					},
				},
			},
			"skills": schema.ListNestedAttribute{
				MarkdownDescription: "Skills the agent has access to. Mutable.",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":     schema.StringAttribute{Required: true, MarkdownDescription: "`anthropic` for pre-built skills or `custom` for user-uploaded skills."},
						"skill_id": schema.StringAttribute{Required: true, MarkdownDescription: "For `anthropic`: short name (e.g. `xlsx`). For `custom`: `skill_*` id."},
						"version":  schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Version selector for `custom` skills. Defaults server-side to `latest`."},
					},
				},
			},
			"multiagent": schema.SingleNestedAttribute{
				MarkdownDescription: "Multi-agent coordinator config. Mutable. Set to null to clear.",
				Optional:            true,
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{Required: true, MarkdownDescription: "Currently only `coordinator`."},
					"agents": schema.ListNestedAttribute{
						MarkdownDescription: "Members of the coordinator's roster.",
						Required:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{Required: true, MarkdownDescription: "`agent` to reference another agent, or `self` for self-delegation."},
								"id":   schema.StringAttribute{Optional: true, MarkdownDescription: "Agent id (`agent_*`). Required when `type = \"agent\"`; must be omitted when `type = \"self\"`."},
							},
						},
					},
				},
			},
		},
	}
}

func (r *agentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *agentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := client.AgentCreateRequest{
		Name:  plan.Name.ValueString(),
		Model: plan.Model.ValueString(),
	}
	if !plan.System.IsNull() && !plan.System.IsUnknown() {
		v := plan.System.ValueString()
		apiReq.System = &v
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		apiReq.Description = &v
	}
	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		m, diags := mapToStringMap(ctx, plan.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Metadata = m
	}
	mcps, d := mcpServersListToAPI(ctx, plan.McpServers)
	resp.Diagnostics.Append(d...)
	apiReq.McpServers = mcps
	skills, d := skillsListToAPI(ctx, plan.Skills)
	resp.Diagnostics.Append(d...)
	apiReq.Skills = skills
	multi, d := multiagentToAPI(ctx, plan.Multiagent)
	resp.Diagnostics.Append(d...)
	apiReq.Multiagent = multi
	if resp.Diagnostics.HasError() {
		return
	}

	agent, err := r.client.CreateAgent(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create agent", err.Error())
		return
	}

	state := agentFromAPI(ctx, agent, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *agentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agent, err := r.client.GetAgent(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read agent", err.Error())
		return
	}

	fresh := agentFromAPI(ctx, agent, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *agentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state agentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.AgentUpdateRequest{Version: int(state.Version.ValueInt64())}

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Model.Equal(state.Model) {
		v := plan.Model.ValueString()
		updateReq.Model = &v
	}
	if !plan.System.Equal(state.System) {
		updateReq.System = nullableStringPayload(plan.System)
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = nullableStringPayload(plan.Description)
	}
	if !plan.Metadata.Equal(state.Metadata) {
		merged, diags := metadataMerge(ctx, plan.Metadata, state.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Metadata = merged
	}
	if !plan.McpServers.Equal(state.McpServers) {
		mcps, d := mcpServersListToAPI(ctx, plan.McpServers)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		// Send an explicit (possibly empty) list — nil means "leave
		// unchanged"; an empty list means "clear".
		if mcps == nil {
			mcps = []client.McpServer{}
		}
		updateReq.McpServers = &mcps
	}
	if !plan.Skills.Equal(state.Skills) {
		skills, d := skillsListToAPI(ctx, plan.Skills)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		if skills == nil {
			skills = []client.Skill{}
		}
		updateReq.Skills = &skills
	}
	if !plan.Multiagent.Equal(state.Multiagent) {
		multi, d := multiagentToAPI(ctx, plan.Multiagent)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Multiagent = multi
	}

	agent, err := r.client.UpdateAgent(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update agent", err.Error())
		return
	}

	fresh := agentFromAPI(ctx, agent, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *agentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.ArchiveAgent(ctx, state.ID.ValueString()); err != nil {
		// Treat a 404 during destroy as success: the resource is already gone.
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to archive agent", err.Error())
		return
	}
}

func (r *agentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// nullableStringPayload returns a JSON literal suitable for the
// AgentUpdateRequest's nullable raw-message fields. A non-null string becomes
// a JSON string; a null value becomes the literal `null` so the upstream API
// clears the field.
func nullableStringPayload(s types.String) json.RawMessage {
	if s.IsNull() || s.IsUnknown() {
		return json.RawMessage([]byte(`null`))
	}
	b, err := json.Marshal(s.ValueString())
	if err != nil {
		return nil
	}
	return b
}

const agentResourceMarkdown = "Manages a Claude Managed Agents agent.\n\n" +
	"### Lifecycle\n\n" +
	"`terraform destroy` issues `POST /v1/agents/{id}/archive`. The upstream API does not expose a DELETE endpoint for agents; archive is the terminal lifecycle operation and is one-way (archived agents cannot be unarchived).\n\n" +
	"### Updates\n\n" +
	"Updates use server-side optimistic concurrency via the `version` field, which the provider manages automatically. If you see a version conflict in a plan, run `terraform apply -refresh-only` to pull the current server version into state.\n\n" +
	"### Metadata\n\n" +
	"The `metadata` map is key-level merged: removing a key from your HCL causes the provider to send the empty string for that key on update, which the API treats as a delete.\n\n" +
	"### Server-side nested fields\n\n" +
	"The upstream agent object also includes `tools`, `mcp_servers`, `skills`, and `multiagent` nested fields. v0.1 of this provider preserves whatever is on the server for those fields, but does not expose them as HCL. To change them today, use the API directly; Terraform updates to other fields will not clobber them."
