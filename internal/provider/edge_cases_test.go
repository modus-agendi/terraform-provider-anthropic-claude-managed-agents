package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccDeploymentResource_invalidCronRejected exercises the schedule cron
// guard: a non-5-field expression (e.g. the "@daily" shortcut the API rejects)
// fails at plan time with a clear error instead of an opaque API rejection.
func TestAccDeploymentResource_invalidCronRejected(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("bad-cron")
	cfg := fmt.Sprintf(`%s

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
    { type = "user.message", content = jsonencode([{ type = "text", text = "hi" }]) },
  ]

  schedule = {
    type       = "cron"
    expression = "@daily"
    timezone   = "UTC"
  }
}`, providerConfig(), name, name, name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, ExpectError: regexp.MustCompile(`(?s)Invalid cron expression.*5-field`)},
		},
	})
}

// TestAccSkillResource_noDrift is the exhaustive no-drift sweep that was missing
// for the skill resource: a multi-file fixture with a non-default
// version_rotation, applied twice with ExpectEmptyPlan on the second apply.
func TestAccSkillResource_noDrift(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "exhaustive no-drift content")
	title := testAgentName("nodrift-skill")
	cfg := skillResourceConfig("s", title, dir, "  version_rotation = 2")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccVaultCredentialResource_readRemovedExternally covers the 404-on-read
// path for vault credentials: when the credential disappears server-side, the
// next refresh removes it from state and the following apply recreates it.
func TestAccVaultCredentialResource_readRemovedExternally(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("external-removal simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-readremoved")
	cfg := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "Linear", "https://mcp.linear.app/mcp", "secret1", 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				// Wipe the vault + credential out of band, then re-apply: the
				// refresh must drop the removed resources from state and the
				// plan must propose recreating them (non-empty plan).
				PreConfig: func() { api.DeleteAllVaults() },
				Config:    cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
			},
		},
	})
}
