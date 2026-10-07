package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const envCredAddr = "claude-managed-agents_vault_credential.c"

// credEnvVarConfig renders an environment_variable credential. hosts is
// rendered verbatim as the allowed_hosts HCL list; injection is rendered
// verbatim as the injection_location attribute ("" omits it).
func credEnvVarConfig(vaultLabel, secretName, secret string, woVersion int, hosts, injection string) string {
	injectionLine := ""
	if injection != "" {
		injectionLine = "    injection_location = " + injection
	}
	return fmt.Sprintf(`
resource "claude-managed-agents_vault_credential" "c" {
  vault_id     = claude-managed-agents_vault.%s.id
  display_name = "Datadog API key"

  auth = {
    type                    = "environment_variable"
    secret_name             = %q
    secret_value            = %q
    secret_value_wo_version = %d
    networking = {
      type          = "limited"
      allowed_hosts = %s
    }
%s
  }
}`, vaultLabel, secretName, secret, woVersion, hosts, injectionLine)
}

func requireAcc(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
}

// checkFakeSecret asserts the fake API last received `want` as the
// credential's secret_value.
func checkFakeSecret(api *fakeAPI, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[envCredAddr]
		if !ok {
			return fmt.Errorf("%s not in state", envCredAddr)
		}
		if got := api.CredSecret(rs.Primary.ID); got != want {
			return fmt.Errorf("fake API secret_value = %q, want %q", got, want)
		}
		return nil
	}
}

func TestAccVaultCredentialResource_envVarBasic(t *testing.T) {
	requireAcc(t)
	if liveMode() {
		t.Skip("asserts on the secret the in-process fake API received")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := vaultConfig("v", testAgentName("cred-env"), "") +
		credEnvVarConfig("v", "DIAGNOSTIC_DD_API_KEY", "dd-secret-1", 1, `["api.datadoghq.com"]`, `{ header = true }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(envCredAddr, "id", regexp.MustCompile(`^(cred|vcrd)_`)),
					resource.TestCheckResourceAttr(envCredAddr, "auth.type", "environment_variable"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.secret_name", "DIAGNOSTIC_DD_API_KEY"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.secret_value_wo_version", "1"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.networking.type", "limited"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.networking.allowed_hosts.#", "1"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.networking.allowed_hosts.0", "api.datadoghq.com"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.injection_location.header", "true"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.injection_location.body", "false"),
					resource.TestCheckNoResourceAttr(envCredAddr, "auth.mcp_server_url"),
					// secret_value is WriteOnly: must not appear in state.
					resource.TestCheckNoResourceAttr(envCredAddr, "auth.secret_value"),
					checkFakeSecret(api, "dd-secret-1"),
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

func TestAccVaultCredentialResource_envVarDefaultInjectionAndUnrestricted(t *testing.T) {
	requireAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := vaultConfig("v", testAgentName("cred-env-default"), "") + `
resource "claude-managed-agents_vault_credential" "c" {
  vault_id     = claude-managed-agents_vault.v.id
  display_name = "unrestricted"
  auth = {
    type                    = "environment_variable"
    secret_name             = "SOME_TOKEN"
    secret_value            = "s"
    secret_value_wo_version = 1
    networking              = { type = "unrestricted" }
  }
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(envCredAddr, "auth.networking.type", "unrestricted"),
					resource.TestCheckNoResourceAttr(envCredAddr, "auth.networking.allowed_hosts"),
					// API-defaulted injection_location is recorded in state.
					resource.TestCheckResourceAttrSet(envCredAddr, "auth.injection_location.header"),
					resource.TestCheckResourceAttrSet(envCredAddr, "auth.injection_location.body"),
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

func TestAccVaultCredentialResource_envVarRotation(t *testing.T) {
	requireAcc(t)
	if liveMode() {
		t.Skip("asserts on the secret the in-process fake API received")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	vault := vaultConfig("v", testAgentName("cred-env-rotate"), "")
	hosts := `["api.datadoghq.com"]`
	step1 := vault + credEnvVarConfig("v", "DD_API_KEY", "old", 1, hosts, `{ header = true }`)
	// New secret, same wo_version: write-only values are not diffed, so no
	// update is planned and the old secret stays.
	step2 := vault + credEnvVarConfig("v", "DD_API_KEY", "new", 1, hosts, `{ header = true }`)
	// Bumping the counter re-sends the secret in place.
	step3 := vault + credEnvVarConfig("v", "DD_API_KEY", "new", 2, hosts, `{ header = true }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1, Check: checkFakeSecret(api, "old")},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: checkFakeSecret(api, "old"),
			},
			{
				Config: step3,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(envCredAddr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					checkFakeSecret(api, "new"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.secret_value_wo_version", "2"),
				),
			},
		},
	})
}

func TestAccVaultCredentialResource_envVarNetworkingAndInjectionUpdateInPlace(t *testing.T) {
	requireAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vault := vaultConfig("v", testAgentName("cred-env-update"), "")
	step1 := vault + credEnvVarConfig("v", "DD_API_KEY", "s", 1, `["api.datadoghq.com"]`, `{ header = true }`)
	step2 := vault + credEnvVarConfig("v", "DD_API_KEY", "s", 1, `["api.datadoghq.com", "*.datadoghq.eu"]`, `{ header = true, body = true }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(envCredAddr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(envCredAddr, "auth.networking.allowed_hosts.#", "2"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.networking.allowed_hosts.1", "*.datadoghq.eu"),
					resource.TestCheckResourceAttr(envCredAddr, "auth.injection_location.body", "true"),
				),
			},
			{
				Config: step2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccVaultCredentialResource_envVarRequiresReplaceOnSecretNameChange(t *testing.T) {
	requireAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vault := vaultConfig("v", testAgentName("cred-env-replace"), "")
	hosts := `["api.datadoghq.com"]`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: vault + credEnvVarConfig("v", "DD_API_KEY", "s", 1, hosts, "")},
			{
				Config: vault + credEnvVarConfig("v", "DD_APP_KEY", "s", 1, hosts, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(envCredAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttr(envCredAddr, "auth.secret_name", "DD_APP_KEY"),
			},
		},
	})
}

func TestAccVaultCredentialResource_envVarImport(t *testing.T) {
	requireAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := vaultConfig("v", testAgentName("cred-env-import"), "") +
		credEnvVarConfig("v", "DD_API_KEY", "s", 1, `["api.datadoghq.com"]`, `{ header = true }`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				ResourceName: envCredAddr,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[envCredAddr]
					if !ok {
						return "", fmt.Errorf("%s not in state", envCredAddr)
					}
					return rs.Primary.Attributes["vault_id"] + ":" + rs.Primary.ID, nil
				},
				ImportStateVerify: true,
				// The rotation counter is config-only; the API cannot
				// return it (or the secret) on import.
				ImportStateVerifyIgnore: []string{"auth.secret_value_wo_version"},
			},
		},
	})
}

func TestAccVaultCredentialDataSource_envVar(t *testing.T) {
	requireAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	cfg := vaultConfig("v", testAgentName("cred-env-ds"), "") +
		credEnvVarConfig("v", "DD_API_KEY", "s", 1, `["api.datadoghq.com"]`, `{ header = true }`) + `

data "claude-managed-agents_vault_credential" "by_id" {
  vault_id = claude-managed-agents_vault.v.id
  id       = claude-managed-agents_vault_credential.c.id
}`
	ds := "data.claude-managed-agents_vault_credential.by_id"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "auth.type", "environment_variable"),
					resource.TestCheckResourceAttr(ds, "auth.secret_name", "DD_API_KEY"),
					resource.TestCheckResourceAttr(ds, "auth.networking.type", "limited"),
					resource.TestCheckResourceAttr(ds, "auth.networking.allowed_hosts.0", "api.datadoghq.com"),
					resource.TestCheckResourceAttr(ds, "auth.injection_location.header", "true"),
					resource.TestCheckResourceAttr(ds, "auth.injection_location.body", "false"),
					resource.TestCheckNoResourceAttr(ds, "auth.mcp_server_url"),
				),
			},
		},
	})
}

