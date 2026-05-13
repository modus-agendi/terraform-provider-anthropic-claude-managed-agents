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
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// environmentResourceConfig builds an HCL `claude-managed-agents_environment`
// block with the supplied name and config body.
func environmentResourceConfig(label, name, configBody string) string {
	return providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_environment" %q {
  name = %q
  config = {
%s
  }
}`, label, name, configBody)
}

const unrestrictedCloudConfig = `
    type = "cloud"
    networking = {
      type = "unrestricted"
    }`

func TestAccEnvironmentResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-basic")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: environmentResourceConfig("e", name, unrestrictedCloudConfig),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "name", name),
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "config.type", "cloud"),
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "config.networking.type", "unrestricted"),
					resource.TestMatchResourceAttr("claude-managed-agents_environment.e", "id", regexp.MustCompile(`^env_`)),
					resource.TestCheckResourceAttrSet("claude-managed-agents_environment.e", "created_at"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_environment.e",
						tfjsonpath.New("config").AtMapKey("networking").AtMapKey("type"),
						knownvalue.StringExact("unrestricted"),
					),
				},
			},
			// Re-apply with no config change: plan must be empty.
			{
				Config: environmentResourceConfig("e", name, unrestrictedCloudConfig),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccEnvironmentResource_packagesAndLimitedNetworking(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-limited")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: environmentResourceConfig("e", name, `
    type = "cloud"
    packages = {
      pip = ["pandas==2.2.0", "numpy==2.0.0"]
      npm = ["typescript@5.4.0"]
    }
    networking = {
      type                   = "limited"
      allowed_hosts          = ["https://pypi.org", "https://registry.npmjs.org"]
      allow_mcp_servers      = false
      allow_package_managers = true
    }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "config.networking.type", "limited"),
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "config.networking.allowed_hosts.#", "2"),
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "config.networking.allow_package_managers", "true"),
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "config.packages.pip.#", "2"),
					resource.TestCheckResourceAttr("claude-managed-agents_environment.e", "config.packages.npm.0", "typescript@5.4.0"),
				),
			},
		},
	})
}

// TestAccEnvironmentResource_requiresReplaceOnNameChange asserts the
// RequiresReplace planmodifier on `name`: Terraform must plan a replace,
// not an in-place update.
func TestAccEnvironmentResource_requiresReplaceOnNameChange(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	first := testAgentName("env-replace-1")
	second := testAgentName("env-replace-2")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: environmentResourceConfig("e", first, unrestrictedCloudConfig)},
			{
				Config: environmentResourceConfig("e", second, unrestrictedCloudConfig),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_environment.e",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
			},
		},
	})
}

// TestAccEnvironmentResource_requiresReplaceOnConfigChange exercises the
// objectplanmodifier.RequiresReplace on `config`.
func TestAccEnvironmentResource_requiresReplaceOnConfigChange(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-cfgreplace")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: environmentResourceConfig("e", name, unrestrictedCloudConfig)},
			{
				Config: environmentResourceConfig("e", name, `
    type = "cloud"
    networking = {
      type          = "limited"
      allowed_hosts = ["https://pypi.org"]
    }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_environment.e",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
			},
		},
	})
}

func TestAccEnvironmentResource_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-import")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: environmentResourceConfig("e", name, unrestrictedCloudConfig)},
			{
				ResourceName:      "claude-managed-agents_environment.e",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccEnvironmentResource_readRemovedExternally exercises the Read 404
// path: the env vanishes server-side, Read must call RemoveResource so the
// next plan re-creates it.
func TestAccEnvironmentResource_readRemovedExternally(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("external-removal simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-readgone")
	cfg := environmentResourceConfig("e", name, unrestrictedCloudConfig)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllEnvs() },
				Config:    cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_environment.e",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
		},
	})
}

// TestAccEnvironmentResource_destroyMissing covers the Delete code path
// when the environment has already been deleted server-side.
func TestAccEnvironmentResource_destroyMissing(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("destroy-while-missing simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-destroymissing")
	cfg := environmentResourceConfig("e", name, unrestrictedCloudConfig)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllEnvs() },
				Config:    providerConfig(),
			},
		},
	})
}

// TestAccEnvironmentResource_destroyArchiveFallback exercises the 409 →
// archive fallback path in Delete. The fake API is configured to block
// DELETE for the active env id; Terraform destroy must succeed by archiving.
func TestAccEnvironmentResource_destroyArchiveFallback(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("409-on-delete simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-409fallback")
	cfg := environmentResourceConfig("e", name, unrestrictedCloudConfig)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() {
					// Block DELETE on every existing env so destroy must
					// fall back to archive.
					for id := range api.envs {
						api.BlockEnvDelete(id)
					}
				},
				Config: providerConfig(),
			},
		},
	})
}

// TestAccEnvironmentResource_invalidMissingNetworkingType verifies that the
// upstream "config.networking.type is required" validation surfaces as a
// Terraform error.
func TestAccEnvironmentResource_invalidMissingNetworkingType(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("invalid-config tests run against the fake API only")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-badnet")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_environment" "bad" {
  name = %q
  config = {
    type = "cloud"
    networking = {
      type = ""
    }
  }
}`, name),
				ExpectError: regexp.MustCompile(`(?i)networking\.type|invalid_request_error`),
			},
		},
	})
}

// TestAccEnvironmentDataSource_basic exercises the environment data source.
func TestAccEnvironmentDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("env-ds")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: environmentResourceConfig("e", name, unrestrictedCloudConfig) + `

data "claude-managed-agents_environment" "by_id" {
  id = claude-managed-agents_environment.e.id
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.claude-managed-agents_environment.by_id", "name",
						"claude-managed-agents_environment.e", "name",
					),
					resource.TestCheckResourceAttr(
						"data.claude-managed-agents_environment.by_id",
						"config.networking.type", "unrestricted",
					),
				),
			},
		},
	})
}
