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

// deploymentConfig renders an agent + environment + deployment. `extra` is
// injected into the deployment block (after the required initial_events).
func deploymentConfig(name, extra string) string {
	return providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_agent" "a" {
  name  = "%s-agent"
  model = "claude-opus-4-7"
}

resource "claude-managed-agents_environment" "e" {
  name = "%s-env"
  config = {
    type       = "cloud"
    networking = { type = "unrestricted" }
  }
}

resource "claude-managed-agents_deployment" "d" {
  name           = %q
  agent          = claude-managed-agents_agent.a.id
  environment_id = claude-managed-agents_environment.e.id

  initial_events = [
    { type = "user.message", content = jsonencode([{ type = "text", text = "Run." }]) },
  ]
%s
}`, name, name, name, extra)
}

func skipUnlessAcc(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
}

func TestAccDeploymentResource_basic(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-basic")
	cfg := deploymentConfig(name, `
  description = "Nightly digest."
  metadata    = { team = "platform" }
  schedule = {
    type       = "cron"
    expression = "0 3 * * *"
    timezone   = "UTC"
  }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "name", name),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "status", "active"),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "desired_status", "active"),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "schedule.expression", "0 3 * * *"),
					resource.TestMatchResourceAttr("claude-managed-agents_deployment.d", "id", regexp.MustCompile(`^deployment_`)),
					resource.TestCheckResourceAttrSet("claude-managed-agents_deployment.d", "agent_version"),
					resource.TestCheckResourceAttrSet("claude-managed-agents_deployment.d", "created_at"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_deployment.d",
						tfjsonpath.New("status"),
						knownvalue.StringExact("active"),
					),
				},
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccDeploymentResource_updateScalars(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-update")
	step1 := deploymentConfig(name, `
  description = "First."
  metadata    = { team = "platform", tier = "gold" }`)
	step2 := deploymentConfig(name, `
  description = "Second."
  metadata    = { team = "platform" }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: step1,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "description", "First."),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "metadata.tier", "gold"),
				),
			},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("claude-managed-agents_deployment.d", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "description", "Second."),
					// tier key removed via merge semantics.
					resource.TestCheckNoResourceAttr("claude-managed-agents_deployment.d", "metadata.tier"),
				),
			},
		},
	})
}

func TestAccDeploymentResource_clearNullable(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-clear")
	withFields := deploymentConfig(name, `
  description = "Has a schedule and vaults."
  vault_ids   = ["vault_FAKE1"]
  schedule = {
    type       = "cron"
    expression = "0 9 * * 1-5"
    timezone   = "UTC"
  }`)
	cleared := deploymentConfig(name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: withFields,
				Check:  resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "schedule.expression", "0 9 * * 1-5"),
			},
			{
				Config: cleared,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("claude-managed-agents_deployment.d", "description"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_deployment.d", "schedule.%"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_deployment.d", "vault_ids.#"),
				),
			},
			{
				Config: cleared,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccDeploymentResource_pauseResumeCycle drives status via desired_status.
func TestAccDeploymentResource_pauseResumeCycle(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-pause")
	active := deploymentConfig(name, `  desired_status = "active"`)
	paused := deploymentConfig(name, `  desired_status = "paused"`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: active,
				Check:  resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "status", "active"),
			},
			{
				Config: paused,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "status", "paused"),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "paused_reason.type", "manual"),
				),
			},
			{
				Config: active,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "status", "active"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_deployment.d", "paused_reason.type"),
				),
			},
		},
	})
}

// TestAccDeploymentResource_noFlapOnAutoPause is the key desired_status test:
// when the API auto-pauses on an error, `status` flips to paused but
// `desired_status` stays active, and Terraform must NOT plan a resume.
func TestAccDeploymentResource_noFlapOnAutoPause(t *testing.T) {
	skipUnlessAcc(t)
	if liveMode() {
		t.Skip("auto-pause simulation requires the in-process fake API")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-noflap")
	cfg := deploymentConfig(name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "status", "active")},
			{
				PreConfig: func() {
					// Simulate an out-of-band error-pause.
					for id := range api.deployments {
						api.MutateDeployment(id, func(d *fakeDeployment) {
							d.Status = "paused"
							d.PausedReason = map[string]any{
								"type":  "error",
								"error": map[string]any{"type": "vault_not_found_error", "message": "vault gone"},
							}
						})
					}
				},
				Config: cfg,
				// desired_status is unchanged (active), so the plan is empty —
				// Terraform does not fight the API's error-pause.
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "status", "paused"),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "desired_status", "active"),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "paused_reason.error.type", "vault_not_found_error"),
				),
			},
		},
	})
}

// TestAccDeploymentResource_initialEventsRequiresReplace verifies that editing
// initial_events forces a replacement rather than an in-place update.
func TestAccDeploymentResource_initialEventsRequiresReplace(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-replace")
	step1 := deploymentConfig(name, "")
	step2 := providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_agent" "a" {
  name  = "%s-agent"
  model = "claude-opus-4-7"
}

resource "claude-managed-agents_environment" "e" {
  name = "%s-env"
  config = {
    type       = "cloud"
    networking = { type = "unrestricted" }
  }
}

resource "claude-managed-agents_deployment" "d" {
  name           = %q
  agent          = claude-managed-agents_agent.a.id
  environment_id = claude-managed-agents_environment.e.id

  initial_events = [
    { type = "system.message", content = jsonencode([{ type = "text", text = "Privileged." }]) },
    { type = "user.define_outcome", description = "Finish the task", max_iterations = 5, rubric = { type = "text", content = "Be correct." } },
  ]
}`, name, name, name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("claude-managed-agents_deployment.d", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "initial_events.1.max_iterations", "5"),
			},
		},
	})
}

