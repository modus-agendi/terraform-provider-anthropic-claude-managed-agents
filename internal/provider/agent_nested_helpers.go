package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

// mcpServerObjectAttrTypes returns the attribute-type map for one entry
// of the agent's `mcp_servers` list.
func mcpServerObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"name": types.StringType,
		"url":  types.StringType,
	}
}

func skillObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"skill_id": types.StringType,
		"version":  types.StringType,
	}
}

func multiagentObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":   types.StringType,
		"agents": types.ListType{ElemType: types.ObjectType{AttrTypes: multiagentMemberObjectAttrTypes()}},
	}
}

func multiagentMemberObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"id":   types.StringType,
	}
}

// mcpServersListFromAPI converts the typed client slice into a types.List.
// Empty input → empty list (not null) so the value is stable across plan,
// apply, and refresh. The attribute is Computed, so users who never set it
// will see an empty list in state.
func mcpServersListFromAPI(ctx context.Context, servers []client.McpServer) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: mcpServerObjectAttrTypes()}
	var diags diag.Diagnostics
	items := make([]attr.Value, 0, len(servers))
	for _, s := range servers {
		obj, d := types.ObjectValue(mcpServerObjectAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(s.Type),
			"name": types.StringValue(s.Name),
			"url":  types.StringValue(s.URL),
		})
		diags.Append(d...)
		items = append(items, obj)
	}
	list, d := types.ListValue(objType, items)
	diags.Append(d...)
	return list, diags
}

func mcpServersListToAPI(ctx context.Context, l types.List) ([]client.McpServer, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	type modelEntry struct {
		Type types.String `tfsdk:"type"`
		Name types.String `tfsdk:"name"`
		URL  types.String `tfsdk:"url"`
	}
	var entries []modelEntry
	diags.Append(l.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]client.McpServer, 0, len(entries))
	for _, e := range entries {
		out = append(out, client.McpServer{
			Type: e.Type.ValueString(),
			Name: e.Name.ValueString(),
			URL:  e.URL.ValueString(),
		})
	}
	return out, diags
}

func skillsListFromAPI(ctx context.Context, skills []client.Skill) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: skillObjectAttrTypes()}
	var diags diag.Diagnostics
	items := make([]attr.Value, 0, len(skills))
	for _, s := range skills {
		version := types.StringNull()
		if s.Version != "" {
			version = types.StringValue(s.Version)
		}
		obj, d := types.ObjectValue(skillObjectAttrTypes(), map[string]attr.Value{
			"type":     types.StringValue(s.Type),
			"skill_id": types.StringValue(s.SkillID),
			"version":  version,
		})
		diags.Append(d...)
		items = append(items, obj)
	}
	list, d := types.ListValue(objType, items)
	diags.Append(d...)
	return list, diags
}

func skillsListToAPI(ctx context.Context, l types.List) ([]client.Skill, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	type modelEntry struct {
		Type    types.String `tfsdk:"type"`
		SkillID types.String `tfsdk:"skill_id"`
		Version types.String `tfsdk:"version"`
	}
	var entries []modelEntry
	diags.Append(l.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]client.Skill, 0, len(entries))
	for _, e := range entries {
		out = append(out, client.Skill{
			Type:    e.Type.ValueString(),
			SkillID: e.SkillID.ValueString(),
			Version: e.Version.ValueString(),
		})
	}
	return out, diags
}

func multiagentFromAPI(ctx context.Context, m *client.Multiagent) (types.Object, diag.Diagnostics) {
	if m == nil {
		return types.ObjectNull(multiagentObjectAttrTypes()), nil
	}
	var diags diag.Diagnostics
	memberObjType := types.ObjectType{AttrTypes: multiagentMemberObjectAttrTypes()}
	items := make([]attr.Value, 0, len(m.Agents))
	for _, a := range m.Agents {
		id := types.StringNull()
		if a.ID != "" {
			id = types.StringValue(a.ID)
		}
		obj, d := types.ObjectValue(multiagentMemberObjectAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(a.Type),
			"id":   id,
		})
		diags.Append(d...)
		items = append(items, obj)
	}
	agentsList, d := types.ListValue(memberObjType, items)
	diags.Append(d...)
	obj, d := types.ObjectValue(multiagentObjectAttrTypes(), map[string]attr.Value{
		"type":   types.StringValue(m.Type),
		"agents": agentsList,
	})
	diags.Append(d...)
	return obj, diags
}

func multiagentToAPI(ctx context.Context, obj types.Object) (*client.Multiagent, diag.Diagnostics) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	var raw struct {
		Type   types.String `tfsdk:"type"`
		Agents types.List   `tfsdk:"agents"`
	}
	diags.Append(obj.As(ctx, &raw, basicObjectAsOpts())...)
	if diags.HasError() {
		return nil, diags
	}
	out := &client.Multiagent{Type: raw.Type.ValueString()}
	if !raw.Agents.IsNull() && !raw.Agents.IsUnknown() {
		type memberModel struct {
			Type types.String `tfsdk:"type"`
			ID   types.String `tfsdk:"id"`
		}
		var members []memberModel
		diags.Append(raw.Agents.ElementsAs(ctx, &members, false)...)
		if diags.HasError() {
			return nil, diags
		}
		for _, m := range members {
			out.Agents = append(out.Agents, client.MultiagentMember{
				Type: m.Type.ValueString(),
				ID:   m.ID.ValueString(),
			})
		}
	}
	return out, diags
}
