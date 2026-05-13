package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func credStaticBearerConfig(label, vaultLabel, displayName, mcpURL, token string, woVersion int) string {
	return fmt.Sprintf(`
resource "claude-managed-agents_vault_credential" %q {
  vault_id     = claude-managed-agents_vault.%s.id
  display_name = %q

  auth = {
    type             = "static_bearer"
    mcp_server_url   = %q
    token            = %q
    token_wo_version = %d
  }
}`, label, vaultLabel, displayName, mcpURL, token, woVersion)
}

func TestAccVaultCredentialResource_staticBearerBasic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-vault")

	cfg := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "Linear API key", "https://mcp.linear.app/mcp", "lin_api_secret", 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Real API uses `vcrd_` prefix; the fake API still uses `cred_`.
					resource.TestMatchResourceAttr("claude-managed-agents_vault_credential.c", "id", regexp.MustCompile(`^(cred|vcrd)_`)),
					resource.TestCheckResourceAttr("claude-managed-agents_vault_credential.c", "display_name", "Linear API key"),
					resource.TestCheckResourceAttr("claude-managed-agents_vault_credential.c", "auth.type", "static_bearer"),
					resource.TestCheckResourceAttr("claude-managed-agents_vault_credential.c", "auth.mcp_server_url", "https://mcp.linear.app/mcp"),
					// `auth.token` is WriteOnly: must not appear in state.
					resource.TestCheckNoResourceAttr("claude-managed-agents_vault_credential.c", "auth.token"),
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

func TestAccVaultCredentialResource_displayNameUpdate(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-vault-rename")

	step1 := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "Original", "https://mcp.example.com/mcp", "secret1", 1)
	step2 := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "Renamed", "https://mcp.example.com/mcp", "secret1", 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_vault_credential.c",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.TestCheckResourceAttr(
					"claude-managed-agents_vault_credential.c", "display_name", "Renamed",
				),
			},
		},
	})
}

func TestAccVaultCredentialResource_requiresReplaceOnMcpUrlChange(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-vault-replace")

	step1 := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "X", "https://mcp.one.com/mcp", "secret1", 1)
	step2 := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "X", "https://mcp.two.com/mcp", "secret1", 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_vault_credential.c",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
			},
		},
	})
}

func TestAccVaultCredentialResource_secretRotation(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-vault-rotate")

	step1 := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "X", "https://mcp.r.com/mcp", "old_secret", 1)
	step2 := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "X", "https://mcp.r.com/mcp", "new_secret", 2)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"claude-managed-agents_vault_credential.c",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

func TestAccVaultCredentialResource_duplicateMcpServerURL(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("duplicate-URL behavior tested via the fake API")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-vault-dup")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: vaultConfig("v", vaultName, "") +
					credStaticBearerConfig("a", "v", "First", "https://mcp.dup.com/mcp", "secret1", 1) +
					credStaticBearerConfig("b", "v", "Duplicate", "https://mcp.dup.com/mcp", "secret2", 1),
				ExpectError: regexp.MustCompile(`(?i)duplicate|conflict`),
			},
		},
	})
}

func TestAccVaultCredentialResource_oauthBasic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-vault-oauth")
	cfg := vaultConfig("v", vaultName, "") + `
resource "claude-managed-agents_vault_credential" "c" {
  vault_id     = claude-managed-agents_vault.v.id
  display_name = "Alice's Slack"

  auth = {
    type                    = "mcp_oauth"
    mcp_server_url          = "https://mcp.slack.com/mcp"
    access_token            = "xoxp-secret"
    access_token_wo_version = 1
    expires_at              = "2099-12-31T23:59:59Z"

    refresh = {
      token_endpoint           = "https://slack.com/api/oauth.v2.access"
      client_id                = "1234567890.0987654321"
      scope                    = "channels:read chat:write"
      refresh_token            = "xoxe-secret"
      refresh_token_wo_version = 1

      token_endpoint_auth = {
        type                     = "client_secret_post"
        client_secret            = "abc123"
        client_secret_wo_version = 1
      }
    }
  }
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude-managed-agents_vault_credential.c", "auth.type", "mcp_oauth"),
					resource.TestCheckResourceAttr("claude-managed-agents_vault_credential.c", "auth.refresh.token_endpoint", "https://slack.com/api/oauth.v2.access"),
					resource.TestCheckResourceAttr("claude-managed-agents_vault_credential.c", "auth.refresh.token_endpoint_auth.type", "client_secret_post"),
					// Secrets must not be in state.
					resource.TestCheckNoResourceAttr("claude-managed-agents_vault_credential.c", "auth.access_token"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_vault_credential.c", "auth.refresh.refresh_token"),
					resource.TestCheckNoResourceAttr("claude-managed-agents_vault_credential.c", "auth.refresh.token_endpoint_auth.client_secret"),
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

func TestAccVaultCredentialResource_destroyMissing(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	if liveMode() {
		t.Skip("destroy-while-missing simulation requires the in-process fake API")
	}

	api, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-destroymissing")
	cfg := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "x", "https://mcp.dm.com/mcp", "secret1", 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() { api.DeleteAllVaults() },
				Config:    providerConfig(),
			},
		},
	})
}

func TestAccVaultCredentialDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vaultName := testAgentName("cred-ds")
	cfg := vaultConfig("v", vaultName, "") +
		credStaticBearerConfig("c", "v", "x", "https://mcp.ds.com/mcp", "secret1", 1) + `

data "claude-managed-agents_vault_credential" "by_id" {
  vault_id = claude-managed-agents_vault.v.id
  id       = claude-managed-agents_vault_credential.c.id
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.claude-managed-agents_vault_credential.by_id", "auth.mcp_server_url",
						"claude-managed-agents_vault_credential.c", "auth.mcp_server_url",
					),
					resource.TestCheckResourceAttr(
						"data.claude-managed-agents_vault_credential.by_id",
						"auth.type", "static_bearer",
					),
				),
			},
		},
	})
}
