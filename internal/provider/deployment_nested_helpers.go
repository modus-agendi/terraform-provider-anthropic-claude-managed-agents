package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

// --- attribute-type maps -----------------------------------------------------

// initialEventObjectAttrTypes is the union shape for one `initial_events`
// entry. Every variant's fields are present; only the relevant ones are set
// per entry (discriminated by `type`).
func initialEventObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":           types.StringType,
		"content":        types.StringType,
		"description":    types.StringType,
		"rubric":         types.ObjectType{AttrTypes: rubricObjectAttrTypes()},
		"max_iterations": types.Int64Type,
	}
}

// rubricObjectAttrTypes is the shape of a user.define_outcome `rubric`.
func rubricObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":    types.StringType,
		"file_id": types.StringType,
		"content": types.StringType,
	}
}

// deploymentResourceObjectAttrTypes is the union shape for one `resources`
// entry (github_repository | file | memory_store).
func deploymentResourceObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":                           types.StringType,
		"url":                            types.StringType,
		"authorization_token":            types.StringType,
		"authorization_token_wo_version": types.Int64Type,
		"checkout":                       types.ObjectType{AttrTypes: checkoutObjectAttrTypes()},
		"mount_path":                     types.StringType,
		"file_id":                        types.StringType,
		"memory_store_id":                types.StringType,
		"access":                         types.StringType,
		"instructions":                   types.StringType,
	}
}

// checkoutObjectAttrTypes is the shape of a github_repository `checkout`.
func checkoutObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"name": types.StringType,
		"sha":  types.StringType,
	}
}

// scheduleObjectAttrTypes is the shape of the optional `schedule` block. Only
// the writable fields are exposed; the API's read-only enrichment
// (last_run_at, upcoming_runs_at) is intentionally dropped to keep state in
// sync with config.
func scheduleObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":       types.StringType,
		"expression": types.StringType,
		"timezone":   types.StringType,
	}
}

// pausedReasonObjectAttrTypes is the computed `paused_reason` shape.
func pausedReasonObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"error": types.ObjectType{AttrTypes: pausedErrorObjectAttrTypes()},
	}
}

// pausedErrorObjectAttrTypes is the nested typed-error shape inside
// paused_reason (present only when paused_reason.type == "error").
func pausedErrorObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":    types.StringType,
		"message": types.StringType,
	}
}

// --- deploymentFromAPI -------------------------------------------------------

// deploymentFromAPI maps a client.Deployment into the Terraform schema model.
//
// priorResources carries the prior plan/state `resources` list so the
// write-only authorization_token (never returned by the API) stays null and
// the TF-only authorization_token_wo_version is preserved across reads.
// priorDesiredStatus carries the prior desired_status: the API exposes only
// the actual `status`, so on read we keep the user's intent unless the prior
// value is unknown (first read), where we seed it from the actual status.
func deploymentFromAPI(ctx context.Context, d *client.Deployment, priorResources types.List, priorDesiredStatus types.String, diags *diag.Diagnostics) deploymentModel {
	m := deploymentModel{
		ID:            types.StringValue(d.ID),
		Name:          types.StringValue(d.Name),
		Agent:         types.StringValue(d.Agent.ID),
		AgentVersion:  types.Int64Value(int64(d.Agent.Version)),
		EnvironmentID: types.StringValue(d.EnvironmentID),
		Status:        types.StringValue(d.Status),
		CreatedAt:     types.StringValue(d.CreatedAt.Format(timeFormatRFC3339)),
	}

	if d.Description != nil {
		m.Description = types.StringValue(*d.Description)
	} else {
		m.Description = types.StringNull()
	}
	if d.ArchivedAt != nil {
		m.ArchivedAt = types.StringValue(d.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		m.ArchivedAt = types.StringNull()
	}

	// desired_status mirrors prior intent; seed from actual status on first read.
	if priorDesiredStatus.IsNull() || priorDesiredStatus.IsUnknown() {
		m.DesiredStatus = types.StringValue(d.Status)
	} else {
		m.DesiredStatus = priorDesiredStatus
	}

	mdMap, dg := stringMapToMap(ctx, d.Metadata)
	diags.Append(dg...)
	m.Metadata = mdMap

	m.VaultIDs = stringSliceToList(d.VaultIDs, diags)

	events, dg := initialEventsListFromAPI(d.InitialEvents, diags)
	_ = dg
	m.InitialEvents = events

	resources, dg := deploymentResourcesListFromAPI(ctx, d.Resources, priorResources)
	diags.Append(dg...)
	m.Resources = resources

	m.Schedule = scheduleFromAPI(d.Schedule, diags)
	m.PausedReason = pausedReasonFromAPI(d.PausedReason, diags)

	return m
}

// --- initial_events ----------------------------------------------------------

func initialEventsListFromAPI(events []client.DeploymentInitialEvent, diags *diag.Diagnostics) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: initialEventObjectAttrTypes()}
	items := make([]attr.Value, 0, len(events))
	for _, e := range events {
		content := types.StringNull()
		if len(e.Content) > 0 {
			if s, err := canonicalJSONArray(e.Content); err == nil {
				content = types.StringValue(s)
			} else {
				diags.AddError("Failed to encode initial_events content", err.Error())
			}
		}
		desc := types.StringNull()
		if e.Description != "" {
			desc = types.StringValue(e.Description)
		}
		maxIter := types.Int64Null()
		if e.MaxIterations != nil {
			maxIter = types.Int64Value(int64(*e.MaxIterations))
		}
		rubric := rubricFromAPI(e.Rubric)
		obj, d := types.ObjectValue(initialEventObjectAttrTypes(), map[string]attr.Value{
			"type":           types.StringValue(e.Type),
			"content":        content,
			"description":    desc,
			"rubric":         rubric,
			"max_iterations": maxIter,
		})
		diags.Append(d...)
		items = append(items, obj)
	}
	return types.ListValue(objType, items)
}

