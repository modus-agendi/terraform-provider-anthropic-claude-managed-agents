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

// agentResourceConfig formats a `claude-managed-agents_agent` resource block
// with the given name into a full test step config (including provider).
//
// We use printf-style templating rather than raw heredocs so that the name
// can be randomized in live mode without scattering format calls through
// each test.
func agentResourceConfig(label, name, extra string) string {
	return providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_agent" %q {
  name  = %q
  model = "claude-opus-4-7"
%s
}`, label, name, extra)
}

func TestAccAgentResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("basic")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: agentResourceConfig("a", name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "name", name),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "model", "claude-opus-4-7"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "version", "1"),
					resource.TestMatchResourceAttr("claude-managed-agents_agent.a", "id", regexp.MustCompile(`^agent_`)),
					resource.TestCheckResourceAttrSet("claude-managed-agents_agent.a", "created_at"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_agent.a",
						tfjsonpath.New("model"),
						knownvalue.StringExact("claude-opus-4-7"),
					),
					statecheck.ExpectKnownValue(
						"claude-managed-agents_agent.a",
						tfjsonpath.New("version"),
						knownvalue.Int64Exact(1),
					),
				},
			},
			// Re-apply with no config change. plancheck.ExpectEmptyPlan
			// catches regressions where the provider would otherwise plan a
			// spurious update.
			{
				Config: agentResourceConfig("a", name, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccAgentResource_updateScalars(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("update")

	step1 := agentResourceConfig("a", name, `
  system      = "Be helpful."
  description = "First description."`)
	step2 := agentResourceConfig("a", name, `
  system      = "Be very helpful."
  description = "Second description."`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: step1,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "system", "Be helpful."),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "description", "First description."),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "version", "1"),
				),
			},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_agent.a",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "system", "Be very helpful."),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "description", "Second description."),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "version", "2"),
				),
			},
		},
	})
}

func TestAccAgentResource_clearNullable(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("clear")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: agentResourceConfig("a", name, `
  system      = "Be helpful."
  description = "To be cleared."`),
				Check: resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "description", "To be cleared."),
			},
			{
				Config: agentResourceConfig("a", name, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_agent.a",
						tfjsonpath.New("description"),
						knownvalue.Null(),
					),
					statecheck.ExpectKnownValue(
						"claude-managed-agents_agent.a",
						tfjsonpath.New("system"),
						knownvalue.Null(),
					),
				},
			},
		},
	})
}

func TestAccAgentResource_metadataMerge(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("metadata")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: agentResourceConfig("a", name, `
  metadata = {
    team  = "platform"
    owner = "alice"
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "metadata.team", "platform"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "metadata.owner", "alice"),
				),
			},
			{
				Config: agentResourceConfig("a", name, `
  metadata = {
    team = "platform"
    cost = "high"
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "metadata.team", "platform"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "metadata.cost", "high"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_agent.a", "metadata.owner"),
				),
			},
		},
	})
}

func TestAccAgentResource_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("import")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: agentResourceConfig("a", name, "")},
			{
				ResourceName:      "claude-managed-agents_agent.a",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAgentResource_driftRefresh(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("drift simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("drift")
	cfg := agentResourceConfig("a", name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "name", name)},
			{
				PreConfig: func() {
					// Simulate an out-of-band edit on the server.
					for id := range api.agents {
						api.MutateAgent(id, func(a *fakeAgent) { a.Name = "Drifted" })
					}
				},
				Config: cfg,
				Check:  resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "name", name),
			},
		},
	})
}

func TestAccAgentResource_invalidEmptyName(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("invalid-config tests run against the fake API only")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	// Empty name passes Terraform schema validation (name is Required but
	// the empty string is a valid string) and is rejected by the upstream
	// API. The provider surfaces the API error verbatim. When schema-level
	// validation lands, this expectation will tighten.
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "bad" {
  name  = ""
  model = "claude-opus-4-7"
}`,
				ExpectError: regexp.MustCompile(`(?i)name.*required|invalid_request_error`),
			},
		},
	})
}
