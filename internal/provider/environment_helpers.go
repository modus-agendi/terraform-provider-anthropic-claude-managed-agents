package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

// configModel mirrors the `config` SingleNestedAttribute as Go types. Used by
// configToAPI to decode the planned object into a client.CloudConfig.
type configModel struct {
	Type       types.String `tfsdk:"type"`
	Packages   types.Object `tfsdk:"packages"`
	Networking types.Object `tfsdk:"networking"`
}

type packagesModel struct {
	Apt   types.List `tfsdk:"apt"`
	Cargo types.List `tfsdk:"cargo"`
	Gem   types.List `tfsdk:"gem"`
	Go    types.List `tfsdk:"go"`
	Npm   types.List `tfsdk:"npm"`
	Pip   types.List `tfsdk:"pip"`
}

type networkingModel struct {
	Type                 types.String `tfsdk:"type"`
	AllowedHosts         types.List   `tfsdk:"allowed_hosts"`
	AllowMcpServers      types.Bool   `tfsdk:"allow_mcp_servers"`
	AllowPackageManagers types.Bool   `tfsdk:"allow_package_managers"`
}

// configToAPI converts the planned `config` object into the client struct.
func configToAPI(ctx context.Context, obj types.Object) (client.CloudConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var cfg configModel
	diags.Append(obj.As(ctx, &cfg, basicObjectAsOpts())...)
	if diags.HasError() {
		return client.CloudConfig{}, diags
	}

	out := client.CloudConfig{Type: cfg.Type.ValueString()}

	if !cfg.Packages.IsNull() && !cfg.Packages.IsUnknown() {
		var pkgs packagesModel
		diags.Append(cfg.Packages.As(ctx, &pkgs, basicObjectAsOpts())...)
		if diags.HasError() {
			return out, diags
		}
		p := &client.Packages{}
		var d diag.Diagnostics
		if p.Apt, d = listToStrings(ctx, pkgs.Apt); d.HasError() {
			diags.Append(d...)
		}
		if p.Cargo, d = listToStrings(ctx, pkgs.Cargo); d.HasError() {
			diags.Append(d...)
		}
		if p.Gem, d = listToStrings(ctx, pkgs.Gem); d.HasError() {
			diags.Append(d...)
		}
		if p.Go, d = listToStrings(ctx, pkgs.Go); d.HasError() {
			diags.Append(d...)
		}
		if p.Npm, d = listToStrings(ctx, pkgs.Npm); d.HasError() {
			diags.Append(d...)
		}
		if p.Pip, d = listToStrings(ctx, pkgs.Pip); d.HasError() {
			diags.Append(d...)
		}
		if diags.HasError() {
			return out, diags
		}
		out.Packages = p
	}

	var net networkingModel
	diags.Append(cfg.Networking.As(ctx, &net, basicObjectAsOpts())...)
	if diags.HasError() {
		return out, diags
	}
	out.Networking.Type = net.Type.ValueString()
	if !net.AllowedHosts.IsNull() && !net.AllowedHosts.IsUnknown() {
		hosts, d := listToStrings(ctx, net.AllowedHosts)
		diags.Append(d...)
		if diags.HasError() {
			return out, diags
		}
		out.Networking.AllowedHosts = hosts
	}
	if !net.AllowMcpServers.IsNull() && !net.AllowMcpServers.IsUnknown() {
		b := net.AllowMcpServers.ValueBool()
		out.Networking.AllowMcpServers = &b
	}
	if !net.AllowPackageManagers.IsNull() && !net.AllowPackageManagers.IsUnknown() {
		b := net.AllowPackageManagers.ValueBool()
		out.Networking.AllowPackageManagers = &b
	}

	return out, diags
}