func rubricFromAPI(r *client.DeploymentRubric) types.Object {
	if r == nil {
		return types.ObjectNull(rubricObjectAttrTypes())
	}
	fileID := types.StringNull()
	if r.FileID != "" {
		fileID = types.StringValue(r.FileID)
	}
	content := types.StringNull()
	if r.Content != "" {
		content = types.StringValue(r.Content)
	}
	obj, _ := types.ObjectValue(rubricObjectAttrTypes(), map[string]attr.Value{
		"type":    types.StringValue(r.Type),
		"file_id": fileID,
		"content": content,
	})
	return obj
}

func initialEventsListToAPI(ctx context.Context, l types.List, diags *diag.Diagnostics) []client.DeploymentInitialEvent {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	type rubricModel struct {
		Type    types.String `tfsdk:"type"`
		FileID  types.String `tfsdk:"file_id"`
		Content types.String `tfsdk:"content"`
	}
	type eventModel struct {
		Type          types.String `tfsdk:"type"`
		Content       types.String `tfsdk:"content"`
		Description   types.String `tfsdk:"description"`
		Rubric        types.Object `tfsdk:"rubric"`
		MaxIterations types.Int64  `tfsdk:"max_iterations"`
	}
	var entries []eventModel
	diags.Append(l.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil
	}
	out := make([]client.DeploymentInitialEvent, 0, len(entries))
	for _, e := range entries {
		ev := client.DeploymentInitialEvent{Type: e.Type.ValueString()}
		if !e.Content.IsNull() && !e.Content.IsUnknown() && e.Content.ValueString() != "" {
			var blocks []json.RawMessage
			if err := json.Unmarshal([]byte(e.Content.ValueString()), &blocks); err != nil {
				diags.AddError("Invalid initial_events content", "content must be a JSON array of content blocks (use jsonencode): "+err.Error())
				return nil
			}
			ev.Content = blocks
		}
		if !e.Description.IsNull() && !e.Description.IsUnknown() {
			ev.Description = e.Description.ValueString()
		}
		if !e.MaxIterations.IsNull() && !e.MaxIterations.IsUnknown() {
			v := int(e.MaxIterations.ValueInt64())
			ev.MaxIterations = &v
		}
		if !e.Rubric.IsNull() && !e.Rubric.IsUnknown() {
			var r rubricModel
			diags.Append(e.Rubric.As(ctx, &r, basicObjectAsOpts())...)
			ev.Rubric = &client.DeploymentRubric{
				Type:    r.Type.ValueString(),
				FileID:  r.FileID.ValueString(),
				Content: r.Content.ValueString(),
			}
		}
		out = append(out, ev)
	}
	return out
}

// --- resources ---------------------------------------------------------------

