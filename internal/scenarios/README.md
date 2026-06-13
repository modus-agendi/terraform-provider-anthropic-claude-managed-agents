# internal/scenarios — L5 behavioral test layer

Scenarios are YAML files that describe a Terraform config plus a question
for the agent to answer. The harness:

1. Applies the config via `resource.UnitTest` (provider, agent,
   environment, …).
2. Opens a real session against `api.anthropic.com` for the agent.
3. Sends the question, polls `/v1/sessions/{id}/events` every 2s, and
   accumulates the full event trajectory.
4. Runs the declared `trajectory_checks` (deterministic predicates on
   the event stream).
5. Calls `/v1/messages` with a separate "judge" model that grades the
   agent's final answer against the rubric and returns PASS / FAIL.
6. Archives the session; Terraform destroy cleans the rest.

L5 sits alongside L1–L4 (see `CLAUDE.md`, "Test layer model"). It
catches **behavioral regressions** — does the agent we provision actually
perform tasks? — not CRUD regressions, which L1/L2/L3 cover.

## Cost

L5 bills real Anthropic inference tokens. Approximate per-scenario cost
is **$0.05–$0.20** depending on tool-use depth. The harness prints a
summary table after every run with token totals and a dollar estimate
keyed by `pricing.go`. The pricing table drifts; the CI line links to
<https://www.anthropic.com/pricing> for live rates.

### Live validation (2026-06-13)

The full catalog was run live against `api.anthropic.com` — **6/6 PASS**,
~198s wall, **~$0.68 total**. Actual harness summary:

| Scenario | Kind | Time | Verdict |
|---|---|---|---|
| `deployment_define_outcome_csv` | deployment | 72s | PASS — `bash` used, outcome `satisfied`, judge PASS |
| `deployment_error_taxonomy` | deployment | 5s | PASS — run recorded `environment_archived_error`, 0 tokens |
| `deployment_memory_store_resource` | deployment | 15s | PASS — mounted store written + read back |
| `deployment_pause_resume` | lifecycle | 4s | PASS — pause→`manual`, resume→active, 0 tokens |
| `fibonacci_default_toolset` | agent | 15s | PASS |
| `multi_capability_research` | agent | 87s | PASS |

The two `define_outcome`/research scenarios dominate cost (cache reads priced
at 10% of input). The two lifecycle/error scenarios spend zero inference.
Re-run after any deployment-resource or harness change with the command below.

## Running locally

```sh
export ANTHROPIC_API_KEY=sk-ant-…
TF_ACC_SCENARIOS=1 go test ./internal/scenarios/... -count=1 -timeout 10m -v
```

Without `TF_ACC_SCENARIOS=1`, `TestScenarios` skips. The harness's
loader / check / pricing / summary unit tests still run under `make test`
(gated by `TF_ACC=1`).

## Scenario kinds

A scenario's `kind` selects its shape (default `agent`):

| Kind | What it drives | Target resource | Required fields |
|---|---|---|---|
| `agent` | Opens a session against the agent and posts `question` | `claude-managed-agents_agent.target` | `question`, `rubric` |
| `deployment` | Fires the deployment via a **manual run** (`POST /v1/deployments/{id}/run`); the deployment's own `initial_events` drive the session | `claude-managed-agents_deployment.target` | `terraform_config` (judge needs `outcome_description` or `question` + `rubric`) |
| `lifecycle` | Drives deployment pause/resume client ops and asserts the transitions; no session, no judge, no tokens | `claude-managed-agents_deployment.target` | `lifecycle_checks` |

### Deployment scenarios

Because the public deployments API is schedule-only (1-hour minimum), the
harness fires deployments via `POST /v1/deployments/{id}/run` — an
**undocumented manual-trigger endpoint** confirmed by live probe. It returns a
`DeploymentRun` synchronously with the new `session_id` and
`trigger_context.type == "manual"`. The harness drives + judges that session
(the `initial_events`, e.g. a `user.define_outcome`, run autonomously — no
`question` is posted), then asserts `run_checks` on the run record.

Extra fields for deployment scenarios:

- `outcome_description` — the judge's TASK line (since there's no `question`).
- `run_checks` — assertions over the `DeploymentRun` (see table below).
- `pre_run_archive: environment` — archive the deployment's environment
  out-of-band before the run, to drive a run-time error path. When the run
  fails to create a session, the harness skips the session/judge phase and
  runs only `run_checks`.

`run_checks`:

| Key | Arg | Passes when |
|---|---|---|
| `require_run_trigger_type` | `"manual"` / `"schedule"` | `run.trigger_context.type == arg` |
| `require_run_session_set` | ignored | the run started a session (`session_id` non-null) |
| `require_run_no_error` | ignored | the run has no error |
| `require_run_error_type` | error-type string | `run.error.type == arg` (e.g. `environment_archived_error`) |

`lifecycle_checks`:

| Key | Arg | Drives + asserts |
|---|---|---|
| `require_pause_resume_cycle` | ignored | Pause → `status=paused` + `paused_reason.type=manual`; Resume → `status=active` + `paused_reason=null` |

## Adding a scenario

1. Drop a new `*.yaml` under `internal/scenarios/scenarios/`.
2. Set `kind` (or omit for `agent`). Always required: `name`,
   `terraform_config`. Other required fields depend on the kind (table above).
3. Optional: `timeout` (default 5m), `judge_model` (default
   `claude-sonnet-4-6`), `trajectory_checks`.
4. An `agent` scenario MUST declare exactly one
   `claude-managed-agents_agent.target`; a `deployment`/`lifecycle` scenario
   MUST declare `claude-managed-agents_deployment.target`. Optionally declare
   `claude-managed-agents_environment.sandbox` (agent kind) for an environment.
5. All resource names MUST start with `tf-acc-test-scenarios-` so the
   shared sweeper (1h age threshold, `tf-acc-test-` prefix) cleans up
   orphans from crashed runs.

## Trajectory checks

The check registry is closed Go-side (see `checks.go`). YAML values are
single-key maps:

```yaml
trajectory_checks:
  - require_event: "agent.tool_use"
  - require_terminal_stop_reason: "end_turn"
  - require_no_session_errors: true
  - require_tool_use_named: "code_execution"
```

| Key | Arg | Passes when |
|---|---|---|
| `require_event` | event type string | Any event in trajectory matches that type |
| `require_terminal_stop_reason` | stop reason string | A `session.status_idle` event carries `stop_reason.type == arg` |
| `require_no_session_errors` | ignored | No `session.error` events appeared |
| `require_tool_use_named` | tool name string | Any tool_use event carries `name == arg` |
| `require_outcome_result` | result string | A `span.outcome_evaluation_end` event carries `result == arg` (e.g. `satisfied`) — for `user.define_outcome` runs |

Unknown keys fail to load with a clear error listing valid keys.

## CI

The `scenarios.yml` workflow runs nightly at 04:00 UTC and on
`workflow_dispatch`. The `release.yml` workflow runs scenarios as a hard
gate between live acceptance and the goreleaser job — a scenario FAIL
blocks the release. Bypass for hotfixes via `workflow_dispatch.inputs.
skip_scenarios=true`.

## Sweeper

Resources from crashed scenario runs are caught by the existing
`sweep_test.go` sweepers in `internal/provider/`. Scenarios use the
`tf-acc-test-scenarios-` prefix; the sweeper matches `tf-acc-test-` (the
shared root) and applies the 1h age threshold to skip in-flight runs.

Run the sweeper alone with `make sweep` or
`gh workflow run scenarios.yml -f sweep_only=true`.
