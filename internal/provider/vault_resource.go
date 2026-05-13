package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

var (
	_ resource.Resource                = (*vaultResource)(nil)
	_ resource.ResourceWithConfigure   = (*vaultResource)(nil)
	_ resource.ResourceWithImportState = (*vaultResource)(nil)
)

type vaultResource struct {
	client *client.Client
}

func newVaultResource() resource.Resource {
	return &vaultResource{}
}

func (r *vaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

func (r *vaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: vaultResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `vlt_01ABC...`). Use this value with `terraform import`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable vault name. Mutable.",
				Required:            true,
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Arbitrary string-string labels. Full-replace on update: removing a key from your HCL deletes it server-side.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "When `true`, `terraform destroy` issues `DELETE /v1/vaults/{id}` which permanently removes the vault and cascades through every credential. When `false` (the default), destroy archives the vault, preserving the audit trail while purging secrets and freeing the bound MCP server URLs.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"updated_at": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 last-modified timestamp."},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 archive timestamp, or `null` if active.",
				Computed:            true,
			},
		},
	}
}

func (r *vaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vaultModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := client.VaultCreateRequest{DisplayName: plan.DisplayName.ValueString()}
	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		m, diags := mapToStringMap(ctx, plan.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Metadata = m
	}

	v, err := r.client.CreateVault(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create vault", err.Error())
		return
	}

	state := vaultFromAPI(ctx, v, plan.DeleteOnDestroy, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *vaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := r.client.GetVault(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read vault", err.Error())
		return
	}

	fresh := vaultFromAPI(ctx, v, state.DeleteOnDestroy, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *vaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vaultModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.VaultUpdateRequest{}
	apiCallNeeded := false

	if !plan.DisplayName.Equal(state.DisplayName) {
		v := plan.DisplayName.ValueString()
		updateReq.DisplayName = &v
		apiCallNeeded = true
	}
	if !plan.Metadata.Equal(state.Metadata) {
		merged, diags := metadataMerge(ctx, plan.Metadata, state.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Metadata = merged
		apiCallNeeded = true
	}

	if apiCallNeeded {
		v, err := r.client.UpdateVault(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update vault", err.Error())
			return
		}
		fresh := vaultFromAPI(ctx, v, plan.DeleteOnDestroy, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
		return
	}

	// Only delete_on_destroy changed — persist without an API call.
	state.DeleteOnDestroy = plan.DeleteOnDestroy
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *vaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if state.DeleteOnDestroy.ValueBool() {
		if err := r.client.DeleteVault(ctx, id); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Failed to delete vault", err.Error())
		}
		return
	}
	if err := r.client.ArchiveVault(ctx, id); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to archive vault", err.Error())
	}
}

func (r *vaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type vaultModel struct {
	ID              types.String `tfsdk:"id"`
	DisplayName     types.String `tfsdk:"display_name"`
	Metadata        types.Map    `tfsdk:"metadata"`
	DeleteOnDestroy types.Bool   `tfsdk:"delete_on_destroy"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	ArchivedAt      types.String `tfsdk:"archived_at"`
}

func vaultFromAPI(ctx context.Context, v *client.Vault, deleteOnDestroy types.Bool, diags *diag.Diagnostics) vaultModel {
	m := vaultModel{
		ID:              types.StringValue(v.ID),
		DisplayName:     types.StringValue(v.DisplayName),
		DeleteOnDestroy: deleteOnDestroy,
		CreatedAt:       types.StringValue(v.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt:       types.StringValue(v.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if deleteOnDestroy.IsNull() || deleteOnDestroy.IsUnknown() {
		m.DeleteOnDestroy = types.BoolValue(false)
	}
	if v.ArchivedAt != nil {
		m.ArchivedAt = types.StringValue(v.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		m.ArchivedAt = types.StringNull()
	}
	mdMap, d := stringMapToMap(ctx, v.Metadata)
	diags.Append(d...)
	m.Metadata = mdMap
	return m
}

const vaultResourceMarkdown = "Manages a Claude Managed Agents vault — a workspace-scoped container of credentials, typically modeling one end-user.\n\n" +
	"### Lifecycle on destroy\n\n" +
	"By default, `terraform destroy` archives the vault (`POST /v1/vaults/{id}/archive`). Archive cascades through credentials: their secret payloads are purged but the records remain visible for audit. Set `delete_on_destroy = true` to hard-delete; that removes the vault and every credential without retention.\n\n" +
	"### Updates\n\n" +
	"`display_name` and `metadata` are mutable. The metadata map uses full-replace semantics: the provider sends the exact map declared in HCL on every update, and the upstream API replaces whatever was stored. Removing a key from your HCL deletes it server-side."