func TestAccDeploymentResource_withGithubResource(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-gh")
	cfg := deploymentConfig(name, `
  resources = [
    {
      type                           = "github_repository"
      url                            = "https://github.com/acme/widgets"
      authorization_token            = "ghp_secret_not_in_state"
      authorization_token_wo_version = 1
      checkout                       = { type = "branch", name = "main" }
    },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "resources.0.url", "https://github.com/acme/widgets"),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "resources.0.checkout.name", "main"),
					// Write-only token is never persisted to state.
					resource.TestCheckNoResourceAttr("claude-managed-agents_deployment.d", "resources.0.authorization_token"),
					resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "resources.0.authorization_token_wo_version", "1"),
				),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccDeploymentResource_import(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-import")
	// No resources block → no write-only-token / wo_version import skew.
	cfg := deploymentConfig(name, `
  description = "Importable."
  schedule = {
    type       = "cron"
    expression = "0 0 * * *"
    timezone   = "UTC"
  }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				ResourceName:      "claude-managed-agents_deployment.d",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccDeploymentResource_driftRefresh(t *testing.T) {
	skipUnlessAcc(t)
	if liveMode() {
		t.Skip("drift simulation requires the in-process fake API")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-drift")
	cfg := deploymentConfig(name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "name", name)},
			{
				PreConfig: func() {
					for id := range api.deployments {
						api.MutateDeployment(id, func(d *fakeDeployment) { d.Name = "Drifted" })
					}
				},
				Config: cfg,
				Check:  resource.TestCheckResourceAttr("claude-managed-agents_deployment.d", "name", name),
			},
		},
	})
}

func TestAccDeploymentResource_destroyMissing(t *testing.T) {
	skipUnlessAcc(t)
	if liveMode() {
		t.Skip("destroy-while-missing simulation requires the in-process fake API")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-destroy-missing")
	cfg := deploymentConfig(name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllDeployments() },
				Config:    providerConfig(),
			},
		},
	})
}

func TestAccDeploymentResource_readRemovedExternally(t *testing.T) {
	skipUnlessAcc(t)
	if liveMode() {
		t.Skip("external-removal simulation requires the in-process fake API")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-read-removed")
	cfg := deploymentConfig(name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllDeployments() },
				Config:    cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("claude-managed-agents_deployment.d", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccDeploymentResource_invalidConfig(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-invalid")
	// Empty initial_events: passes HCL (Required list may be empty) but the
	// client rejects it (1-50 required).
	cfg := providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_agent" "a" {
  name  = "%s-agent"
  model = "claude-opus-4-7"
}

resource "claude-managed-agents_environment" "e" {
  name = "%s-env"
  config = {
    type       = "cloud"
    networking = { type = "unrestricted" }
  }
}

resource "claude-managed-agents_deployment" "d" {
  name           = %q
  agent          = claude-managed-agents_agent.a.id
  environment_id = claude-managed-agents_environment.e.id
  initial_events = []
}`, name, name, name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?i)initial_events|invalid_request_error`),
			},
		},
	})
}
