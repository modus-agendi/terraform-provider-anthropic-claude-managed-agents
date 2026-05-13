package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// No-drift sweep: for every resource type, apply an exhaustive config that
// exercises as many schema attributes as possible, then re-apply the same
// config and assert the plan is empty.
//
// This catches the entire class of "Provider produced inconsistent result"
// and "config drift after apply" bugs in PR CI, without requiring L3 live
// runs. The multiagent `self`-roundtrip bug, the metadata-merge bug, and
// the empty-string description bug would each have surfaced here.
//
// Rules for adding a new resource:
//   - Every nullable attribute must appear in the config at least once.
//   - Every list / map / nested block must appear in the config at least once.
//   - The test must NOT skip in live mode — the whole point is to make fake
//     and real API agree.

// runNoDrift is the shared test driver. buildCfg is invoked after the fake
// API (or live API) is initialized so the provider block in the rendered
// HCL points at the correct base URL.
func runNoDrift(t *testing.T, buildCfg func() string) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := buildCfg()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
			},
			{
				// Re-apply the same config: nothing the API normalized
				// on the response should appear as drift.
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccAgentResource_noDrift(t *testing.T) {
	name := testAgentName("nodrift-agent")
	runNoDrift(t, func() string {
		return fmt.Sprintf(`%s

resource "claude-managed-agents_agent" "reviewer" {
  name  = "%s-reviewer"
  model = "claude-opus-4-7"
}

resource "claude-managed-agents_agent" "subject" {
  name        = %q
  model       = "claude-opus-4-7"
  system      = "You are a thorough code reviewer."
  description = "Sweep test subject."

  metadata = {
    team        = "platform"
    environment = "test"
  }

  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
  ]

  tools = [
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_allow" }
      }
      configs = [
        { name = "web_fetch", enabled = false },
      ]
    },
    {
      type            = "mcp_toolset"
      mcp_server_name = "github"
      default_config = {
        permission_policy = { type = "always_ask" }
      }
    },
    {
      type        = "custom"
      name        = "lookup_user"
      description = "Look up a user by id"
      input_schema = jsonencode({
        type       = "object"
        properties = { user_id = { type = "string" } }
        required   = ["user_id"]
      })
    },
  ]

  skills = [
    { type = "anthropic", skill_id = "xlsx" },
  ]

  multiagent = {
    type = "coordinator"
    agents = [
      { type = "agent", id = claude-managed-agents_agent.reviewer.id },
      { type = "self" },
    ]
  }
}`, providerConfig(), name, name)
	})
}

func TestAccEnvironmentResource_noDrift(t *testing.T) {
	name := testAgentName("nodrift-env")
	runNoDrift(t, func() string {
		return fmt.Sprintf(`%s

# Unrestricted networking + package preinstall. Covers the
# "allow_mcp_servers / allow_package_managers must not be sent when
# unrestricted" bug.
resource "claude-managed-agents_environment" "unrestricted" {
  name = "%s-unrestricted"
  config = {
    type = "cloud"
    packages = {
      pip = ["pandas==2.2.0"]
      npm = ["typescript@5.0.0"]
    }
    networking = { type = "unrestricted" }
  }
}

# Limited networking with allowlist + explicit flags. Covers the
# bare-hostname requirement on allowed_hosts.
resource "claude-managed-agents_environment" "locked_down" {
  name = "%s-locked"
  config = {
    type = "cloud"
    networking = {
      type                   = "limited"
      allowed_hosts          = ["api.example.com", "pypi.org"]
      allow_mcp_servers      = false
      allow_package_managers = false
    }
  }
}`, providerConfig(), name, name)
	})
}

func TestAccVaultResource_noDrift(t *testing.T) {
	name := testAgentName("nodrift-vault")
	runNoDrift(t, func() string {
		return fmt.Sprintf(`%s

resource "claude-managed-agents_vault" "subject" {
  display_name = %q
  metadata = {
    external_user_id = "usr_abc123"
    cohort           = "beta"
  }
}`, providerConfig(), name)
	})
}

func TestAccVaultCredentialResource_noDrift(t *testing.T) {
	name := testAgentName("nodrift-cred")
	runNoDrift(t, func() string {
		return fmt.Sprintf(`%s

resource "claude-managed-agents_vault" "parent" {
  display_name = "%s-vault"
}

# Static bearer credential. WriteOnly token is not stored in state so it
# does not contribute to drift, but display_name + mcp_server_url do.
resource "claude-managed-agents_vault_credential" "bearer" {
  vault_id     = claude-managed-agents_vault.parent.id
  display_name = "%s-bearer"
  auth = {
    type             = "static_bearer"
    mcp_server_url   = "https://mcp.linear.app/mcp"
    token            = "secret-token-not-in-state"
    token_wo_version = 1
  }
}

# OAuth credential with refresh block. Covers the bigger auth variant.
resource "claude-managed-agents_vault_credential" "oauth" {
  vault_id     = claude-managed-agents_vault.parent.id
  display_name = "%s-oauth"
  auth = {
    type                    = "mcp_oauth"
    mcp_server_url          = "https://mcp.slack.com/mcp"
    access_token            = "access-not-in-state"
    access_token_wo_version = 1
    expires_at              = "2099-12-31T23:59:59Z"
    refresh = {
      token_endpoint           = "https://slack.com/api/oauth.v2.access"
      client_id                = "1234567890.0987654321"
      scope                    = "channels:read chat:write"
      refresh_token            = "refresh-not-in-state"
      refresh_token_wo_version = 1
      token_endpoint_auth = {
        type                     = "client_secret_post"
        client_secret            = "secret-not-in-state"
        client_secret_wo_version = 1
      }
    }
  }
}`, providerConfig(), name, name, name)
	})
}

func TestAccMemoryStoreResource_noDrift(t *testing.T) {
	name := testAgentName("nodrift-mem")
	runNoDrift(t, func() string {
		return fmt.Sprintf(`%s

# Default archive-on-destroy.
resource "claude-managed-agents_memory_store" "archived" {
  name        = "%s-archived"
  description = "Preserves audit trail on destroy."
}

# Opt-in hard-delete on destroy.
resource "claude-managed-agents_memory_store" "scratch" {
  name              = "%s-scratch"
  description       = "Hard-deleted on destroy."
  delete_on_destroy = true
}`, providerConfig(), name, name)
	})
}
