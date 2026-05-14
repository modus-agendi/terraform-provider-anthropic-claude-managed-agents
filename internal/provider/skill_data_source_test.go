package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSkillDataSource_anthropicPrebuilt reads one of the four prebuilt
// Anthropic skills seeded by the fake API.
func TestAccSkillDataSource_anthropicPrebuilt(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("uses fake-seeded prebuilt skills; live workspace may not expose them")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "claude-managed-agents_skill" "xlsx" {
  skill_id = "xlsx"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.claude-managed-agents_skill.xlsx", "skill_id", "xlsx"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_skill.xlsx", "source", "anthropic"),
					resource.TestCheckResourceAttrSet("data.claude-managed-agents_skill.xlsx", "latest_version"),
				),
			},
		},
	})
}

// TestAccSkillDataSource_customSkill creates a skill via the resource then
// reads it back via the data source.
func TestAccSkillDataSource_customSkill(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	title := testAgentName("skill-ds")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: skillResourceConfig("s", title, dir, "") + `

data "claude-managed-agents_skill" "by_id" {
  skill_id = claude-managed-agents_skill.s.id
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.claude-managed-agents_skill.by_id", "display_title",
						"claude-managed-agents_skill.s", "display_title",
					),
					resource.TestCheckResourceAttr(
						"data.claude-managed-agents_skill.by_id", "source", "custom",
					),
				),
			},
		},
	})
}

func TestAccSkillDataSource_notFound(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("not-found check uses the fake API's deterministic responses")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + fmt.Sprintf(`
data "claude-managed-agents_skill" "missing" {
  skill_id = %q
}`, "skill_DOESNOTEXIST"),
				ExpectError: regexp.MustCompile(`(?i)skill not found|no skill with id`),
			},
		},
	})
}
