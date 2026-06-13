# Scheduled deployment with an outcome

A nightly **deployment**: it runs an agent on a cron schedule, hands it a
`define_outcome` task, mounts a memory store the agent persists into, and
exposes the deployment's run history so you can alert on failures.

## What you'll learn

- How a deployment binds an agent + environment + mounted resources +
  `initial_events` + a `schedule` into one runnable unit.
- The `define_outcome` initial event: the agent works toward a description,
  graded against a rubric, refining up to `max_iterations` times.
- Mounting a `memory_store` into every session the deployment starts.
- The `desired_status` (intent) vs `status` (observed) split — and how to
  surface an error-pause as an output instead of fighting the API.
- Reading append-only run records with the `deployment_runs` data source.

## Topology

```
        schedule (0 3 * * * UTC)
                 │
                 ▼
   claude-managed-agents_deployment.nightly
        ├── agent        → Nightly Reporter (default toolset)
        ├── environment  → cloud sandbox
        ├── resources    → memory_store (read_write)
        └── initial_events → user.define_outcome (compute + persist + report)
                 │ each fire
                 ▼
        run (drun_…)  ──▶  session  ──▶  report written to memory store
```

## Prerequisites

- Terraform >= 1.11 (or OpenTofu >= 1.8)
- `ANTHROPIC_API_KEY` exported

## Run

```sh
terraform init
terraform apply
```

`apply` prints `deployment_id` (a `depl_…` id), a `health` object (intended vs
observed status), and `recent_failures` (empty until a scheduled run fails).

## Caveats this example demonstrates

- **Terraform does not fire the deployment.** It runs on its `schedule` (here,
  03:00 UTC nightly) or out of band. There is no "run now" from `terraform
  apply`. The API enforces a **1-hour minimum** schedule cadence.
- **Editing `initial_events` forces replacement.** Changing the task later
  destroys and recreates the deployment (new `id`). The API has no in-place
  patch for events.
- **Destroy archives, one-way.** `terraform destroy` archives the deployment
  (no DELETE endpoint; archived deployments cannot be unarchived). The agent
  and memory store are archived too (the memory store's default preserves its
  contents — set `delete_on_destroy = true` to hard-delete).
- **`status` may diverge from `desired_status`.** If a run fails, the platform
  auto-pauses the deployment: `status` → `paused` while `desired_status` stays
  `active`. Terraform reports the drift but does not auto-resume. Read
  `paused_reason`, fix the cause, and re-apply.

For the full behavior reference, see the
[Deployments guide](https://registry.terraform.io/providers/modus-agendi/anthropic-claude-managed-agents/latest/docs/guides/deployments).

## Tear down

```sh
terraform destroy
```
