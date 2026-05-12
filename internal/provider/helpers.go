package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/asvirida/terraform-provider-claude-managed-agents/internal/client"
)

// agentFromAPI maps a client.Agent into the Terraform schema model.
func agentFromAPI(ctx context.Context, a *client.Agent, diags *diag.Diagnostics) agentModel {
	m := agentModel{
		ID:        types.StringValue(a.ID),
		Name:      types.StringValue(a.Name),
		Model:     types.StringValue(a.Model.ID),
		Version:   types.Int64Value(int64(a.Version)),
		CreatedAt: types.StringValue(a.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt: types.StringValue(a.UpdatedAt.Format(timeFormatRFC3339)),
	}

	if a.System != nil {
		m.System = types.StringValue(*a.System)
	} else {
		m.System = types.StringNull()
	}
	if a.Description != nil {
		m.Description = types.StringValue(*a.Description)
	} else {
		m.Description = types.StringNull()
	}
	if a.ArchivedAt != nil {
		m.ArchivedAt = types.StringValue(a.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		m.ArchivedAt = types.StringNull()
	}

	mdMap, d := stringMapToMap(ctx, a.Metadata)
	diags.Append(d...)
	m.Metadata = mdMap

	return m
}

// stringMapToMap converts a Go map[string]string into types.Map. A nil input
// — or an empty map — becomes a null Terraform map. Treating empty the same
// as null keeps Terraform plans clean for users who do not configure the
// attribute at all, since the upstream API returns `{}` for unset metadata.
func stringMapToMap(ctx context.Context, in map[string]string) (types.Map, diag.Diagnostics) {
	if len(in) == 0 {
		return types.MapNull(types.StringType), nil
	}
	elements := make(map[string]string, len(in))
	for k, v := range in {
		elements[k] = v
	}
	return types.MapValueFrom(ctx, types.StringType, elements)
}

// mapToStringMap converts types.Map into Go map[string]string. Null or
// unknown returns a nil map.
func mapToStringMap(ctx context.Context, m types.Map) (map[string]string, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	out := make(map[string]string, len(m.Elements()))
	d := m.ElementsAs(ctx, &out, false)
	return out, d
}

// metadataMerge produces the metadata payload for an update call. Per the
// upstream API: setting a key to an empty string deletes it. So we send
// every key the user kept plus an explicit "" for any key the state had but
// the plan dropped.
func metadataMerge(ctx context.Context, plan, state types.Map) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	planned, d := mapToStringMap(ctx, plan)
	diags.Append(d...)
	current, d := mapToStringMap(ctx, state)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	out := make(map[string]string, len(planned)+len(current))
	for k, v := range planned {
		out[k] = v
	}
	for k := range current {
		if _, kept := planned[k]; !kept {
			out[k] = ""
		}
	}
	if len(out) == 0 {
		return nil, diags
	}
	return out, diags
}

// timeFormatRFC3339 is the standard format we emit timestamps in. Using a
// constant makes it trivial to switch to RFC3339Nano later if needed.
const timeFormatRFC3339 = "2006-01-02T15:04:05Z07:00"
