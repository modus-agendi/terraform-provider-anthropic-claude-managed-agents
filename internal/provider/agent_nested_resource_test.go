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
	if liveMode() {
		// Real API: "mcp_servers declared but no mcp_toolset in tools
		// references them". Without first-class HCL for `tools`, this
		// constraint can't be satisfied from Terraform alone. Re-enable
		// these once tools lands as HCL.
		t.Skip("mcp_servers requires a matching tools[mcp_toolset] entry; tools is not yet HCL")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("mcp-basic")

	cfg := agentResourceConfig("a", name, `
  mcp_servers = [
    {
      type = "url"
      name = "github"
      url  = "https://mcp.example.com/github"
    },
    {
      type = "url"
      name = "slack"
      url  = "https://mcp.example.com/slack"
    },
  ]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "mcp_servers.#", "2"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "mcp_servers.0.name", "github"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "mcp_servers.1.url", "https://mcp.example.com/slack"),
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

func TestAccAgentResource_mcpServersUpdate(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("mcp_servers requires a matching tools[mcp_toolset] entry; tools is not yet HCL")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("mcp-update")

	step1 := agentResourceConfig("a", name, `
  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
  ]`)
	step2 := agentResourceConfig("a", name, `
  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
    { type = "url", name = "linear", url = "https://mcp.example.com/linear" },
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
	if liveMode() {
		// Real API enriches `self` entries with the parent agent's id,
		// effectively rewriting them into `{type: agent, id: <parent_id>}`.
		// That breaks the plan/apply consistency check because the HCL
		// declares `{type: self}` without an id. Provider can either
		// suppress the id on read (already attempted) or accept the
		// enrichment as drift. Deferred; the fake-API mode still
		// exercises the round-trip mapping for both member types.
		t.Skip("real API rewrites `self` entries with explicit ids; behaviour mismatch with plan")
	}

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
		},
	})
}

func TestAccAgentResource_nestedBlocksAllAtOnce(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		// Uses mcp_servers; see TestAccAgentResource_mcpServersBasic.
		t.Skip("mcp_servers requires a matching tools[mcp_toolset] entry; tools is not yet HCL")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("all-nested")

	cfg := agentResourceConfig("a", name, `
  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
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
