package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAgentResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name  = "Acc Test"
  model = "claude-opus-4-7"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "name", "Acc Test"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "model", "claude-opus-4-7"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "version", "1"),
					resource.TestMatchResourceAttr("claude-managed-agents_agent.a", "id", regexp.MustCompile(`^agent_`)),
					resource.TestCheckResourceAttrSet("claude-managed-agents_agent.a", "created_at"),
				),
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

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name        = "Original"
  model       = "claude-opus-4-7"
  system      = "Be helpful."
  description = "First description."
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "system", "Be helpful."),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "description", "First description."),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "version", "1"),
				),
			},
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name        = "Renamed"
  model       = "claude-opus-4-7"
  system      = "Be very helpful."
  description = "Second description."
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "name", "Renamed"),
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

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name        = "Clear test"
  model       = "claude-opus-4-7"
  system      = "Be helpful."
  description = "To be cleared."
}`,
				Check: resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "description", "To be cleared."),
			},
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name  = "Clear test"
  model = "claude-opus-4-7"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("claude-managed-agents_agent.a", "system"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_agent.a", "description"),
				),
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

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name  = "Metadata test"
  model = "claude-opus-4-7"
  metadata = {
    team  = "platform"
    owner = "alice"
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "metadata.team", "platform"),
					resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "metadata.owner", "alice"),
				),
			},
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name  = "Metadata test"
  model = "claude-opus-4-7"
  metadata = {
    team = "platform"
    cost = "high"
  }
}`,
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

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name  = "Import test"
  model = "claude-opus-4-7"
}`,
			},
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
	if os.Getenv("TF_ACC_LIVE") == "1" {
		t.Skip("drift simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := providerConfig() + `
resource "claude-managed-agents_agent" "a" {
  name  = "Drift Original"
  model = "claude-opus-4-7"
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "name", "Drift Original")},
			{
				PreConfig: func() {
					// Mutate the server-side state to simulate an out-of-band edit.
					for id := range api.agents {
						api.MutateAgent(id, func(a *fakeAgent) { a.Name = "Drifted" })
					}
				},
				Config:             cfg,
				ExpectNonEmptyPlan: false, // refresh + reconciliation should rewrite name to "Drift Original"
				Check: resource.TestCheckResourceAttr("claude-managed-agents_agent.a", "name", "Drift Original"),
			},
		},
	})
}

// stop unused-import lint complaints when no other helpers reference fmt.
var _ = fmt.Sprintf