func TestAccVaultCredentialResource_authValidation(t *testing.T) {
	requireAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	vault := vaultConfig("v", testAgentName("cred-validate"), "")
	cred := func(auth string) string {
		return vault + fmt.Sprintf(`
resource "claude-managed-agents_vault_credential" "c" {
  vault_id     = claude-managed-agents_vault.v.id
  display_name = "x"
  auth = {
%s
  }
}`, auth)
	}
	envBase := `
    type         = "environment_variable"
    secret_name  = "K"
    secret_value = "s"
`
	limited := `    networking = { type = "limited", allowed_hosts = ["api.example.com"] }
`

	cases := map[string]struct {
		auth string
		want string
	}{
		"env var with mcp_server_url": {
			auth: envBase + limited + `    mcp_server_url = "https://mcp.example.com/mcp"`,
			want: `auth.mcp_server_url must not be set`,
		},
		"env var with token": {
			auth: envBase + limited + `    token = "t"`,
			want: `auth.token must not be set`,
		},
		"env var missing secret_name": {
			auth: `
    type         = "environment_variable"
    secret_value = "s"
` + limited,
			want: `auth.secret_name is required`,
		},
		"env var missing secret_value": {
			auth: `
    type        = "environment_variable"
    secret_name = "K"
` + limited,
			want: `auth.secret_value is required`,
		},
		"env var missing networking": {
			auth: envBase,
			want: `auth.networking is required`,
		},
		"limited without allowed_hosts": {
			auth: envBase + `    networking = { type = "limited" }`,
			want: `allowed_hosts is required`,
		},
		"limited with empty allowed_hosts": {
			auth: envBase + `    networking = { type = "limited", allowed_hosts = [] }`,
			want: `between 1 and 16 entries`,
		},
		"unrestricted with allowed_hosts": {
			auth: envBase + `    networking = { type = "unrestricted", allowed_hosts = ["a.example.com"] }`,
			want: `allowed_hosts must not be set`,
		},
		"bad networking type": {
			auth: envBase + `    networking = { type = "open" }`,
			want: `networking.type must be`,
		},
		"static_bearer missing mcp_server_url": {
			auth: `
    type  = "static_bearer"
    token = "t"`,
			want: `auth.mcp_server_url is required`,
		},
		"static_bearer with secret_name": {
			auth: `
    type           = "static_bearer"
    mcp_server_url = "https://mcp.example.com/mcp"
    token          = "t"
    secret_name    = "K"`,
			want: `auth.secret_name must not be set`,
		},
		"unknown type": {
			auth: `
    type           = "api_key"
    mcp_server_url = "https://mcp.example.com/mcp"`,
			want: `auth.type must be one of`,
		},
	}

	for name, tc := range cases {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      cred(tc.auth),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(regexp.QuoteMeta(tc.want)),
					},
				},
			})
		})
	}
}
