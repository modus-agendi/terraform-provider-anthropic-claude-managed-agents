---
page_title: "Drift detection and remediation"
subcategory: "Guides"
description: |-
  Detecting and reconciling out-of-band changes to Claude Managed Agents
  resources, the expected-drift patterns specific to this provider, and when to
  use ignore_changes.
---

# Drift detection and remediation

~> **Reminder:** experimental community provider — not for production, no
warranty, use at your own risk.

## Detecting drift

Use a refresh-only plan with a detailed exit code in CI to detect out-of-band
changes without applying anything:

```sh
terraform plan -refresh-only -detailed-exitcode
# exit 0 = no drift, 1 = error, 2 = drift detected
```

Wire that into a scheduled job (CI cron, or Terraform Cloud's drift detection)
and alert on exit code `2`. To see the difference and sync state without
changing any resource:

```sh
terraform apply -refresh-only
```

## Expected-drift patterns specific to this provider

Some changes are normal and surface as drift on refresh:

- **Skill content (`claude-managed-agents_skill`).** `content_hash` is computed
  from `source_dir` at every plan. If someone uploads a new skill version out of
  band (or the local `source_dir` changes), the next plan shows a new
  `latest_version`. This is intentional drift detection — re-apply to converge.
- **Agent `version`.** Server-managed, used for optimistic concurrency. If you
  hit a version conflict on update, run `terraform apply -refresh-only` to sync
  the version, then re-apply your change.
- **Deployment `status` (auto-pause).** The platform can pause a deployment on a
  run error, moving `status` to `paused` while your `desired_status` stays
  `active`. This is **deliberately decoupled** — Terraform does not fight it.
  Inspect `paused_reason`, fix the cause, then `terraform apply` to resume. See
  the [Deployments guide](deployments.md).
- **`metadata` merge.** Removing a key from HCL deletes it server-side. If an
  external system tags resources via metadata, those keys will show as drift
  (Terraform wants to remove them) — see `ignore_changes` below.

## Remediation

| Situation | Action |
|---|---|
| State out of sync, resources unchanged | `terraform apply -refresh-only` |
| Out-of-band change you want to keep | refresh, then update HCL to match |
| Out-of-band change you want to revert | `terraform apply` (re-asserts HCL) |
| Agent version conflict on update | `terraform apply -refresh-only`, then re-apply |
| Deployment error-paused | read `paused_reason.error`, fix, `terraform apply` |

## `lifecycle { ignore_changes }`

Use `ignore_changes` when an attribute is legitimately managed outside
Terraform and you don't want plans to fight it:

```hcl
# Ops teams tag agents server-side via metadata; don't let Terraform remove it.
resource "claude-managed-agents_agent" "assistant" {
  name  = "Assistant"
  model = "claude-opus-4-7"
  metadata = { team = "platform" }

  lifecycle {
    ignore_changes = [metadata]
  }
}
```

```hcl
# Manage the agent's spec in Terraform, but let the version float (e.g. another
# pipeline updates the agent), avoiding optimistic-concurrency churn.
resource "claude-managed-agents_agent" "assistant" {
  # ...
  lifecycle {
    ignore_changes = [version]
  }
}
```

Reach for `ignore_changes` sparingly — it suppresses real drift too. Prefer it
only where a field is genuinely co-owned with another system.

## Cost note

Frequent `plan` runs are cheap (read-only API calls), but the
`claude-managed-agents_skill` plan walks and hashes `source_dir` on every plan;
keep skill directories reasonably small.

## See also

- [Deployments guide](deployments.md)
- [State security and remote backends](state-security-and-backends.md)
- [Upgrading and migration](upgrading-and-migration.md)
