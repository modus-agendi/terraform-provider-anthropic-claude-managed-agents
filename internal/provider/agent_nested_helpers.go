package provider

import (
	"context"
	"encoding/json"

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

// skillObjectAttrTypes returns the attribute-type map for one entry of the
// agent's `skills` list.
func skillObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"skill_id": types.StringType,
		"version":  types.StringType,
	}
}

// multiagentObjectAttrTypes returns the attribute-type map for the
// `multiagent` single-nested coordinator block.
func multiagentObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":   types.StringType,
		"agents": types.ListType{ElemType: types.ObjectType{AttrTypes: multiagentMemberObjectAttrTypes()}},
	}
}

// multiagentMemberObjectAttrTypes returns the attribute-type map for one
// entry of the coordinator's `agents` list.
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

// skillsListFromAPI mirrors mcpServersListFromAPI for the `skills` field.
func skillsListFromAPI(ctx context.Context, skills []client.AgentSkillRef) (types.List, diag.Diagnostics) {
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

// skillsListToAPI is the inverse of skillsListFromAPI.
func skillsListToAPI(ctx context.Context, l types.List) ([]client.AgentSkillRef, diag.Diagnostics) {
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
	out := make([]client.AgentSkillRef, 0, len(entries))
	for _, e := range entries {
		out = append(out, client.AgentSkillRef{
			Type:    e.Type.ValueString(),
			SkillID: e.SkillID.ValueString(),
			Version: e.Version.ValueString(),
		})
	}
	return out, diags
}

// multiagentFromAPI maps the optional coordinator block. nil → object-null.
// The API normalizes `{type: "self"}` entries to `{type: "agent", id: <parent>}`
// on response; we detect that pattern using parentID and rewrite it back to
// the user-input shape so state matches the HCL config.
func multiagentFromAPI(ctx context.Context, m *client.Multiagent, parentID string) (types.Object, diag.Diagnostics) {
	if m == nil {
		return types.ObjectNull(multiagentObjectAttrTypes()), nil
	}
	var diags diag.Diagnostics
	memberObjType := types.ObjectType{AttrTypes: multiagentMemberObjectAttrTypes()}
	items := make([]attr.Value, 0, len(m.Agents))
	for _, a := range m.Agents {
		entryType := a.Type
		id := types.StringNull()
		switch {
		case entryType == "self":
			// Defensive: real API rewrites this to "agent"; keep handling
			// in case some response path returns the literal "self".
		case entryType == "agent" && a.ID != "" && a.ID == parentID:
			entryType = "self"
		case a.ID != "":
			id = types.StringValue(a.ID)
		}
		obj, d := types.ObjectValue(multiagentMemberObjectAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(entryType),
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

// --- tools mapping ---

// toolObjectAttrTypes is the union shape: every variant's fields are
// present; only the relevant ones are populated per entry.
func toolObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"mcp_server_name": types.StringType,
		"name":            types.StringType,
		"description":     types.StringType,
		"input_schema":    types.StringType,
		"default_config":  types.ObjectType{AttrTypes: defaultConfigAttrTypes()},
		"configs":         types.ListType{ElemType: types.ObjectType{AttrTypes: toolConfigEntryAttrTypes()}},
	}
}

// defaultConfigAttrTypes is the shape of the toolset-wide `default_config`
// (no per-tool name).
func defaultConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":           types.BoolType,
		"permission_policy": types.ObjectType{AttrTypes: permissionPolicyAttrTypes()},
	}
}

// toolConfigEntryAttrTypes is the shape of one `configs[*]` entry (per-tool
// override, identified by name).
func toolConfigEntryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":              types.StringType,
		"enabled":           types.BoolType,
		"permission_policy": types.ObjectType{AttrTypes: permissionPolicyAttrTypes()},
	}
}

// permissionPolicyAttrTypes is the single-field shape for both default and
// per-tool permission policies (`always_allow` or `always_ask`).
func permissionPolicyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
	}
}