func deploymentResourcesListFromAPI(ctx context.Context, resources []client.DeploymentResource, prior types.List) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: deploymentResourceObjectAttrTypes()}
	var diags diag.Diagnostics

	// Empty → null (not an empty list) so an unset optional `resources`
	// attribute does not drift against the API's `[]` response.
	if len(resources) == 0 {
		return types.ListNull(objType), diags
	}

	// Preserve the TF-only authorization_token_wo_version from prior state,
	// aligned by index (the API preserves resource ordering).
	priorWoVersions := decodeResourceWoVersions(ctx, prior, &diags)

	items := make([]attr.Value, 0, len(resources))
	for i, r := range resources {
		woVersion := types.Int64Null()
		if i < len(priorWoVersions) {
			woVersion = priorWoVersions[i]
		}
		obj, d := types.ObjectValue(deploymentResourceObjectAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(r.Type),
			"url":  stringOrNull(r.URL),
			// Write-only: never persisted to state.
			"authorization_token":            types.StringNull(),
			"authorization_token_wo_version": woVersion,
			"checkout":                       checkoutFromAPI(r.Checkout),
			"mount_path":                     stringOrNull(r.MountPath),
			"file_id":                        stringOrNull(r.FileID),
			"memory_store_id":                stringOrNull(r.MemoryStoreID),
			"access":                         stringOrNull(r.Access),
			"instructions":                   stringOrNull(r.Instructions),
		})
		diags.Append(d...)
		items = append(items, obj)
	}
	list, d := types.ListValue(objType, items)
	diags.Append(d...)
	return list, diags
}

func decodeResourceWoVersions(ctx context.Context, prior types.List, diags *diag.Diagnostics) []types.Int64 {
	if prior.IsNull() || prior.IsUnknown() {
		return nil
	}
	// ElementsAs needs a struct covering every attribute of the object; we
	// only read the wo_version field out of it.
	type fullEntry struct {
		Type          types.String `tfsdk:"type"`
		URL           types.String `tfsdk:"url"`
		AuthToken     types.String `tfsdk:"authorization_token"`
		WoVersion     types.Int64  `tfsdk:"authorization_token_wo_version"`
		Checkout      types.Object `tfsdk:"checkout"`
		MountPath     types.String `tfsdk:"mount_path"`
		FileID        types.String `tfsdk:"file_id"`
		MemoryStoreID types.String `tfsdk:"memory_store_id"`
		Access        types.String `tfsdk:"access"`
		Instructions  types.String `tfsdk:"instructions"`
	}
	var entries []fullEntry
	diags.Append(prior.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil
	}
	out := make([]types.Int64, len(entries))
	for i, e := range entries {
		out[i] = e.WoVersion
	}
	return out
}

func checkoutFromAPI(c *client.DeploymentCheckout) types.Object {
	if c == nil {
		return types.ObjectNull(checkoutObjectAttrTypes())
	}
	obj, _ := types.ObjectValue(checkoutObjectAttrTypes(), map[string]attr.Value{
		"type": types.StringValue(c.Type),
		"name": stringOrNull(c.Name),
		"sha":  stringOrNull(c.SHA),
	})
	return obj
}

// deploymentResourcesListToAPI flattens the `resources` list into client
// structs. The list MUST come from config (req.Config), not plan/state, so
// the write-only authorization_token values are present.
func deploymentResourcesListToAPI(ctx context.Context, l types.List, diags *diag.Diagnostics) []client.DeploymentResource {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	type checkoutModel struct {
		Type types.String `tfsdk:"type"`
		Name types.String `tfsdk:"name"`
		SHA  types.String `tfsdk:"sha"`
	}
	type resourceModel struct {
		Type          types.String `tfsdk:"type"`
		URL           types.String `tfsdk:"url"`
		AuthToken     types.String `tfsdk:"authorization_token"`
		WoVersion     types.Int64  `tfsdk:"authorization_token_wo_version"`
		Checkout      types.Object `tfsdk:"checkout"`
		MountPath     types.String `tfsdk:"mount_path"`
		FileID        types.String `tfsdk:"file_id"`
		MemoryStoreID types.String `tfsdk:"memory_store_id"`
		Access        types.String `tfsdk:"access"`
		Instructions  types.String `tfsdk:"instructions"`
	}
	var entries []resourceModel
	diags.Append(l.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil
	}
	out := make([]client.DeploymentResource, 0, len(entries))
	for _, e := range entries {
		r := client.DeploymentResource{
			Type:               e.Type.ValueString(),
			URL:                e.URL.ValueString(),
			AuthorizationToken: e.AuthToken.ValueString(),
			MountPath:          e.MountPath.ValueString(),
			FileID:             e.FileID.ValueString(),
			MemoryStoreID:      e.MemoryStoreID.ValueString(),
			Access:             e.Access.ValueString(),
			Instructions:       e.Instructions.ValueString(),
		}
		if !e.Checkout.IsNull() && !e.Checkout.IsUnknown() {
			var c checkoutModel
			diags.Append(e.Checkout.As(ctx, &c, basicObjectAsOpts())...)
			r.Checkout = &client.DeploymentCheckout{
				Type: c.Type.ValueString(),
				Name: c.Name.ValueString(),
				SHA:  c.SHA.ValueString(),
			}
		}
		out = append(out, r)
	}
	return out
}

