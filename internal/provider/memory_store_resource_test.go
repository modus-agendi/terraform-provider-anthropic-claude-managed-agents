package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func memoryStoreConfig(label, name, extra string) string {
	return providerConfig() + fmt.Sprintf(`
resource "claude-managed-agents_memory_store" %q {
  name = %q
%s
}`, label, name, extra)
}

func TestAccMemoryStoreResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-basic")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: memoryStoreConfig("m", name, `  description = "Per-user prefs"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_memory_store.m", "name", name),
					resource.TestCheckResourceAttr("claude-managed-agents_memory_store.m", "description", "Per-user prefs"),
					resource.TestCheckResourceAttr("claude-managed-agents_memory_store.m", "delete_on_destroy", "false"),
					resource.TestMatchResourceAttr("claude-managed-agents_memory_store.m", "id", regexp.MustCompile(`^memstore_`)),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_memory_store.m",
						tfjsonpath.New("delete_on_destroy"),
						knownvalue.Bool(false),
					),
				},
			},
			{
				Config: memoryStoreConfig("m", name, `  description = "Per-user prefs"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccMemoryStoreResource_updateScalars(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-update")
	renamed := name + "-v2"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: memoryStoreConfig("m", name, `  description = "v1"`)},
			{
				Config: memoryStoreConfig("m", renamed, `  description = "v2"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_memory_store.m",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_memory_store.m", "name", renamed),
					resource.TestCheckResourceAttr("claude-managed-agents_memory_store.m", "description", "v2"),
				),
			},
		},
	})
}

func TestAccMemoryStoreResource_clearDescription(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-clear")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: memoryStoreConfig("m", name, `  description = "will be cleared"`)},
			{
				Config: memoryStoreConfig("m", name, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"claude-managed-agents_memory_store.m",
						tfjsonpath.New("description"),
						knownvalue.Null(),
					),
				},
			},
		},
	})
}

func TestAccMemoryStoreResource_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-import")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: memoryStoreConfig("m", name, "")},
			{
				ResourceName:      "claude-managed-agents_memory_store.m",
				ImportState:       true,
				ImportStateVerify: true,
				// delete_on_destroy is provider-only state with no
				// upstream representation, so it can't be verified by
				// import.
				ImportStateVerifyIgnore: []string{"delete_on_destroy"},
			},
		},
	})
}

func TestAccMemoryStoreResource_readRemovedExternally(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("external-removal simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-readgone")
	cfg := memoryStoreConfig("m", name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllStores() },
				Config:    cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_memory_store.m",
							plancheck.ResourceActionCreate,
						),
					},
				},
			},
		},
	})
}

func TestAccMemoryStoreResource_destroyMissing(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("destroy-while-missing simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-destroymissing")
	cfg := memoryStoreConfig("m", name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllStores() },
				Config:    providerConfig(),
			},
		},
	})
}

// TestAccMemoryStoreResource_deleteOnDestroyTrue verifies the hard-delete
// path: destroy issues DELETE rather than archive when the flag is true.
// We observe the choice by inspecting the fake API state after destroy.
func TestAccMemoryStoreResource_deleteOnDestroyTrue(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("hard-delete behavior is observed via the fake API map")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-harddelete")
	cfg := memoryStoreConfig("m", name, `  delete_on_destroy = true`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config: providerConfig(),
				Check: func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					if len(api.stores) != 0 {
						return fmt.Errorf("expected store map to be empty after hard delete, got %d entries", len(api.stores))
					}
					return nil
				},
			},
		},
	})
}

// TestAccMemoryStoreResource_deleteOnDestroyFalse verifies the archive path:
// destroy archives the store and the fake API map still contains it
// (with ArchivedAt set).
func TestAccMemoryStoreResource_deleteOnDestroyFalse(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("archive behavior is observed via the fake API map")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-archive")
	cfg := memoryStoreConfig("m", name, "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config: providerConfig(),
				Check: func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					if len(api.stores) != 1 {
						return fmt.Errorf("expected store map to still contain the archived store, got %d entries", len(api.stores))
					}
					for _, s := range api.stores {
						if s.ArchivedAt == nil {
							return fmt.Errorf("expected ArchivedAt to be set, got nil")
						}
					}
					return nil
				},
			},
		},
	})
}

func TestAccMemoryStoreResource_invalidEmptyName(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("invalid-config tests run against the fake API only")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "claude-managed-agents_memory_store" "bad" {
  name = ""
}`,
				ExpectError: regexp.MustCompile(`(?i)name.*required|invalid_request_error`),
			},
		},
	})
}

func TestAccMemoryStoreDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("ms-ds")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: memoryStoreConfig("m", name, `  description = "ds"`) + `

data "claude-managed-agents_memory_store" "by_id" {
  id = claude-managed-agents_memory_store.m.id
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.claude-managed-agents_memory_store.by_id", "name",
						"claude-managed-agents_memory_store.m", "name",
					),
					resource.TestCheckResourceAttr(
						"data.claude-managed-agents_memory_store.by_id",
						"description", "ds",
					),
				),
			},
		},
	})
}
