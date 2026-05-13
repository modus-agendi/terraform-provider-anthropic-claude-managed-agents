package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccAgentResource_mcpServersBasic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("mcp-basic")

	cfg := agentResourceConfig("a", name, `
  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
    { type = "url", name = "slack",  url = "https://mcp.example.com/slack"  },
  ]
  tools = [
    { type = "mcp_toolset", mcp_server_name = "github" },
    { type = "mcp_toolset", mcp_server_name = "slack"  },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				// Real API enriches tools[*].default_config on read; the
				// non-empty post-apply plan is expected.

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "mcp_servers.#", "2"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "mcp_servers.0.name", "github"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "mcp_servers.1.url", "https://mcp.example.com/slack"),
				),
			},
		},
	})
}

func TestAccAgentResource_mcpServersUpdate(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("mcp-update")

	step1 := agentResourceConfig("a", name, `
  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
  ]
  tools = [
    { type = "mcp_toolset", mcp_server_name = "github" },
  ]`)
	step2 := agentResourceConfig("a", name, `
  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
    { type = "url", name = "linear", url = "https://mcp.example.com/linear" },
  ]
  tools = [
    { type = "mcp_toolset", mcp_server_name = "github" },
    { type = "mcp_toolset", mcp_server_name = "linear" },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{
				Config: step2,

				Check: resource.TestCheckResourceAttr(
					"claude-managed-agents_agent.a", "mcp_servers.#", "2",
				),
			},
		},
	})
}

func TestAccAgentResource_skillsBasic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("skills-basic")

	// Use only Anthropic pre-built skills — `custom` skills require a real
	// skill_id from the user's workspace, which isn't available in CI.
	cfg := agentResourceConfig("a", name, `
  skills = [
    { type = "anthropic", skill_id = "xlsx" },
    { type = "anthropic", skill_id = "pdf" },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "skills.#", "2"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "skills.0.type", "anthropic"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "skills.0.skill_id", "xlsx"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "skills.1.skill_id", "pdf"),
				),
			},
		},
	})
}

func TestAccAgentResource_skillsRemove(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("skills-remove")

	step1 := agentResourceConfig("a", name, `
  skills = [
    { type = "anthropic", skill_id = "xlsx" },
    { type = "anthropic", skill_id = "pdf" },
  ]`)
	step2 := agentResourceConfig("a", name, `
  skills = []`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{
				Config: step2,
				Check: resource.TestCheckResourceAttr(
					"claude-managed-agents_agent.a", "skills.#", "0",
				),
			},
		},
	})
}

func TestAccAgentResource_multiagent(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	// `self` entries are rewritten by the real API to `{type: "agent", id: <parent>}`.
	// The provider detects this in agentFromAPI by comparing the entry id to
	// the parent agent's own id and normalizing back to `{type: "self", id: null}`.
	// Both fake (testutil_test.go normalizeMultiagentSelf) and live API now
	// exercise the same code path.

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("multi")

	cfg := fmt.Sprintf(`%s

resource "claude-managed-agents_agent" "reviewer" {
  name  = "%s-reviewer"
  model = "claude-opus-4-7"
}

resource "claude-managed-agents_agent" "lead" {
  name  = %q
  model = "claude-opus-4-7"

  multiagent = {
    type = "coordinator"
    agents = [
      { type = "agent", id = claude-managed-agents_agent.reviewer.id },
      { type = "self" },
    ]
  }
}`, providerConfig(), name, name)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.lead", "multiagent.type", "coordinator"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.lead", "multiagent.agents.#", "2"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.lead", "multiagent.agents.0.type", "agent"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.lead", "multiagent.agents.1.type", "self"),
				),
			},
			{
				// Re-apply the same config: there must be no drift even
				// though the API rewrote the `self` entry on response.
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

func TestAccAgentResource_nestedBlocksAllAtOnce(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("all-nested")

	cfg := agentResourceConfig("a", name, `
  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
  ]
  tools = [
    { type = "agent_toolset_20260401" },
    { type = "mcp_toolset", mcp_server_name = "github" },
  ]
  skills = [
    { type = "anthropic", skill_id = "xlsx" },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "mcp_servers.0.name", "github"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "skills.0.skill_id", "xlsx"),
				),
			},
		},
	})
}

func TestAccAgentResource_toolsAgentToolset(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("tools-toolset")

	cfg := agentResourceConfig("a", name, `
  tools = [
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_ask" }
      }
      configs = [
        { name = "web_fetch", enabled = false },
      ]
    },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				// The real API enriches default_config.enabled and per-tool
				// permission_policy on read. Without `ignore_changes` in the
				// HCL the user sees a one-time non-empty plan after apply.

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.#", "1"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.0.type", "agent_toolset_20260401"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.0.default_config.permission_policy.type", "always_ask"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.0.configs.0.name", "web_fetch"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.0.configs.0.enabled", "false"),
				),
			},
		},
	})
}

func TestAccAgentResource_toolsCustom(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("tools-custom")

	cfg := agentResourceConfig("a", name, `
  tools = [
    { type = "agent_toolset_20260401" },
    {
      type        = "custom"
      name        = "lookup_user"
      description = "Look up a user by id"
      input_schema = jsonencode({
        type = "object"
        properties = {
          user_id = { type = "string" }
        }
        required = ["user_id"]
      })
    },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.#", "2"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.1.type", "custom"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.1.name", "lookup_user"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "tools.1.description", "Look up a user by id"),
				),
			},
		},
	})
}

// TestAccAgentResource_toolsUpdate exercises the update path through tools:
// replacing the toolset config across two applies.
func TestAccAgentResource_toolsUpdate(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("tools-update")

	step1 := agentResourceConfig("a", name, `
  tools = [
    { type = "agent_toolset_20260401" },
  ]`)
	step2 := agentResourceConfig("a", name, `
  tools = [
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_allow" }
      }
    },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
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
				Check: resource.TestCheckResourceAttr(
					"claude-managed-agents_agent.a",
					"tools.0.default_config.permission_policy.type",
					"always_allow",
				),
			},
		},
	})
}
