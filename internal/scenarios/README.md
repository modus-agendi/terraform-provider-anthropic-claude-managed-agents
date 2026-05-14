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

## Running locally

```sh
export ANTHROPIC_API_KEY=sk-ant-…
TF_ACC_SCENARIOS=1 go test ./internal/scenarios/... -count=1 -timeout 10m -v
```

Without `TF_ACC_SCENARIOS=1`, `TestScenarios` skips. The harness's
loader / check / pricing / summary unit tests still run under `make test`
(gated by `TF_ACC=1`).

## Adding a scenario

1. Drop a new `*.yaml` under `internal/scenarios/scenarios/`.
2. Required fields: `name`, `terraform_config`, `question`, `rubric`.
3. Optional: `timeout` (default 5m), `judge_model` (default
   `claude-sonnet-4-6`), `trajectory_checks`.
4. The Terraform config MUST declare exactly one
   `claude-managed-agents_agent.target` resource — that's the agent the
   harness opens a session against. Optionally declare
   `claude-managed-agents_environment.sandbox` to attach an environment.
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
