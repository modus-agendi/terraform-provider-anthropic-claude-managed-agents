package provider

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

// TestEnvironmentResource_Update_AlwaysErrors locks in the contract that the
// Update method returns an error: every attribute is RequiresReplace, so
// Update should never be reachable through normal Terraform flow. If anyone
// ever removes the RequiresReplace modifiers without wiring a real update
// path, this test will fail and force a deliberate decision.
func TestEnvironmentResource_Update_AlwaysErrors(t *testing.T) {
	r := &environmentResource{}
	var resp resource.UpdateResponse
	r.Update(context.Background(), resource.UpdateRequest{}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("Update should always produce an error diagnostic")
	}
}

func TestEnvironmentFromAPI_Archived(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	archived := now.Add(time.Hour)
	allow := true
	env := &client.Environment{
		ID:        "env_test",
		Name:      "x",
		CreatedAt: now,
		UpdatedAt: now,
		Config: client.CloudConfig{
			Type:     "cloud",
			Packages: &client.Packages{Pip: []string{"pandas"}, Apt: []string{"jq"}},
			Networking: client.Networking{
				Type:                 "limited",
				AllowedHosts:         []string{"https://example.com"},
				AllowMcpServers:      &allow,
				AllowPackageManagers: &allow,
			},
		},
		ArchivedAt: &archived,
	}

	m, diags := environmentFromAPI(context.Background(), env)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if m.ID.ValueString() != "env_test" {
		t.Errorf("ID = %q", m.ID.ValueString())
	}
	if m.ArchivedAt.IsNull() {
		t.Errorf("ArchivedAt should be populated")
	}
}

func TestEnvironmentFromAPI_NoPackages(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	env := &client.Environment{
		ID:        "env_2",
		Name:      "y",
		CreatedAt: now,
		UpdatedAt: now,
		Config: client.CloudConfig{
			Type:       "cloud",
			Networking: client.Networking{Type: "unrestricted"},
		},
	}

	m, diags := environmentFromAPI(context.Background(), env)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if !m.ArchivedAt.IsNull() {
		t.Errorf("ArchivedAt should be null")
	}

	// Drill into m.Config to verify packages is null.
	cfgAttrs := m.Config.Attributes()
	if cfgAttrs["packages"] == nil || !cfgAttrs["packages"].(types.Object).IsNull() {
		t.Errorf("packages should be null, got %v", cfgAttrs["packages"])
	}
}

func TestConfigToAPI_Unrestricted(t *testing.T) {
	ctx := context.Background()
	netObj, _ := types.ObjectValue(networkingObjectAttrTypes(), map[string]attr.Value{
		"type":                   types.StringValue("unrestricted"),
		"allowed_hosts":          types.ListNull(types.StringType),
		"allow_mcp_servers":      types.BoolNull(),
		"allow_package_managers": types.BoolNull(),
	})
	cfgObj, _ := types.ObjectValue(configObjectAttrTypes(), map[string]attr.Value{
		"type":       types.StringValue("cloud"),
		"packages":   types.ObjectNull(packagesObjectAttrTypes()),
		"networking": netObj,
	})

	out, diags := configToAPI(ctx, cfgObj)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if out.Type != "cloud" {
		t.Errorf("Type = %q", out.Type)
	}
	if out.Networking.Type != "unrestricted" {
		t.Errorf("Networking.Type = %q", out.Networking.Type)
	}
	if out.Packages != nil {
		t.Errorf("Packages should be nil, got %+v", out.Packages)
	}
}
