package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

// Helpers for the `environment_variable` vault credential type. API reference:
// https://platform.claude.com/docs/en/api/beta/vaults/credentials/create
// https://platform.claude.com/docs/en/api/beta/vaults/credentials/update

// networkingToAPI renders the decoded networking block as the API object.
func networkingToAPI(v authDecoded) map[string]any {
	out := map[string]any{"type": v.networkingType}
	if v.allowedHostsSet {
		hosts := v.allowedHosts
		if hosts == nil {
			hosts = []string{}
		}
		out["allowed_hosts"] = hosts
	}
	return out
}

// injectionLocationToAPI renders the known injection_location booleans, or
// nil when the block is null/unknown (the API default then applies).
func injectionLocationToAPI(v authDecoded) map[string]any {
	if v.injectionLocation.IsNull() || v.injectionLocation.IsUnknown() {
		return nil
	}
	out := map[string]any{}
	if !v.injectHeader.IsNull() && !v.injectHeader.IsUnknown() {
		out["header"] = v.injectHeader.ValueBool()
	}
	if !v.injectBody.IsNull() && !v.injectBody.IsUnknown() {
		out["body"] = v.injectBody.ValueBool()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func networkingFromAPI(n *client.VaultCredentialAuthNetworking) (types.Object, diag.Diagnostics) {
	if n == nil {
		return types.ObjectNull(credentialNetworkingAttrTypes()), nil
	}
	var diags diag.Diagnostics
	hosts := types.ListNull(types.StringType)
	if len(n.AllowedHosts) > 0 {
		elems := make([]attr.Value, 0, len(n.AllowedHosts))
		for _, h := range n.AllowedHosts {
			elems = append(elems, types.StringValue(h))
		}
		l, d := types.ListValue(types.StringType, elems)
		diags.Append(d...)
		hosts = l
	}
	obj, d := types.ObjectValue(credentialNetworkingAttrTypes(), map[string]attr.Value{
		"type":          types.StringValue(n.Type),
		"allowed_hosts": hosts,
	})
	diags.Append(d...)
	return obj, diags
}

func injectionLocationFromAPI(il *client.VaultCredentialAuthInjectionLocation) (types.Object, diag.Diagnostics) {
	if il == nil {
		return types.ObjectNull(credentialInjectionLocationAttrTypes()), nil
	}
	return types.ObjectValue(credentialInjectionLocationAttrTypes(), map[string]attr.Value{
		"header": types.BoolValue(il.Header),
		"body":   types.BoolValue(il.Body),
	})
}

// injectionLocationPlanModifier keeps `auth.injection_location` null for
// credential types that have no injection location, and otherwise behaves
// like UseStateForUnknown so the API-defaulted value does not show as
// "(known after apply)" on every unrelated update.
type injectionLocationPlanModifier struct{}

func (injectionLocationPlanModifier) Description(_ context.Context) string {
	return "Null for non-environment_variable credentials; otherwise keeps the prior state value when unconfigured."
}

func (m injectionLocationPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (injectionLocationPlanModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	var typ types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("auth").AtName("type"), &typ)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !typ.IsNull() && !typ.IsUnknown() && typ.ValueString() != credTypeEnvVar {
		if req.ConfigValue.IsNull() {
			resp.PlanValue = types.ObjectNull(credentialInjectionLocationAttrTypes())
		}
		return
	}
	if !req.PlanValue.IsUnknown() || req.StateValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	resp.PlanValue = req.StateValue
}

// ValidateConfig enforces the per-type required and forbidden auth
// attributes, which the schema cannot express because `auth` is one object
// shared by every credential type.
func (r *vaultCredentialResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var authObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("auth"), &authObj)...)
	if resp.Diagnostics.HasError() || authObj.IsNull() || authObj.IsUnknown() {
		return
	}

	attrs := authObj.Attributes()
	typ, _ := attrs["type"].(types.String)
	if typ.IsNull() || typ.IsUnknown() {
		return
	}
	authPath := path.Root("auth")
	isSet := func(name string) bool {
		v, ok := attrs[name]
		return ok && !v.IsNull()
	}
	require := func(name string) {
		if !isSet(name) {
			resp.Diagnostics.AddAttributeError(authPath.AtName(name), "Missing required attribute",
				fmt.Sprintf("auth.%s is required when auth.type is %q.", name, typ.ValueString()))
		}
	}
	forbid := func(name string) {
		if isSet(name) {
			resp.Diagnostics.AddAttributeError(authPath.AtName(name), "Invalid attribute for credential type",
				fmt.Sprintf("auth.%s must not be set when auth.type is %q.", name, typ.ValueString()))
		}
	}

	envVarOnly := []string{"secret_name", "secret_value", "secret_value_wo_version", "networking", "injection_location"}
	mcpOnly := []string{"mcp_server_url", "token", "token_wo_version", "access_token", "access_token_wo_version", "expires_at", "refresh"}

	switch typ.ValueString() {
	case credTypeStaticBearer, credTypeMCPOAuth:
		require("mcp_server_url")
		for _, n := range envVarOnly {
			forbid(n)
		}
	case credTypeEnvVar:
		for _, n := range mcpOnly {
			forbid(n)
		}
		require("secret_name")
		require("secret_value")
		require("networking")
		validateNetworking(attrs["networking"], authPath.AtName("networking"), &resp.Diagnostics)
	default:
		resp.Diagnostics.AddAttributeError(authPath.AtName("type"), "Invalid credential type",
			fmt.Sprintf("auth.type must be one of %q, %q, or %q; got %q.",
				credTypeStaticBearer, credTypeMCPOAuth, credTypeEnvVar, typ.ValueString()))
	}
}

func validateNetworking(v attr.Value, p path.Path, diags *diag.Diagnostics) {
	obj, ok := v.(types.Object)
	if !ok || obj.IsNull() || obj.IsUnknown() {
		return
	}
	attrs := obj.Attributes()
	nt, _ := attrs["type"].(types.String)
	if nt.IsNull() || nt.IsUnknown() {
		return
	}
	hosts, _ := attrs["allowed_hosts"].(types.List)
	switch nt.ValueString() {
	case "limited":
		if hosts.IsNull() {
			diags.AddAttributeError(p.AtName("allowed_hosts"), "Missing required attribute",
				"auth.networking.allowed_hosts is required when auth.networking.type is \"limited\".")
			return
		}
		if !hosts.IsUnknown() {
			n := len(hosts.Elements())
			if n == 0 || n > 16 {
				diags.AddAttributeError(p.AtName("allowed_hosts"), "Invalid allowed_hosts",
					fmt.Sprintf("auth.networking.allowed_hosts must have between 1 and 16 entries; got %d.", n))
			}
		}
	case "unrestricted":
		if !hosts.IsNull() {
			diags.AddAttributeError(p.AtName("allowed_hosts"), "Invalid attribute for networking type",
				"auth.networking.allowed_hosts must not be set when auth.networking.type is \"unrestricted\".")
		}
	default:
		diags.AddAttributeError(p.AtName("type"), "Invalid networking type",
			fmt.Sprintf("auth.networking.type must be \"limited\" or \"unrestricted\"; got %q.", nt.ValueString()))
	}
}
