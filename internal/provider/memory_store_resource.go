package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ resource.Resource                = (*memoryStoreResource)(nil)
	_ resource.ResourceWithConfigure   = (*memoryStoreResource)(nil)
	_ resource.ResourceWithImportState = (*memoryStoreResource)(nil)
)

type memoryStoreResource struct {
	client *client.Client
}

func newMemoryStoreResource() resource.Resource {
	return &memoryStoreResource{}
}

func (r *memoryStoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory_store"
}

func (r *memoryStoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: memoryStoreResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `memstore_01ABC...`). Use this value with `terraform import`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable memory store name. Mutable.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description. Surfaced in the agent's system prompt when the store is attached to a session, so make it informative. Optional; set to `null` to clear.",
				Optional:            true,
			},
			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "When `true`, `terraform destroy` issues `DELETE /v1/memory_stores/{id}` which permanently removes every memory and every memory version inside the store. When `false` (the default), destroy archives the store instead, preserving the audit trail. Set to `true` only if you are sure you do not need the audit trail.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the memory store was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp of the most recent change.",
				Computed:            true,
			},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the memory store was archived, or `null` if active.",
				Computed:            true,
			},
		},
	}
}

func (r *memoryStoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *memoryStoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan memoryStoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := client.MemoryStoreCreateRequest{Name: plan.Name.ValueString()}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		apiReq.Description = &v
	}

	store, err := r.client.CreateMemoryStore(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create memory store", err.Error())
		return
	}

	state := memoryStoreFromAPI(store, plan.DeleteOnDestroy)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *memoryStoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state memoryStoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	store, err := r.client.GetMemoryStore(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read memory store", err.Error())
		return
	}

	fresh := memoryStoreFromAPI(store, state.DeleteOnDestroy)
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *memoryStoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state memoryStoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.MemoryStoreUpdateRequest{}
	apiCallNeeded := false

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
		apiCallNeeded = true
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = nullableStringPayload(plan.Description)
		apiCallNeeded = true
	}

	if apiCallNeeded {
		store, err := r.client.UpdateMemoryStore(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update memory store", err.Error())
			return
		}
		fresh := memoryStoreFromAPI(store, plan.DeleteOnDestroy)
		resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
		return
	}

	// Only delete_on_destroy changed — no API call needed, just persist the
	// new flag.
	state.DeleteOnDestroy = plan.DeleteOnDestroy
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete defaults to archive (audit-preserving) and uses DELETE only when the
// user opted in via delete_on_destroy = true.
func (r *memoryStoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state memoryStoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if state.DeleteOnDestroy.ValueBool() {
		if err := r.client.DeleteMemoryStore(ctx, id); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Failed to delete memory store", err.Error())
		}
		return
	}
	if err := r.client.ArchiveMemoryStore(ctx, id); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to archive memory store", err.Error())
	}
}

func (r *memoryStoreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// memoryStoreModel mirrors the resource schema.
type memoryStoreModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	DeleteOnDestroy types.Bool   `tfsdk:"delete_on_destroy"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	ArchivedAt      types.String `tfsdk:"archived_at"`
}

func memoryStoreFromAPI(s *client.MemoryStore, deleteOnDestroy types.Bool) memoryStoreModel {
	m := memoryStoreModel{
		ID:              types.StringValue(s.ID),
		Name:            types.StringValue(s.Name),
		DeleteOnDestroy: deleteOnDestroy,
		CreatedAt:       types.StringValue(s.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt:       types.StringValue(s.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if deleteOnDestroy.IsNull() || deleteOnDestroy.IsUnknown() {
		// Should not happen because of the Bool default, but be defensive
		// so import works without a prior plan.
		m.DeleteOnDestroy = types.BoolValue(false)
	}
	// Real API normalizes an unset description to "" instead of null.
	// Treat the two as equivalent so plans stay clean for users who do not
	// configure the attribute.
	if s.Description != nil && *s.Description != "" {
		m.Description = types.StringValue(*s.Description)
	} else {
		m.Description = types.StringNull()
	}
	if s.ArchivedAt != nil {
		m.ArchivedAt = types.StringValue(s.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		m.ArchivedAt = types.StringNull()
	}
	return m
}

const memoryStoreResourceMarkdown = "Manages a Claude Managed Agents memory store.\n\n" +
	"Memory stores are workspace-scoped collections of text documents that persist across sessions. The agent mounts the store as a directory inside its sandbox and a brief note describing the store is added to the system prompt — so `description` is user-facing both for humans and for the agent.\n\n" +
	"### Lifecycle on destroy\n\n" +
	"By default, `terraform destroy` archives the store (`POST /v1/memory_stores/{id}/archive`). Archive is reversible-ish: existing memories and versions remain queryable but the store no longer accepts new attachments. Set `delete_on_destroy = true` to hard-delete instead — every memory and every memory version is removed, with no recovery path.\n\n" +
	"### Updates\n\n" +
	"`name` and `description` are mutable; the upstream API merges fields you send. Provide `description = null` to clear the description."
