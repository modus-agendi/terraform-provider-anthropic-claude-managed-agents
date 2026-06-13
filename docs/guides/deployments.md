---
page_title: "Deployments: scheduling and running agents"
subcategory: "Guides"
description: |-
  How to model Claude Managed Agents deployments in Terraform: scheduled vs
  manual runs, the desired_status/status split, mounting resources, and
  reading run history. Includes the caveats you need before relying on it.
---

# Deployments

A **deployment** is a configured, persistent instance of an agent: it binds an
agent to an environment (and optionally vaults, mounted resources, and a set of
initial events), then runs that agent on a schedule or on demand. Each time a
deployment fires it produces one **run** (an append-only audit record) which, on
success, starts a **session** that executes the agent's `initial_events`.

```
deployment  ──fires──▶  run (audit record)  ──starts──▶  session (executes initial_events)
   │                        │                                  │
 schedule or            depl_…  drun_…                      the agent does the work
 out-of-band            success → session_id
                        failure → error_type
```

This guide covers what the reference pages
([resource](../resources/deployment.md),
[`deployment_runs` data source](../data-sources/deployment_runs.md)) do not: how
the pieces fit together, and the behaviors that will surprise you if you do not
know them up front. Read the [Caveats and nuances](#caveats-and-nuances) section
before you depend on deployments in production.

## A minimal deployment

The smallest useful deployment binds an agent to an environment and gives it one
message to act on. With no `schedule`, it does not fire on its own (see
[Scheduled vs manual](#scheduled-vs-manual-runs)).

```hcl
resource "claude-managed-agents_agent" "digest" {
  name   = "Nightly Digest"
  model  = "claude-opus-4-7"
  system = "Summarize the day's activity into a concise digest."
}

resource "claude-managed-agents_environment" "default" {
  name = "digest-env"
  config = {
    type       = "cloud"
    networking = { type = "unrestricted" }
  }
}

resource "claude-managed-agents_deployment" "digest" {
  name           = "nightly-digest"
  agent          = claude-managed-agents_agent.digest.id
  environment_id = claude-managed-agents_environment.default.id

  initial_events = [
    {
      type    = "user.message"
      content = jsonencode([{ type = "text", text = "Run the nightly digest." }])
    },
  ]
}
```

## Scheduled vs manual runs

A deployment fires in one of two ways.

**Scheduled** — add a `schedule` block with a 5-field POSIX cron expression
(no `@daily`-style shortcuts) and an IANA timezone. The platform fires the
deployment on that cadence.

```hcl
resource "claude-managed-agents_deployment" "digest" {
  # ... agent, environment_id, initial_events ...

  schedule = {
    type       = "cron"
    expression = "0 3 * * *" # 03:00 every day
    timezone   = "UTC"
  }
}
```

**Manual / on-demand** — omit `schedule`. The deployment exists but only runs
when something fires it out of band.

> **Caveat — the provider does not fire deployments.** Terraform creates,
> configures, and tears down deployments; it does not trigger runs. A
> deployment with no `schedule` will not run from `terraform apply`. Fire it
> from the Anthropic console or the REST API. The manual-run endpoint
> (`POST /v1/deployments/{id}/run`) is **undocumented** in the published SDK
> reference; this project confirmed it by live probe and uses it only inside
> the L5 test harness ([details](https://github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/blob/main/internal/scenarios/README.md)).
> Do not build production automation on it without confirming it against your
> own account.

> **Caveat — minimum schedule cadence.** The deployments API enforces a
> **1-hour minimum** between scheduled fires. Cron expressions that imply a
> tighter cadence are clamped/rejected by the API, not by this provider.

The `schedule` block's read-only "next run" enrichment from the API is
intentionally **not** surfaced in Terraform state, so it cannot churn your
plans. Query the next-run time via the API or console.

## Pause and resume: `desired_status` vs `status`

This is the single most important behavior to understand.

- `desired_status` (writable: `active` | `paused`, default `active`) is **your
  intent**. Setting it to `paused` calls the pause endpoint; setting it back to
  `active` calls resume.
- `status` (read-only: `active` | `paused`) is **what the API observes**.
- `paused_reason` (read-only) explains a pause: `type = "manual"` (you paused
  it) or `type = "error"` (the platform auto-paused it after a run failure,
  with the typed cause under `paused_reason.error`).

The two are **deliberately decoupled**. When the platform auto-pauses a
deployment on error, `status` becomes `paused` while your `desired_status`
stays `active`. Terraform sees the difference but does **not** fight it: it will
not blindly re-issue resume on every plan and flap the deployment. You inspect
`paused_reason`, fix the underlying cause, and re-apply to resume.

To pause deliberately:

```hcl
resource "claude-managed-agents_deployment" "digest" {
  # ...
  desired_status = "paused"
}
```

To detect and react to an error-pause, surface the gap:

```hcl
output "digest_health" {
  value = {
    intended  = claude-managed-agents_deployment.digest.desired_status # "active"
    observed  = claude-managed-agents_deployment.digest.status         # "paused" if auto-paused
    paused_by = try(claude-managed-agents_deployment.digest.paused_reason.type, null)  # "error"
    error     = try(claude-managed-agents_deployment.digest.paused_reason.error.type, null) # e.g. "vault_not_found_error"
  }
}
```

Recovery flow after an error-pause:

1. `status = "paused"`, `paused_reason.type = "error"`,
   `desired_status = "active"`. Terraform reports drift on `status` but takes no
   corrective action (status is read-only).
2. Read `paused_reason.error.type` / `.message`, fix the cause (e.g. restore the
   missing vault, repair the environment).
3. Run `terraform apply`. Because `desired_status` is still `active`, the
   provider re-issues resume and `status` returns to `active`.

## Initial events

`initial_events` (1–50 entries) is the script sent to every session the
deployment starts. Three variants, discriminated by `type`:

```hcl
initial_events = [
  # 1. A user message. `content` is a jsonencode'd array of content blocks —
  #    kept as a JSON string because blocks are a deep union (text/image/
  #    document with nested sources) that round-trips anything the API accepts.
  {
    type    = "user.message"
    content = jsonencode([{ type = "text", text = "Produce today's digest." }])
  },

  # 2. A define_outcome task: the agent works toward `description`, graded
  #    against `rubric`, refining up to `max_iterations` times (default 3,
  #    max 20). This is the autonomous task-completion path.
  {
    type           = "user.define_outcome"
    description    = "Write a one-page digest of today's activity to digest.md."
    max_iterations = 3
    rubric = {
      type    = "text" # or "file" with file_id
      content = "PASS only if every claim cites a source and there is no speculation."
    }
  },

  # 3. A privileged system message. MUST be the last event and MUST follow a
  #    user.message; only supported on models that accept system messages.
  {
    type    = "system.message"
    content = jsonencode([{ type = "text", text = "Use a terse, factual tone." }])
  },
]
```

> **Caveat — editing `initial_events` forces replacement.** The API does not
> patch initial events in place, so any change to this list replaces the whole
> deployment (a new `id`). Plan accordingly: an in-place edit you expect to be
> cheap will destroy and recreate.

## Mounting resources

`resources` (max 500) mounts external context into every session the deployment
starts. Three variants:

```hcl
resources = [
  # A git repository. The token is WRITE-ONLY: sent to the API, never stored in
  # state or read back. Rotate by changing the token AND bumping its _wo_version.
  {
    type                           = "github_repository"
    url                            = "https://github.com/acme/reports"
    authorization_token            = var.github_token # write-only (TF 1.11+)
    authorization_token_wo_version = 1                # bump to re-send on rotation
    checkout                       = { type = "branch", name = "main" } # or { type = "commit", sha = "..." }
  },

  # An uploaded file, by id.
  {
    type    = "file"
    file_id = "file_01ABC..."
  },

  # A memory store, with access level and usage instructions for the agent.
  {
    type            = "memory_store"
    memory_store_id = claude-managed-agents_memory_store.notes.id
    access          = "read_write" # or "read_only"
    instructions    = "Persist and retrieve the user's status files here."
  },
]
```

> **Caveat — write-only token on import.** Because `authorization_token` is
> never returned by the API, after `terraform import` you must set the token
> (and bump `authorization_token_wo_version`) in config before the next apply,
> or the github mount will be sent without credentials.

Vault credentials are mounted separately via `vault_ids` (a list of vault ids,
max 50) — credentials managed by `claude-managed-agents_vault` /
`claude-managed-agents_vault_credential` become available to each session.

## Reading run history

Every fire — scheduled or manual — appends one **run** record. Exactly one of
`session_id` (success) or `error_type` (failure) is set. Use the
`deployment_runs` data source to read them and alert on failures:

```hcl
data "claude-managed-agents_deployment_runs" "failures" {
  deployment_id = claude-managed-agents_deployment.digest.id

  # All filters are optional and combinable:
  trigger_type = "schedule" # or "manual"
  has_error    = true       # failed runs only
  limit        = 50         # default 20, max 1000 (first page only)
}

output "recent_failures" {
  value = [
    for run in data.claude-managed-agents_deployment_runs.failures.runs : {
      id         = run.id         # drun_...
      error_type = run.error_type # typed failure reason
      message    = run.error_message
      at         = run.created_at
    }
  ]
}
```

The `error_type` is a typed taxonomy (for example `environment_archived_error`,
`vault_not_found_error`) — the same shape the L5 `deployment_error_taxonomy`
scenario asserts against the live API. Wiring these into an alert lets you catch
a misconfigured deployment the first time it fires rather than on the next
manual inspection.

## Lifecycle summary

| Operation | Behavior |
|---|---|
| Create / Read / Update | `PATCH` is **last-write-wins** — there is no optimistic-concurrency `version`. Concurrent edits silently clobber. |
| Update `initial_events` | Forces replacement (new `id`). |
| Pause / resume | Driven by `desired_status`, reconciled via the pause/resume endpoints; decoupled from observed `status`. |
| Destroy | Archives the deployment (`POST /v1/deployments/{id}/archive`). **One-way** — there is no `DELETE` endpoint, and archived deployments cannot be unarchived. |
| `metadata` | Key-level merge: removing a key from HCL deletes it server-side. |
| Import | `terraform import claude-managed-agents_deployment.<name> depl_…` (ids are `depl_…`). Re-supply the write-only github token afterward. |

## Caveats and nuances

A consolidated list — most are expanded above.

- **The provider does not trigger runs.** Deployments run on their `schedule` or
  out of band. There is no Terraform-level "run now".
- **The manual-run endpoint is undocumented.** `POST /v1/deployments/{id}/run`
  is not in the published SDK reference; this project confirmed it by live probe
  and uses it only in the L5 harness.
- **1-hour minimum schedule cadence**, enforced by the API.
- **`initial_events` edits force replacement** — no in-place patch.
- **Updates are last-write-wins** — no concurrency guard.
- **Destroy is archive, one-way** — no delete, no unarchive.
- **`status` is read-only and can diverge from `desired_status`** on an
  error-pause; that divergence is expected and Terraform will not auto-correct
  it. Resume by fixing the cause and re-applying.
- **The github `authorization_token` is write-only** — never in state, must be
  re-supplied after import; rotate via `authorization_token_wo_version`.
- **Schedule next-run time is not in state**, by design.
- **`agent_version` is computed.** The deployment pins the agent's latest
  version at create/update time and exposes the resolved version as
  `agent_version`; updating the agent does not re-pin until you re-apply the
  deployment.

## See also

- [`claude-managed-agents_deployment` resource reference](../resources/deployment.md)
- [`claude-managed-agents_deployment` data source](../data-sources/deployment.md)
- [`claude-managed-agents_deployment_runs` data source](../data-sources/deployment_runs.md)
- [Worked example: scheduled deployment with an outcome](https://github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/tree/main/examples/advanced/05-scheduled-deployment-with-outcome)
