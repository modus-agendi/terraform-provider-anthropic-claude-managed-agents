package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ resource.Resource                = (*skillResource)(nil)
	_ resource.ResourceWithConfigure   = (*skillResource)(nil)
	_ resource.ResourceWithImportState = (*skillResource)(nil)
)

type skillResource struct {
	client *client.Client
}

func newSkillResource() resource.Resource {
	return &skillResource{}
}

func (r *skillResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (r *skillResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: skillResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned identifier (e.g. `skill_01ABC...`). Use this value with `terraform import`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_title": schema.StringAttribute{
				MarkdownDescription: "Human-readable skill title. Limited to 64 characters by the upstream API. Immutable — the Skills API has no PATCH endpoint for display title, so changing it forces destroy + recreate (which re-uploads every file and rotates the `id`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"source_dir": schema.StringAttribute{
				MarkdownDescription: "Local directory containing `SKILL.md` and any sibling files. Walked at every plan to compute `content_hash`; a byte-level change to any file (or any rename / add / remove) produces a new hash, which uploads a new immutable version on the next apply.",
				Required:            true,
			},
			"version_rotation": schema.Int64Attribute{
				MarkdownDescription: "Manual rotation counter. Incrementing this integer forces a new version even if every file in `source_dir` is unchanged. Useful to invalidate cached state on a downstream agent without editing skill content.",
				Optional:            true,
			},
			"content_hash": schema.StringAttribute{
				MarkdownDescription: "Sha256 over the sorted `source_dir` contents combined with `version_rotation`. Recomputed at every plan; drift is detected by comparing this to the value last applied. Hex-encoded, no `sha256:` prefix.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					newContentHashPlanModifier(),
				},
			},
			"latest_version": schema.StringAttribute{
				MarkdownDescription: "Server-side version string of the most recent uploaded version. Epoch-timestamp form (e.g. `1747000000`). Updates every time the provider uploads a new version.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{newLatestVersionPlanModifier()},
			},
			"source": schema.StringAttribute{
				MarkdownDescription: "Either `custom` (resource-managed) or `anthropic` (prebuilt). Always `custom` for this resource — the resource refuses to manage prebuilt skills.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the skill was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *skillResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *skillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan skillModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dir := resolveSkillSourceDir(plan.SourceDir.ValueString())
	files, err := walkSkillDir(dir)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Failed to walk source_dir", err.Error())
		return
	}
	if err := validateSkillFilesProvider(files); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Invalid skill source_dir", err.Error())
		return
	}

	created, err := r.client.CreateSkill(ctx, client.SkillCreateRequest{
		DisplayTitle: plan.DisplayTitle.ValueString(),
		Files:        files,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create skill", err.Error())
		return
	}

	state := plan
	state.ID = types.StringValue(created.ID)
	state.LatestVersion = types.StringValue(created.LatestVersion)
	state.Source = types.StringValue(created.Source)
	state.CreatedAt = types.StringValue(created.CreatedAt.Format(timeFormatRFC3339))
	// Recompute content_hash from the planned files so the post-apply state
	// matches what the plan modifier would compute on a noop re-plan.
	rotVal := int64(0)
	if !plan.VersionRotation.IsNull() && !plan.VersionRotation.IsUnknown() {
		rotVal = plan.VersionRotation.ValueInt64()
	}
	state.ContentHash = types.StringValue(canonicalSkillHash(files, rotVal))

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *skillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state skillModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s, err := r.client.GetSkill(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read skill", err.Error())
		return
	}
	if s.Source == "anthropic" {
		resp.Diagnostics.AddError(
			"Skill is an Anthropic prebuilt",
			fmt.Sprintf("Skill %q has source=anthropic; prebuilt skills cannot be managed via this resource. Use the claude-managed-agents_skill data source instead.", s.ID),
		)
		return
	}

	// Refresh server-side computed fields. content_hash is not derived from
	// the API (the API doesn't return file contents) — leave it as state's
	// value so the plan modifier will recompute it fresh on the next plan.
	state.LatestVersion = types.StringValue(s.LatestVersion)
	state.Source = types.StringValue(s.Source)
	state.CreatedAt = types.StringValue(s.CreatedAt.Format(timeFormatRFC3339))
	// display_title is server-authoritative — refresh in case of out-of-band
	// rename (which would only happen via a future API; current API forbids).
	state.DisplayTitle = types.StringValue(s.DisplayTitle)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *skillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state skillModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// display_title is RequiresReplace; the only mutation we handle here is
	// "content_hash changed" → upload a new version.
	if plan.ContentHash.Equal(state.ContentHash) {
		// Nothing to do server-side. Persist plan (covers version_rotation
		// state-only changes that didn't alter the hash, which shouldn't
		// happen because rotation is folded into the hash — but defensive).
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
		return
	}

	dir := resolveSkillSourceDir(plan.SourceDir.ValueString())
	files, err := walkSkillDir(dir)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Failed to walk source_dir", err.Error())
		return
	}
	if err := validateSkillFilesProvider(files); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Invalid skill source_dir", err.Error())
		return
	}

	ver, err := r.client.CreateSkillVersion(ctx, state.ID.ValueString(), client.SkillVersionCreateRequest{Files: files})
	if err != nil {
		resp.Diagnostics.AddError("Failed to upload new skill version", err.Error())
		return
	}

	fresh := plan
	fresh.ID = state.ID
	fresh.Source = state.Source
	fresh.CreatedAt = state.CreatedAt
	fresh.LatestVersion = types.StringValue(ver.Version)
	rotVal := int64(0)
	if !plan.VersionRotation.IsNull() && !plan.VersionRotation.IsUnknown() {
		rotVal = plan.VersionRotation.ValueInt64()
	}
	fresh.ContentHash = types.StringValue(canonicalSkillHash(files, rotVal))

	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

