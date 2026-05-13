package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// contentHashPlanModifier is the heart of skill drift detection. At plan
// time it walks `source_dir`, hashes the result, folds in `version_rotation`,
// and writes the resulting digest into the plan for `content_hash`.
//
// The sequence (matching spec §3.4):
//
//  1. If `source_dir` is unknown (an interpolated reference whose value
//     isn't decided yet), bail out cleanly — leave `content_hash` as
//     "unknown" so the apply can recompute, and add no diagnostic.
//  2. Walk the directory. If walking fails, emit a Warning (not Error)
//     and leave `content_hash` as-is. Resource Create / Update will
//     fail loudly with the real reason later — surfacing the same
//     filesystem error twice (once at plan, once at apply) is noisy.
//  3. Compute the hash. Write it to the plan.
//
// We deliberately do NOT compare against state here — drift detection works
// because any change in walked bytes produces a different hash, so the plan
// shows the diff naturally. The Update path inspects the diff to decide
// whether to upload a new version.
type contentHashPlanModifier struct{}

func newContentHashPlanModifier() planmodifier.String {
	return &contentHashPlanModifier{}
}

func (m *contentHashPlanModifier) Description(_ context.Context) string {
	return "Hashes the contents of source_dir at plan time to detect drift; combines with version_rotation."
}

func (m *contentHashPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *contentHashPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// On destroy the plan has no config — leave content_hash alone.
	if req.Plan.Raw.IsNull() {
		return
	}

	var sourceDir types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("source_dir"), &sourceDir)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if sourceDir.IsUnknown() {
		// Interpolated path not yet resolved. Mark hash as unknown so
		// apply recomputes.
		resp.PlanValue = types.StringUnknown()
		return
	}
	if sourceDir.IsNull() || sourceDir.ValueString() == "" {
		// The schema marks source_dir as Required, so this path should
		// never trigger during a real plan. Leave the value alone if it
		// somehow does.
		return
	}

	var rotation types.Int64
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("version_rotation"), &rotation)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rotVal := int64(0)
	if !rotation.IsNull() && !rotation.IsUnknown() {
		rotVal = rotation.ValueInt64()
	}

	dir := resolveSkillSourceDir(sourceDir.ValueString())
	files, err := walkSkillDir(dir)
	if err != nil {
		// Don't fail the plan — Create / Update will surface the same
		// error with full context. Leaving `content_hash` known prevents
		// the framework from blowing up with "unknown after apply" for
		// what should be a static value.
		resp.Diagnostics.AddAttributeWarning(
			path.Root("source_dir"),
			"Could not walk source_dir at plan time",
			err.Error(),
		)
		return
	}
	if err := validateSkillFilesProvider(files); err != nil {
		// Same treatment: warn now, error on apply.
		resp.Diagnostics.AddAttributeWarning(
			path.Root("source_dir"),
			"source_dir contents are invalid",
			err.Error(),
		)
		return
	}

	resp.PlanValue = types.StringValue(canonicalSkillHash(files, rotVal))
}

// latestVersionPlanModifier marks `latest_version` as known-after-apply
// whenever `content_hash` in the plan differs from state. Otherwise it
// rolls forward from state (the standard UseStateForUnknown semantics).
//
// Without this, the framework keeps `latest_version` pinned to the prior
// state value and then complains "Provider produced inconsistent result"
// when the Update path actually rotates it. Symmetric with the
// content_hash modifier — both pieces work together to model "the
// content changed, so the upstream version will too."
type latestVersionPlanModifier struct{}

func newLatestVersionPlanModifier() planmodifier.String {
	return &latestVersionPlanModifier{}
}

func (m *latestVersionPlanModifier) Description(_ context.Context) string {
	return "Marks latest_version as known-after-apply when content_hash changes; otherwise rolls forward from state."
}

func (m *latestVersionPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *latestVersionPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	if req.State.Raw.IsNull() {
		// Create — framework's default (unknown after apply) is correct.
		return
	}

	// Recompute the hash directly from the plan inputs so we don't depend on
	// the order in which attribute modifiers run (the framework does not
	// guarantee `content_hash` modifier has already executed when this one
	// fires on `latest_version`).
	var sourceDir types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("source_dir"), &sourceDir)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if sourceDir.IsUnknown() || sourceDir.IsNull() || sourceDir.ValueString() == "" {
		return
	}

	var rotation types.Int64
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("version_rotation"), &rotation)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rotVal := int64(0)
	if !rotation.IsNull() && !rotation.IsUnknown() {
		rotVal = rotation.ValueInt64()
	}

	files, err := walkSkillDir(resolveSkillSourceDir(sourceDir.ValueString()))
	if err != nil {
		// Filesystem error at plan time — leave latest_version pinned to
		// state. Create / Update will surface the real error.
		resp.PlanValue = req.StateValue
		return
	}
	freshHash := canonicalSkillHash(files, rotVal)

	var stateHash types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("content_hash"), &stateHash)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if freshHash != stateHash.ValueString() {
		resp.PlanValue = types.StringUnknown()
		return
	}
	resp.PlanValue = req.StateValue
}