// toolsListFromAPI maps the API response into a TF list, preserving the
// prior plan/state's default_config and configs fields where the API
// enriches them (the real API populates default_config.enabled and
// per-tool permission_policy on read; we want state to mirror user intent,
// not the API's defaults).
//
// `prior` is the previous plan or state's tools list; pass an empty/null
// list for first-time reads.
func toolsListFromAPI(ctx context.Context, tools []client.Tool, prior types.List) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: toolObjectAttrTypes()}
	var diags diag.Diagnostics

	// Decode prior into a slice we can index by position. The API preserves
	// tool ordering, so we align on index.
	priorByIdx := decodeToolsPrior(ctx, prior, &diags)
	if diags.HasError() {
		return types.ListNull(objType), diags
	}

	items := make([]attr.Value, 0, len(tools))
	for i, t := range tools {
		var priorEntry *priorTool
		if i < len(priorByIdx) {
			priorEntry = &priorByIdx[i]
		}

		// default_config + configs: preserve prior state if any; otherwise
		// drop the API-enriched value to keep plan/state in sync.
		var defaultCfg types.Object
		var configs types.List
		if priorEntry != nil {
			defaultCfg = priorEntry.DefaultConfig
			configs = priorEntry.Configs
		} else {
			defaultCfg = types.ObjectNull(defaultConfigAttrTypes())
			configs = types.ListNull(types.ObjectType{AttrTypes: toolConfigEntryAttrTypes()})
		}

		mcpName := types.StringNull()
		if t.McpServerName != "" {
			mcpName = types.StringValue(t.McpServerName)
		}
		name := types.StringNull()
		if t.Name != "" {
			name = types.StringValue(t.Name)
		}
		desc := types.StringNull()
		if t.Description != "" {
			desc = types.StringValue(t.Description)
		}
		schema := types.StringNull()
		if len(t.InputSchema) > 0 && string(t.InputSchema) != "null" {
			schema = types.StringValue(string(t.InputSchema))
		}

		obj, d := types.ObjectValue(toolObjectAttrTypes(), map[string]attr.Value{
			"type":            types.StringValue(t.Type),
			"mcp_server_name": mcpName,
			"name":            name,
			"description":     desc,
			"input_schema":    schema,
			"default_config":  defaultCfg,
			"configs":         configs,
		})
		diags.Append(d...)
		items = append(items, obj)
	}
	list, d := types.ListValue(objType, items)
	diags.Append(d...)
	return list, diags
}

// priorTool is the subset of tools[*] fields we preserve from prior state.
type priorTool struct {
	DefaultConfig types.Object
	Configs       types.List
}

// decodeToolsPrior extracts the (default_config, configs) tuples from a
// prior-state tools list so toolsListFromAPI can preserve them through Read.
func decodeToolsPrior(ctx context.Context, prior types.List, diags *diag.Diagnostics) []priorTool {
	if prior.IsNull() || prior.IsUnknown() {
		return nil
	}
	type entry struct {
		Type          types.String `tfsdk:"type"`
		McpServerName types.String `tfsdk:"mcp_server_name"`
		Name          types.String `tfsdk:"name"`
		Description   types.String `tfsdk:"description"`
		InputSchema   types.String `tfsdk:"input_schema"`
		DefaultConfig types.Object `tfsdk:"default_config"`
		Configs       types.List   `tfsdk:"configs"`
	}
	var entries []entry
	diags.Append(prior.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil
	}
	out := make([]priorTool, len(entries))
	for i, e := range entries {
		out[i] = priorTool{DefaultConfig: e.DefaultConfig, Configs: e.Configs}
	}
	return out
}