// Delete cascades through every version then deletes the skill itself.
// Tolerates 404 at every step so already-gone resources don't fail destroy.
func (r *skillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state skillModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	versions, err := r.client.ListSkillVersions(ctx, id)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to list skill versions for cascade delete", err.Error())
		return
	}
	if versions != nil {
		for _, v := range versions.Data {
			if err := r.client.DeleteSkillVersion(ctx, id, v.Version); err != nil && !client.IsNotFound(err) {
				resp.Diagnostics.AddError(
					"Failed to delete skill version",
					fmt.Sprintf("skill_id=%s version=%s: %s", id, v.Version, err.Error()),
				)
				return
			}
		}
	}

	if err := r.client.DeleteSkill(ctx, id); err != nil && !client.IsNotFound(err) {
		// Surface client-side validation errors with their original
		// wording — tests rely on the wording in IsConflict/IsNotFound
		// detection elsewhere.
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			resp.Diagnostics.AddError("Failed to delete skill", apiErr.Error())
			return
		}
		resp.Diagnostics.AddError("Failed to delete skill", err.Error())
	}
}

// ImportState accepts the bare skill_id. The caller must re-supply
// `source_dir` and `display_title` in HCL after import; `content_hash`
// recomputes from the local files on the next plan.
func (r *skillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// skillModel mirrors the resource schema.
type skillModel struct {
	ID              types.String `tfsdk:"id"`
	DisplayTitle    types.String `tfsdk:"display_title"`
	SourceDir       types.String `tfsdk:"source_dir"`
	VersionRotation types.Int64  `tfsdk:"version_rotation"`
	ContentHash     types.String `tfsdk:"content_hash"`
	LatestVersion   types.String `tfsdk:"latest_version"`
	Source          types.String `tfsdk:"source"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

const skillResourceMarkdown = "Manages a Claude Managed Agents custom skill.\n\n" +
	"Skills are immutable, versioned bundles of files (a `SKILL.md` plus supporting assets) that an agent can call as a packaged capability. This resource owns the skill end-to-end: uploading content, tracking the latest version, and tearing down every version on destroy.\n\n" +
	"### Lifecycle on destroy\n\n" +
	"`terraform destroy` issues `DELETE /v1/skills/{id}/versions/{v}` for every version returned by `GET /v1/skills/{id}/versions`, then `DELETE /v1/skills/{id}`. The API rejects deleting a skill while versions remain, so the cascade is required. Every step tolerates 404 so already-gone resources do not fail destroy. There is no archive concept for skills — destroy is permanent.\n\n" +
	"### Versions\n\n" +
	"The provider walks `source_dir` at every plan and computes a sha256 over the sorted contents combined with `version_rotation`. When the resulting `content_hash` differs from the stored value, `terraform apply` calls `POST /v1/skills/{id}/versions` to upload a new immutable version and refreshes `latest_version`. Old versions remain visible via `GET /v1/skills/{id}/versions` until destroy.\n\n" +
	"### Drift detection\n\n" +
	"The Skills API does not return file contents on read, so drift is detected exclusively by comparing the locally-walked hash to the value last applied. Out-of-band version uploads (via `curl`) are visible on next refresh as a new `latest_version` but do not cause the provider to download or replace local content. If you edit local files AND a sibling out-of-band version was uploaded, the next apply uploads your local version on top, making it the new `latest_version`.\n\n" +
	"### 30 MB upload cap\n\n" +
	"The total walked size of `source_dir` (sum of every file's bytes) must be ≤ 30 MB. The provider validates this at plan time so the error surfaces before any network call.\n\n" +
	"### Display title is immutable\n\n" +
	"The Skills API exposes only POST (create), POST (version), GET, and DELETE — there is no PATCH for `display_title`. Changing it in HCL plans destroy + recreate, which rotates the `id` and re-uploads every file. Avoid touching `display_title` after first apply unless you mean it.\n\n" +
	"### Prebuilt skills\n\n" +
	"Anthropic's prebuilt skills (`pptx`, `xlsx`, `docx`, `pdf`, etc.) cannot be managed by this resource — `Read` errors loudly if the server returns `source = \"anthropic\"`. Use the `claude-managed-agents_skill` data source to reference them instead.\n\n" +
	"### Import\n\n" +
	"`terraform import` accepts the bare `skill_*` id. After import, the next plan will require `source_dir` and `display_title` to be set in HCL; the provider cannot reconstruct them from the API alone. If your local `source_dir` matches the server-side content, `content_hash` will pin without re-uploading; otherwise the next apply uploads your local content as a new version."
