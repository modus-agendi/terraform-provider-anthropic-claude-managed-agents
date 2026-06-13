package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// These tests assert that an explicit empty collection is rejected at plan
// time with a clear, actionable error (issue #79 and its latent class). The
// upstream API normalizes empty → null, which Terraform core cannot represent
// consistently; rejecting at validate time turns the otherwise-cryptic
// "Provider produced inconsistent result after apply" crash into clear
// guidance to omit the attribute. The "omitted" and "populated" cases stay
// valid and are covered by the no-drift tests.

var (
	emptyListErr = regexp.MustCompile(`(?s)Empty list not allowed.*explicit empty list`)
	emptyMapErr  = regexp.MustCompile(`(?s)Empty map not allowed.*explicit empty map`)
)

func runEmptyRejected(t *testing.T, cfg string, wantErr *regexp.Regexp) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	_, cleanup := startFakeAPI(t)
	defer cleanup()
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, ExpectError: wantErr},
		},
	})
}

// TestAccEnvironmentResource_emptyAllowedHostsRejected is the issue #79
// reproduction: limited networking with allowed_hosts = [].
func TestAccEnvironmentResource_emptyAllowedHostsRejected(t *testing.T) {
	name := testAgentName("empty-hosts")
	runEmptyRejected(t, fmt.Sprintf(`%s

resource "claude-managed-agents_environment" "e" {
  name = %q
  config = {
    type = "cloud"
    networking = {
      type          = "limited"
      allowed_hosts = []
    }
  }
}`, providerConfig(), name), emptyListErr)
}

func TestAccAgentResource_emptyMetadataRejected(t *testing.T) {
	name := testAgentName("empty-meta-agent")
	runEmptyRejected(t, fmt.Sprintf(`%s

resource "claude-managed-agents_agent" "a" {
  name     = %q
  model    = "claude-opus-4-7"
  metadata = {}
}`, providerConfig(), name), emptyMapErr)
}

func TestAccVaultResource_emptyMetadataRejected(t *testing.T) {
	name := testAgentName("empty-meta-vault")
	runEmptyRejected(t, fmt.Sprintf(`%s

resource "claude-managed-agents_vault" "v" {
  display_name = %q
  metadata     = {}
}`, providerConfig(), name), emptyMapErr)
}

// deploymentEmptyConfig builds a minimal valid deployment with one collection
// attribute set to its empty form (injected via extra).
func deploymentEmptyConfig(name, extra string) string {
	return fmt.Sprintf(`%s

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
%s
}`, providerConfig(), name, name, name, extra)
}

func TestAccDeploymentResource_emptyVaultIdsRejected(t *testing.T) {
	name := testAgentName("empty-vid-dep")
	runEmptyRejected(t, deploymentEmptyConfig(name, "  vault_ids = []"), emptyListErr)
}

func TestAccDeploymentResource_emptyResourcesRejected(t *testing.T) {
	name := testAgentName("empty-res-dep")
	runEmptyRejected(t, deploymentEmptyConfig(name, "  resources = []"), emptyListErr)
}

func TestAccDeploymentResource_emptyMetadataRejected(t *testing.T) {
	name := testAgentName("empty-meta-dep")
	runEmptyRejected(t, deploymentEmptyConfig(name, "  metadata = {}"), emptyMapErr)
}