// toolsListToAPI flattens the HCL `tools` list into the client struct slice.
// Null/unknown inputs return nil (treated by the caller as "leave unchanged"
// or "send no tools field").
func toolsListToAPI(ctx context.Context, l types.List) ([]client.Tool, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	type toolModel struct {
		Type          types.String `tfsdk:"type"`
		McpServerName types.String `tfsdk:"mcp_server_name"`
		Name          types.String `tfsdk:"name"`
		Description   types.String `tfsdk:"description"`
		InputSchema   types.String `tfsdk:"input_schema"`
		DefaultConfig types.Object `tfsdk:"default_config"`
		Configs       types.List   `tfsdk:"configs"`
	}
	var entries []toolModel
	diags.Append(l.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]client.Tool, 0, len(entries))
	for _, e := range entries {
		t := client.Tool{
			Type:          e.Type.ValueString(),
			McpServerName: e.McpServerName.ValueString(),
			Name:          e.Name.ValueString(),
			Description:   e.Description.ValueString(),
		}
		if !e.InputSchema.IsNull() && !e.InputSchema.IsUnknown() && e.InputSchema.ValueString() != "" {
			t.InputSchema = json.RawMessage(e.InputSchema.ValueString())
		}
		if !e.DefaultConfig.IsNull() && !e.DefaultConfig.IsUnknown() {
			cfg, d := toolConfigToAPI(ctx, e.DefaultConfig)
			diags.Append(d...)
			t.DefaultConfig = cfg
		}
		if !e.Configs.IsNull() && !e.Configs.IsUnknown() {
			cfgs, d := toolConfigListToAPI(ctx, e.Configs)
			diags.Append(d...)
			t.Configs = cfgs
		}
		out = append(out, t)
	}
	return out, diags
}

// toolConfigToAPI maps a default_config object (no name field) into the
// client struct.
func toolConfigToAPI(ctx context.Context, obj types.Object) (*client.ToolConfig, diag.Diagnostics) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	var raw struct {
		Enabled          types.Bool   `tfsdk:"enabled"`
		PermissionPolicy types.Object `tfsdk:"permission_policy"`
	}
	diags.Append(obj.As(ctx, &raw, basicObjectAsOpts())...)
	if diags.HasError() {
		return nil, diags
	}
	out := &client.ToolConfig{}
	if !raw.Enabled.IsNull() && !raw.Enabled.IsUnknown() {
		b := raw.Enabled.ValueBool()
		out.Enabled = &b
	}
	if !raw.PermissionPolicy.IsNull() && !raw.PermissionPolicy.IsUnknown() {
		pp, d := permissionPolicyToAPI(ctx, raw.PermissionPolicy)
		diags.Append(d...)
		out.PermissionPolicy = pp
	}
	return out, diags
}

// toolConfigListToAPI maps a configs list (each entry has a `name`) into
// the client struct slice.
func toolConfigListToAPI(ctx context.Context, l types.List) ([]client.ToolConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	type entry struct {
		Name             types.String `tfsdk:"name"`
		Enabled          types.Bool   `tfsdk:"enabled"`
		PermissionPolicy types.Object `tfsdk:"permission_policy"`
	}
	var entries []entry
	diags.Append(l.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]client.ToolConfig, 0, len(entries))
	for _, e := range entries {
		cfg := client.ToolConfig{Name: e.Name.ValueString()}
		if !e.Enabled.IsNull() && !e.Enabled.IsUnknown() {
			b := e.Enabled.ValueBool()
			cfg.Enabled = &b
		}
		if !e.PermissionPolicy.IsNull() && !e.PermissionPolicy.IsUnknown() {
			pp, d := permissionPolicyToAPI(ctx, e.PermissionPolicy)
			diags.Append(d...)
			cfg.PermissionPolicy = pp
		}
		out = append(out, cfg)
	}
	return out, diags
}

// permissionPolicyToAPI extracts {type: "..."} from the nested object.
func permissionPolicyToAPI(ctx context.Context, obj types.Object) (*client.PermissionPolicy, diag.Diagnostics) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var diags diag.Diagnostics
	var raw struct {
		Type types.String `tfsdk:"type"`
	}
	diags.Append(obj.As(ctx, &raw, basicObjectAsOpts())...)
	if diags.HasError() {
		return nil, diags
	}
	return &client.PermissionPolicy{Type: raw.Type.ValueString()}, diags
}

// multiagentToAPI is the inverse of multiagentFromAPI for the create/update path.
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