// --- schedule ----------------------------------------------------------------

func scheduleFromAPI(s *client.DeploymentSchedule, diags *diag.Diagnostics) types.Object {
	if s == nil {
		return types.ObjectNull(scheduleObjectAttrTypes())
	}
	obj, d := types.ObjectValue(scheduleObjectAttrTypes(), map[string]attr.Value{
		"type":       types.StringValue(s.Type),
		"expression": types.StringValue(s.Expression),
		"timezone":   types.StringValue(s.Timezone),
	})
	diags.Append(d...)
	return obj
}

func scheduleToAPI(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.DeploymentSchedule {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	var raw struct {
		Type       types.String `tfsdk:"type"`
		Expression types.String `tfsdk:"expression"`
		Timezone   types.String `tfsdk:"timezone"`
	}
	diags.Append(obj.As(ctx, &raw, basicObjectAsOpts())...)
	if diags.HasError() {
		return nil
	}
	return &client.DeploymentSchedule{
		Type:       raw.Type.ValueString(),
		Expression: raw.Expression.ValueString(),
		Timezone:   raw.Timezone.ValueString(),
	}
}

// --- paused_reason -----------------------------------------------------------

func pausedReasonFromAPI(pr *client.DeploymentPausedReason, diags *diag.Diagnostics) types.Object {
	if pr == nil {
		return types.ObjectNull(pausedReasonObjectAttrTypes())
	}
	errObj := types.ObjectNull(pausedErrorObjectAttrTypes())
	if pr.Error != nil {
		o, d := types.ObjectValue(pausedErrorObjectAttrTypes(), map[string]attr.Value{
			"type":    types.StringValue(pr.Error.Type),
			"message": stringOrNull(pr.Error.Message),
		})
		diags.Append(d...)
		errObj = o
	}
	obj, d := types.ObjectValue(pausedReasonObjectAttrTypes(), map[string]attr.Value{
		"type":  types.StringValue(pr.Type),
		"error": errObj,
	})
	diags.Append(d...)
	return obj
}

// --- small shared helpers ----------------------------------------------------

// stringOrNull returns a null TF string for the empty Go string, else a value.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// stringSliceToList converts a Go string slice to a TF list. Empty/nil → null
// (keeps plans clean for the unset optional case).
func stringSliceToList(in []string, diags *diag.Diagnostics) types.List {
	if len(in) == 0 {
		return types.ListNull(types.StringType)
	}
	items := make([]attr.Value, 0, len(in))
	for _, s := range in {
		items = append(items, types.StringValue(s))
	}
	list, d := types.ListValue(types.StringType, items)
	diags.Append(d...)
	return list
}

// listToStringSlice converts a TF list of strings to a Go slice. Null/unknown
// → nil.
func listToStringSlice(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// metadataMergePtr adapts metadataMerge (which returns map[string]any with nil
// for deletes) into the map[string]*string shape DeploymentUpdateRequest
// expects. A nil value marshals to JSON null (delete the key).
func metadataMergePtr(ctx context.Context, plan, state types.Map) (map[string]*string, diag.Diagnostics) {
	merged, diags := metadataMerge(ctx, plan, state)
	if merged == nil {
		return nil, diags
	}
	out := make(map[string]*string, len(merged))
	for k, v := range merged {
		if v == nil {
			out[k] = nil
			continue
		}
		s := v.(string)
		out[k] = &s
	}
	return out, diags
}

// canonicalJSONArray marshals a content-block slice into a canonical compact
// JSON string. Re-encoding through `any` sorts object keys alphabetically and
// strips whitespace, matching Terraform's jsonencode output so config and
// state converge (no perpetual diff on refresh).
func canonicalJSONArray(blocks []json.RawMessage) (string, error) {
	raw, err := json.Marshal(blocks)
	if err != nil {
		return "", err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	out, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
