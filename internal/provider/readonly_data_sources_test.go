package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAgentVersionDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("seeded version history requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("v-lookup")

	api.SeedAgentVersion("agent_FAKE0001", map[string]any{
		"type":        "agent_version",
		"agent_id":    "agent_FAKE0001",
		"version":     1,
		"name":        name,
		"model":       map[string]string{"id": "claude-opus-4-7"},
		"system":      nil,
		"description": nil,
		"metadata":    map[string]string{},
		"created_at":  "2026-05-13T00:00:00Z",
		"updated_at":  "2026-05-13T00:00:00Z",
	})
	api.SeedAgentVersion("agent_FAKE0001", map[string]any{
		"type":        "agent_version",
		"agent_id":    "agent_FAKE0001",
		"version":     2,
		"name":        name + "-v2",
		"model":       map[string]string{"id": "claude-opus-4-7"},
		"system":      "Be helpful.",
		"description": nil,
		"metadata":    map[string]string{},
		"created_at":  "2026-05-13T00:00:00Z",
		"updated_at":  "2026-05-13T00:00:00Z",
	})

	cfg := providerConfig() + fmt.Sprintf(`
# A real agent isn't needed; we seeded the version directly into the fake.
data "claude-managed-agents_agent_version" "v2" {
  agent_id = %q
  version  = 2
}`, "agent_FAKE0001")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.claude-managed-agents_agent_version.v2", "name", name+"-v2"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_agent_version.v2", "version", "2"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_agent_version.v2", "system", "Be helpful."),
				),
			},
		},
	})
}

func TestAccAgentVersionDataSource_NotFound(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("not-found test runs against fake API")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := providerConfig() + `
data "claude-managed-agents_agent_version" "missing" {
  agent_id = "agent_never_existed"
  version  = 99
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?i)not found|no version`),
			},
		},
	})
}

func TestAccFileDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("seeded file metadata requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	api.SeedFile(&fakeFile{
		ID:        "file_xyz",
		Type:      "file",
		Filename:  "report.csv",
		SizeBytes: 4096,
		MimeType:  "text/csv",
		ScopeID:   "sesn_abc",
		CreatedAt: "2026-05-13T00:00:00Z",
	})

	cfg := providerConfig() + `
data "claude-managed-agents_file" "r" {
  id = "file_xyz"
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.claude-managed-agents_file.r", "filename", "report.csv"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_file.r", "size_bytes", "4096"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_file.r", "mime_type", "text/csv"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_file.r", "scope_id", "sesn_abc"),
				),
			},
		},
	})
}

func TestAccFileDataSource_NotFound(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("not-found test runs against fake API")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := providerConfig() + `
data "claude-managed-agents_file" "missing" {
  id = "file_never"
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?i)not found|no file`),
			},
		},
	})
}
