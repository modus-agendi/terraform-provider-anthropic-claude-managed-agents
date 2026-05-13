package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func vaultConfig(label, displayName, extra string) string {
	return providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_vault" %q {
  display_name = %q
%s
}`, label, displayName, extra)
}

func TestAccVaultResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("vault-basic")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: vaultConfig("v", name, `
  metadata = {
    external_user_id = "usr_abc"
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_vault.v", "display_name", name),
					resource.TestCheckResourceAttr("claude-managed-agents_vault.v", "metadata.external_user_id", "usr_abc"),
					resource.TestMatchResourceAttr("claude-managed-agents_vault.v", "id", regexp.MustCompile(`^vlt_`)),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_vault.v",
						tfjsonpath.New("delete_on_destroy"),
						knownvalue.Bool(false),
					),
				},
			},
			{
				Config: vaultConfig("v", name, `
  metadata = {
    external_user_id = "usr_abc"
  }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccVaultResource_updateDisplayNameAndMetadata(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("vault-upd")
	rename := name + "-v2"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: vaultConfig("v", name, `
  metadata = {
    team = "platform"
    owner = "alice"
  }`),
			},
			{
				Config: vaultConfig("v", rename, `
  metadata = {
    team = "platform"
    cost = "high"
  }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_vault.v",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_vault.v", "display_name", rename),
					resource.TestCheckResourceAttr("claude-managed-agents_vault.v", "metadata.team", "platform"),
					resource.TestCheckResourceAttr("claude-managed-agents_vault.v", "metadata.cost", "high"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_vault.v", "metadata.owner"),
				),
			},
		},
	})
}

func TestAccVaultResource_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("vault-import")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: vaultConfig("v", name, "")},
			{
				ResourceName:            "claude-managed-agents_vault.v",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_on_destroy"},
			},
		},
	})
}

func TestAccVaultResource_readRemovedExternally(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("external-removal simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("vault-readgone")
	cfg := vaultConfig("v", name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllVaults() },
				Config:    cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_vault.v",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
		},
	})
}

func TestAccVaultResource_destroyMissing(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("destroy-while-missing simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("vault-destroymissing")
	cfg := vaultConfig("v", name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllVaults() },
				Config:    providerConfig(),
			},
		},
	})
}

func TestAccVaultResource_deleteOnDestroyTrue(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("hard-delete behavior is observed via the fake API map")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("vault-harddelete")
	cfg := vaultConfig("v", name, `  delete_on_destroy = true`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config: providerConfig(),
				Check: func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					if len(api.vaults) != 0 {
						return fmt.Errorf("expected vault map empty after hard delete, got %d", len(api.vaults))
					}
					return nil
				},
			},
		},
	})
}

func TestAccVaultResource_invalidEmptyDisplayName(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("invalid-config tests run against the fake API only")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_vault" "bad" {
  display_name = ""
}`,
				ExpectError: regexp.MustCompile(`(?i)display_name.*required|invalid_request_error`),
			},
		},
	})
}

func TestAccVaultDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("vault-ds")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: vaultConfig("v", name, "") + `

data "claude-managed-agents_vault" "by_id" {
  id = claude-managed-agents_vault.v.id
}`,
				Check: resource.TestCheckResourceAttrPair(
					"data.claude-managed-agents_vault.by_id", "display_name",
					"claude-managed-agents_vault.v", "display_name",
				),
			},
		},
	})
}
