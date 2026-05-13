package provider

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAgentDataSource_notFound exercises the 404 path in the data source
// Read: an explicit "agent not found" diagnostic should surface to the user.
func TestAccAgentDataSource_notFound(t *testing.T) {
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
data "claude-managed-agents_agent" "missing" {
  id = "agent_DOES_NOT_EXIST"
}`,
				ExpectError: regexp.MustCompile(`(?i)agent not found|no agent with id`),
			},
		},
	})
}

func TestAccAgentDataSource_basic(t *testing.T) {
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
resource "claude-managed-agents_agent" "src" {
  name        = "DS source"
  model       = "claude-opus-4-7"
  description = "data source target"
}

data "claude-managed-agents_agent" "lookup" {
  id = claude-managed-agents_agent.src.id
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.claude-managed-agents_agent.lookup", "id",
						"claude-managed-agents_agent.src", "id",
					),
					resource.TestCheckResourceAttr("data.claude-managed-agents_agent.lookup", "name", "DS source"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_agent.lookup", "description", "data source target"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_agent.lookup", "model", "claude-opus-4-7"),
				),
			},
		},
	})
}
