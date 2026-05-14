package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

// writeSkillFixture lays down a minimal skill directory under t.TempDir():
//
//	SKILL.md
//	notes.md
//
// Each fixture file gets the same body string so callers can produce a
// known content hash by comparing dirs side-by-side.
func writeSkillFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: tf-acc-skill\n---\n"+body), 0o600); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\n"+body), 0o600); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
	return dir
}

// writeSkillFixtureNoEntrypoint creates a dir missing SKILL.md to drive the
// validation error path.
func writeSkillFixtureNoEntrypoint(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("not a skill"), 0o600); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
	return dir
}

// writeSkillFixtureOversize creates a >30 MB fixture so the size guard fires.
func writeSkillFixtureOversize(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: too-big\n---\n"), 0o600); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	big := make([]byte, 31*1024*1024)
	if _, err := rand.Read(big); err != nil {
		t.Fatalf("rand: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "big.bin"), big, 0o600); err != nil {
		t.Fatalf("write big.bin: %v", err)
	}
	return dir
}

// skillResourceConfig builds an HCL block referencing a fixture directory.
// HCL strings must escape backslashes on Windows; we don't currently test
// there, so a raw %q is sufficient.
func skillResourceConfig(label, title, sourceDir, extra string) string {
	return providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_skill" %q {
  display_title = %q
  source_dir    = %q
%s
}`, label, title, sourceDir, extra)
}

func TestAccSkillResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "v1 content")
	title := testAgentName("skill-basic")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: skillResourceConfig("s", title, dir, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_skill.s", "display_title", title),
					resource.TestCheckResourceAttr("claude-managed-agents_skill.s", "source", "custom"),
					resource.TestMatchResourceAttr("claude-managed-agents_skill.s", "id", regexp.MustCompile(`^skill_`)),
					resource.TestCheckResourceAttrSet("claude-managed-agents_skill.s", "latest_version"),
					resource.TestMatchResourceAttr("claude-managed-agents_skill.s", "content_hash", regexp.MustCompile(`^[a-f0-9]{64}$`)),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_skill.s",
						tfjsonpath.New("source"),
						knownvalue.StringExact("custom"),
					),
				},
			},
			{
				Config: skillResourceConfig("s", title, dir, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccSkillResource_versionBumpOnContentChange(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "v1 content")
	title := testAgentName("skill-bump")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				PreConfig: func() {
					// Mutate fixture between steps to force a new content_hash.
					if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: tf-acc-skill\n---\nv2 content"), 0o600); err != nil {
						t.Fatalf("rewrite SKILL.md: %v", err)
					}
				},
				Config: skillResourceConfig("s", title, dir, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_skill.s",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.TestCheckResourceAttrSet("claude-managed-agents_skill.s", "latest_version"),
			},
		},
	})
}

func TestAccSkillResource_versionBumpOnRotation(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	title := testAgentName("skill-rot")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				Config: skillResourceConfig("s", title, dir, `  version_rotation = 1`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_skill.s",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

func TestAccSkillResource_addFile(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	title := testAgentName("skill-addfile")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				PreConfig: func() {
					if err := os.WriteFile(filepath.Join(dir, "extra.md"), []byte("# Extra"), 0o600); err != nil {
						t.Fatalf("write extra.md: %v", err)
					}
				},
				Config: skillResourceConfig("s", title, dir, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_skill.s",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

func TestAccSkillResource_renameFile(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	title := testAgentName("skill-rename")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				PreConfig: func() {
					if err := os.Rename(filepath.Join(dir, "notes.md"), filepath.Join(dir, "renamed.md")); err != nil {
						t.Fatalf("rename: %v", err)
					}
				},
				Config: skillResourceConfig("s", title, dir, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_skill.s",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

// TestAccSkillResource_destroyCascade asserts that destroy walks every
// version then deletes the skill itself. After two updates, three versions
// exist; after destroy, the fake API has no record of the skill or its
// versions.
func TestAccSkillResource_destroyCascade(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("version-cascade observation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "v1")
	title := testAgentName("skill-cascade")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				PreConfig: func() {
					if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v2"), 0o600); err != nil {
						t.Fatalf("rewrite SKILL.md: %v", err)
					}
				},
				Config: skillResourceConfig("s", title, dir, ""),
			},
			{
				PreConfig: func() {
					if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v3"), 0o600); err != nil {
						t.Fatalf("rewrite SKILL.md: %v", err)
					}
				},
				Config: skillResourceConfig("s", title, dir, ""),
				Check: func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					// At this point the resource has 3 versions stored.
					var customCount int
					for _, s := range api.skills {
						if s.Source == "custom" {
							customCount++
							if got := len(api.skillVersions[s.ID]); got != 3 {
								return fmt.Errorf("expected 3 versions before destroy, got %d", got)
							}
						}
					}
					if customCount != 1 {
						return fmt.Errorf("expected 1 custom skill, got %d", customCount)
					}
					return nil
				},
			},
			{
				Config: providerConfig(),
				Check: func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					for _, s := range api.skills {
						if s.Source == "custom" {
							return fmt.Errorf("expected no custom skills after destroy, found %s", s.ID)
						}
					}
					return nil
				},
			},
		},
	})
}

func TestAccSkillResource_missingSkillMd(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("invalid-config tests run against the fake API only")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixtureNoEntrypoint(t)
	title := testAgentName("skill-noskillmd")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      skillResourceConfig("s", title, dir, ""),
				ExpectError: regexp.MustCompile(`SKILL\.md is required`),
			},
		},
	})
}

func TestAccSkillResource_oversize(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("oversize fixture would consume real quota in live mode")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixtureOversize(t)
	title := testAgentName("skill-toobig")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      skillResourceConfig("s", title, dir, ""),
				ExpectError: regexp.MustCompile(`exceeds 30 MB`),
			},
		},
	})
}

func TestAccSkillResource_invalidSourceDir(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("invalid-config tests run against the fake API only")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	title := testAgentName("skill-baddir")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      skillResourceConfig("s", title, "/this/path/does/not/exist/anywhere", ""),
				ExpectError: regexp.MustCompile(`(?i)stat source_dir|no such file`),
			},
		},
	})
}

func TestAccSkillResource_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	title := testAgentName("skill-import")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				ResourceName: "claude-managed-agents_skill.s",
				ImportState:  true,
				// source_dir / content_hash / version_rotation are local
				// truths that the API does not return. They round-trip from
				// the HCL after import.
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"source_dir",
					"content_hash",
					"version_rotation",
				},
			},
		},
	})
}

// TestAccSkillResource_driftRefresh exercises the out-of-band-version case:
// a curl uploads a new version while local files are unchanged. The next
// refresh updates `latest_version` in state without proposing a new
// upload (because content_hash still matches what was applied).
func TestAccSkillResource_driftRefresh(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("out-of-band-version simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	title := testAgentName("skill-drift")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				PreConfig: func() {
					// Find the custom skill and inject a fake new version.
					api.mu.Lock()
					defer api.mu.Unlock()
					for id, s := range api.skills {
						if s.Source != "custom" {
							continue
						}
						v := &fakeSkillVersion{
							Type: "skill_version", SkillID: id,
							Version: "9999999999", CreatedAt: "2026-01-01T00:00:00Z",
						}
						api.skillVersions[id] = append(api.skillVersions[id], v)
						s.LatestVersion = v.Version
					}
				},
				Config: skillResourceConfig("s", title, dir, ""),
				// content_hash is unchanged → no upload. latest_version is
				// refreshed via Read → state shows the new value. Plan is
				// non-empty because latest_version drifted.
				Check: func(state *terraform.State) error {
					rs := state.RootModule().Resources["claude-managed-agents_skill.s"]
					if rs == nil {
						return fmt.Errorf("resource not in state")
					}
					if rs.Primary.Attributes["latest_version"] != "9999999999" {
						return fmt.Errorf("expected latest_version to refresh to 9999999999, got %q", rs.Primary.Attributes["latest_version"])
					}
					return nil
				},
			},
		},
	})
}

func TestAccSkillResource_displayTitleForcesReplace(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	first := testAgentName("skill-rename-1")
	second := testAgentName("skill-rename-2")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", first, dir, "")},
			{
				Config: skillResourceConfig("s", second, dir, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_skill.s",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
			},
		},
	})
}

// TestAccSkillResource_destroyMissing covers the Delete path when the
// skill has already been removed server-side.
func TestAccSkillResource_destroyMissing(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("destroy-while-missing simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	dir := writeSkillFixture(t, "content")
	title := testAgentName("skill-destroymissing")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: skillResourceConfig("s", title, dir, "")},
			{
				PreConfig: func() { api.DeleteAllSkills() },
				Config:    providerConfig(),
			},
		},
	})
}

// TestAccSkillResource_clientDirectDelete asserts that the client interaction
// for cascade-delete cleans up the skill list in the fake API. Exercises
// the same path as TestAccSkillResource_destroyCascade but uses the client
// directly to keep coverage of the delete path tight.
func TestAccSkillResource_clientDirectDelete(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("uses the in-process fake API and the client directly")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	// Build a client pointing at the fake API.
	c, err := client.New(client.Config{
		APIKey:  "sk-test",
		BaseURL: os.Getenv("CLAUDE_MANAGED_AGENTS_BASE_URL"),
	})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}

	// Create a skill and a couple of extra versions.
	ctx := context.Background()
	created, err := c.CreateSkill(ctx, client.SkillCreateRequest{
		DisplayTitle: testResourcePrefix + "direct",
		Files: []client.SkillFile{
			{Path: "SKILL.md", Content: []byte("---\nname: x\n---\n")},
		},
	})
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	if _, err := c.CreateSkillVersion(ctx, created.ID, client.SkillVersionCreateRequest{
		Files: []client.SkillFile{{Path: "SKILL.md", Content: []byte("v2")}},
	}); err != nil {
		t.Fatalf("CreateSkillVersion: %v", err)
	}

	// DeleteSkill while versions exist must fail with the API's
	// "delete versions first" error.
	if err := c.DeleteSkill(ctx, created.ID); err == nil {
		t.Fatalf("expected DeleteSkill to fail while versions exist")
	}

	// Now cascade: list versions, delete each, then delete the skill.
	versions, err := c.ListSkillVersions(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListSkillVersions: %v", err)
	}
	for _, v := range versions.Data {
		if err := c.DeleteSkillVersion(ctx, created.ID, v.Version); err != nil {
			t.Fatalf("DeleteSkillVersion %s: %v", v.Version, err)
		}
	}
	if err := c.DeleteSkill(ctx, created.ID); err != nil {
		t.Fatalf("DeleteSkill after cascade: %v", err)
	}

	if got := api.SnapshotSkill(created.ID); got != nil {
		t.Fatalf("expected skill to be gone, got %+v", got)
	}
	// Title-prefix sanity: the prefix we used matches the sweeper convention.
	if !strings.HasPrefix(testResourcePrefix+"direct", testResourcePrefix) {
		t.Fatalf("test resource prefix invariant broken")
	}
}