// environmentFromAPI maps a client.Environment into the Terraform schema model.
func environmentFromAPI(ctx context.Context, e *client.Environment) (environmentModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	m := environmentModel{
		ID:        types.StringValue(e.ID),
		Name:      types.StringValue(e.Name),
		CreatedAt: types.StringValue(e.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt: types.StringValue(e.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if e.ArchivedAt != nil {
		m.ArchivedAt = types.StringValue(e.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		m.ArchivedAt = types.StringNull()
	}

	cfgObj, d := cloudConfigToObject(ctx, &e.Config)
	diags.Append(d...)
	m.Config = cfgObj
	return m, diags
}

func cloudConfigToObject(ctx context.Context, c *client.CloudConfig) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	pkgsObj, d := packagesToObject(ctx, c.Packages)
	diags.Append(d...)
	netObj, d := networkingToObject(ctx, &c.Networking)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(configObjectAttrTypes()), diags
	}

	obj, d := types.ObjectValue(configObjectAttrTypes(), map[string]attr.Value{
		"type":       types.StringValue(c.Type),
		"packages":   pkgsObj,
		"networking": netObj,
	})
	diags.Append(d...)
	return obj, diags
}

func packagesToObject(ctx context.Context, p *client.Packages) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	// Treat nil + "all empty slices" as equivalent: the real API returns
	// a non-nil object with empty per-language lists when packages is
	// unset, which we normalize to a null Terraform object so plans stay
	// clean for users who don't configure packages.
	if p == nil || (len(p.Apt) == 0 && len(p.Cargo) == 0 && len(p.Gem) == 0 && len(p.Go) == 0 && len(p.Npm) == 0 && len(p.Pip) == 0) {
		return types.ObjectNull(packagesObjectAttrTypes()), diags
	}

	apt, d := stringsToList(ctx, p.Apt)
	diags.Append(d...)
	cargo, d := stringsToList(ctx, p.Cargo)
	diags.Append(d...)
	gem, d := stringsToList(ctx, p.Gem)
	diags.Append(d...)
	goList, d := stringsToList(ctx, p.Go)
	diags.Append(d...)
	npm, d := stringsToList(ctx, p.Npm)
	diags.Append(d...)
	pip, d := stringsToList(ctx, p.Pip)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(packagesObjectAttrTypes()), diags
	}

	obj, d := types.ObjectValue(packagesObjectAttrTypes(), map[string]attr.Value{
		"apt":   apt,
		"cargo": cargo,
		"gem":   gem,
		"go":    goList,
		"npm":   npm,
		"pip":   pip,
	})
	diags.Append(d...)
	return obj, diags
}

func networkingToObject(ctx context.Context, n *client.Networking) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	hosts, d := stringsToList(ctx, n.AllowedHosts)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(networkingObjectAttrTypes()), diags
	}

	allowMcp := types.BoolNull()
	if n.AllowMcpServers != nil {
		allowMcp = types.BoolValue(*n.AllowMcpServers)
	}
	allowPkg := types.BoolNull()
	if n.AllowPackageManagers != nil {
		allowPkg = types.BoolValue(*n.AllowPackageManagers)
	}

	obj, d := types.ObjectValue(networkingObjectAttrTypes(), map[string]attr.Value{
		"type":                   types.StringValue(n.Type),
		"allowed_hosts":          hosts,
		"allow_mcp_servers":      allowMcp,
		"allow_package_managers": allowPkg,
	})
	diags.Append(d...)
	return obj, diags
}

// stringsToList converts a Go []string into a types.List of strings. nil or
// empty slice becomes a null list — matches the upstream API's `omitempty`
// behavior so plans stay clean.
func stringsToList(ctx context.Context, in []string) (types.List, diag.Diagnostics) {
	if len(in) == 0 {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, in)
}

// listToStrings is the inverse of stringsToList.
func listToStrings(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	out := make([]string, 0, len(l.Elements()))
	d := l.ElementsAs(ctx, &out, false)
	return out, d
}

// basicObjectAsOpts returns the default Object.As options used throughout
// this package. Kept in a helper to centralize the "allow null/unknown"
// preference if we ever want to change it.
func basicObjectAsOpts() basicObjectOpts {
	return basicObjectOpts{}
}

// basicObjectOpts is a thin alias so we can pass an empty struct without
// importing the framework types repeatedly at each call site.
type basicObjectOpts = struct {
	UnhandledNullAsEmpty    bool
	UnhandledUnknownAsEmpty bool
}
