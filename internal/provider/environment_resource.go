package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ resource.Resource                = (*environmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*environmentResource)(nil)
	_ resource.ResourceWithImportState = (*environmentResource)(nil)
)

type environmentResource struct {
	client *client.Client
}

func newEnvironmentResource() resource.Resource {
	return &environmentResource{}
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: environmentResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `env_01ABC...`). Use this value with `terraform import`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable environment name. Immutable — changing forces replacement.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config": schema.SingleNestedAttribute{
				MarkdownDescription: "Sandbox configuration. Immutable — changing any field forces replacement.",
				Required:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.RequiresReplace()},
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "Config discriminator. Currently only `cloud` is supported.",
						Required:            true,
					},
					"packages": schema.SingleNestedAttribute{
						MarkdownDescription: "Per-package-manager install lists. All fields optional.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"apt":   schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "APT packages to install."},
							"cargo": schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Cargo crates to install."},
							"gem":   schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "RubyGems to install."},
							"go":    schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Go modules to install."},
							"npm":   schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "npm packages to install."},
							"pip":   schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "pip packages to install."},
						},
					},
					"networking": schema.SingleNestedAttribute{
						MarkdownDescription: "Outbound networking policy. Discriminated on `type`.",
						Required:            true,
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								MarkdownDescription: "Either `unrestricted` (allow all outbound) or `limited` (restrict to `allowed_hosts`).",
								Required:            true,
							},
							"allowed_hosts": schema.ListAttribute{
								MarkdownDescription: "Bare hostnames the agent may reach (e.g. `api.example.com`). URL schemes are rejected by the upstream API. Only valid when `type = \"limited\"`.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"allow_mcp_servers": schema.BoolAttribute{
								MarkdownDescription: "When `type = \"limited\"`, whether the agent may call out to MCP servers. Must be unset (null) when `type = \"unrestricted\"`.",
								Optional:            true,
								Computed:            true,
							},
							"allow_package_managers": schema.BoolAttribute{
								MarkdownDescription: "When `type = \"limited\"`, whether the agent may run package-manager installs. Must be unset (null) when `type = \"unrestricted\"`.",
								Optional:            true,
								Computed:            true,
							},
						},
					},
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the environment was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp of the most recent change.",
				Computed:            true,
			},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the environment was archived, or `null` if active.",
				Computed:            true,
			},
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiCfg, diags := configToAPI(ctx, plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.CreateEnvironment(ctx, client.EnvironmentCreateRequest{
		Name:   plan.Name.ValueString(),
		Config: apiCfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create environment", err.Error())
		return
	}

	state, diags := environmentFromAPI(ctx, env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}

	fresh, diags := environmentFromAPI(ctx, env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

// Update is required by the framework but environments have no upstream
// update endpoint. Every mutable-looking attribute is marked RequiresReplace,
// so this method should never be called with a meaningful diff. If it ever
// is, surface an error rather than silently dropping the change.
func (r *environmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Environment is immutable",
		"Environments cannot be updated in place. Every attribute requires replacement. This is a provider bug if you see this — please file an issue.",
	)
}

// Delete tries DELETE /v1/environments/{id} first. On 409 (sessions still
// reference the environment), it falls back to POST /archive so the resource
// can leave Terraform state without leaking server-side artifacts. Both paths
// are documented in the resource markdown so users understand the behavior.
func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	err := r.client.DeleteEnvironment(ctx, id)
	if err == nil {
		return
	}
	if client.IsNotFound(err) {
		return
	}
	if client.IsConflict(err) {
		if archiveErr := r.client.ArchiveEnvironment(ctx, id); archiveErr != nil {
			if client.IsNotFound(archiveErr) {
				return
			}
			resp.Diagnostics.AddError("Failed to archive environment after delete conflict", archiveErr.Error())
			return
		}
		return
	}
	resp.Diagnostics.AddError("Failed to delete environment", err.Error())
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// environmentModel mirrors the resource schema as Go types.
type environmentModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Config     types.Object `tfsdk:"config"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
	ArchivedAt types.String `tfsdk:"archived_at"`
}

// configObjectAttrTypes is the attribute-type map for the `config` object.
// Used when constructing types.Object values from the API response.
func configObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":       types.StringType,
		"packages":   types.ObjectType{AttrTypes: packagesObjectAttrTypes()},
		"networking": types.ObjectType{AttrTypes: networkingObjectAttrTypes()},
	}
}

func packagesObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"apt":   types.ListType{ElemType: types.StringType},
		"cargo": types.ListType{ElemType: types.StringType},
		"gem":   types.ListType{ElemType: types.StringType},
		"go":    types.ListType{ElemType: types.StringType},
		"npm":   types.ListType{ElemType: types.StringType},
		"pip":   types.ListType{ElemType: types.StringType},
	}
}

func networkingObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":                   types.StringType,
		"allowed_hosts":          types.ListType{ElemType: types.StringType},
		"allow_mcp_servers":      types.BoolType,
		"allow_package_managers": types.BoolType,
	}
}

const environmentResourceMarkdown = "Manages a Claude Managed Agents sandbox environment.\n\n" +
	"### Immutability\n\n" +
	"The upstream API does not expose an update endpoint for environments. Every attribute is marked `RequiresReplace`: changing any field — including a single package list entry — causes Terraform to destroy and re-create the environment.\n\n" +
	"### Lifecycle on destroy\n\n" +
	"`terraform destroy` first issues `DELETE /v1/environments/{id}`. If the API returns 409 (typically because an active session references the environment), the provider falls back to `POST /v1/environments/{id}/archive`. Archived environments remain visible via the data source until the API server prunes them, but they no longer accept new sessions.\n\n" +
	"### Networking policy\n\n" +
	"`config.networking.type` is a discriminator:\n\n" +
	"  - `unrestricted` — the agent may reach any host. Leave `allowed_hosts`, `allow_mcp_servers`, and `allow_package_managers` unset.\n" +
	"  - `limited` — the agent is restricted to `allowed_hosts`. The two `allow_*` booleans gate MCP-server and package-manager access; both default to `false`."
