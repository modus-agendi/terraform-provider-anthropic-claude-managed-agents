package provider

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDeploymentDataSource_byID(t *testing.T) {
	skipUnlessAcc(t)
	_, cleanup := startFakeAPI(t)
	defer cleanup()

	name := testAgentName("dep-ds")
	cfg := deploymentConfig(name, `
  description = "Looked up by the data source."`) + `

data "claude-managed-agents_deployment" "by_id" {
  id = claude-managed-agents_deployment.d.id
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment.by_id", "name", name),
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment.by_id", "description", "Looked up by the data source."),
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment.by_id", "status", "active"),
					resource.TestCheckResourceAttrSet("data.claude-managed-agents_deployment.by_id", "agent_version"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment.by_id", "initial_events.0.type", "user.message"),
				),
			},
		},
	})
}

func TestAccDeploymentRunsDataSource_basic(t *testing.T) {
	skipUnlessAcc(t)
	if liveMode() {
		t.Skip("seeded run records require the in-process fake API")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	sessionID := "session_OK"
	now := time.Now().UTC().Format(time.RFC3339)

	cfg := providerConfig() + `
data "claude-managed-agents_deployment_runs" "for_dep" {
  deployment_id = "deployment_SEED"
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					api.SeedDeploymentRun(&fakeDeploymentRun{
						ID:             "drun_ok",
						Type:           "deployment_run",
						Agent:          map[string]any{"id": "agent_x", "type": "agent", "version": 2},
						DeploymentID:   "deployment_SEED",
						CreatedAt:      now,
						SessionID:      &sessionID,
						TriggerContext: map[string]any{"type": "schedule", "scheduled_at": now},
					})
					api.SeedDeploymentRun(&fakeDeploymentRun{
						ID:             "drun_err",
						Type:           "deployment_run",
						Agent:          map[string]any{"id": "agent_x", "type": "agent", "version": 2},
						DeploymentID:   "deployment_SEED",
						CreatedAt:      now,
						Error:          map[string]any{"type": "vault_not_found_error", "message": "gone"},
						TriggerContext: map[string]any{"type": "schedule", "scheduled_at": now},
					})
					// A run for a different deployment, to prove the filter works.
					api.SeedDeploymentRun(&fakeDeploymentRun{
						ID:             "drun_other",
						Type:           "deployment_run",
						Agent:          map[string]any{"id": "agent_y", "type": "agent", "version": 1},
						DeploymentID:   "deployment_OTHER",
						CreatedAt:      now,
						SessionID:      &sessionID,
						TriggerContext: map[string]any{"type": "manual"},
					})
				},
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment_runs.for_dep", "runs.#", "2"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment_runs.for_dep", "runs.0.session_id", "session_OK"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment_runs.for_dep", "runs.0.trigger_type", "schedule"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment_runs.for_dep", "runs.1.error_type", "vault_not_found_error"),
				),
			},
		},
	})
}

func TestAccDeploymentRunsDataSource_hasErrorFilter(t *testing.T) {
	skipUnlessAcc(t)
	if liveMode() {
		t.Skip("seeded run records require the in-process fake API")
	}
	api, cleanup := startFakeAPI(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	sessionID := "session_OK"
	cfg := providerConfig() + `
data "claude-managed-agents_deployment_runs" "errors" {
  deployment_id = "deployment_FILT"
  has_error     = true
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					api.SeedDeploymentRun(&fakeDeploymentRun{
						ID: "drun_a", Type: "deployment_run", DeploymentID: "deployment_FILT",
						Agent: map[string]any{"id": "agent_x", "type": "agent", "version": 1}, CreatedAt: now,
						SessionID: &sessionID, TriggerContext: map[string]any{"type": "manual"},
					})
					api.SeedDeploymentRun(&fakeDeploymentRun{
						ID: "drun_b", Type: "deployment_run", DeploymentID: "deployment_FILT",
						Agent: map[string]any{"id": "agent_x", "type": "agent", "version": 1}, CreatedAt: now,
						Error: map[string]any{"type": "session_rate_limited_error", "message": "rate"}, TriggerContext: map[string]any{"type": "schedule", "scheduled_at": now},
					})
				},
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment_runs.errors", "runs.#", "1"),
					resource.TestCheckResourceAttr("data.claude-managed-agents_deployment_runs.errors", "runs.0.error_type", "session_rate_limited_error"),
				),
			},
		},
	})
}
