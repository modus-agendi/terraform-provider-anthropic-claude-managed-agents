---
page_title: "Import and state recovery"
subcategory: "Guides"
description: |-
  How to adopt existing Claude Managed Agents resources into Terraform with
  terraform import, including the composite vault-credential id and the
  write-only fields you must re-supply afterward.
---

# Import and state recovery

~> **Reminder:** experimental community provider — not for production, no
warranty, use at your own risk.

## Import support by resource

Every resource supports `terraform import`. Most take their bare server id; the
vault credential takes a composite id.

| Resource | Import id | Notes |
|---|---|---|
| `claude-managed-agents_agent` | `agent_…` | |
| `claude-managed-agents_environment` | `env_…` | |
| `claude-managed-agents_memory_store` | `mem_…` | |
| `claude-managed-agents_skill` | `skill_…` | re-supply `source_dir` after import (see below) |
| `claude-managed-agents_vault` | `vlt_…` | |
| `claude-managed-agents_deployment` | `depl_…` | re-supply the github token after import (see below) |
| `claude-managed-agents_vault_credential` | `<vault_id>:<credential_id>` | **composite id** |

## Basic flow

1. Write a resource block matching the upstream object (name/config can be
   filled in after the import refreshes state).
2. Import:

   ```sh
   terraform import claude-managed-agents_agent.assistant agent_01ABC...
   ```

3. Run `terraform plan` and reconcile your HCL until the plan is empty.

For the **vault credential**, the id is `vault_id` and `credential_id` joined by
a colon:

```sh
terraform import claude-managed-agents_vault_credential.linear \
  vlt_01HqR2k7vXbZ9mNpL3wYcT8f:vcrd_01HFEDCBA9876543210ABCD
```

## Write-only fields you MUST re-supply after import

Write-only secrets are never returned by the API, so after import they are
`null` in state. Before your next apply you must put the value back in config
**and** set the rotation counter, or the apply will send the resource without
its credential:

- **`claude-managed-agents_vault_credential`** — set `auth.token` (or the OAuth
  secrets) and the matching `*_wo_version` (start at `1`).
- **`claude-managed-agents_deployment`** with a `github_repository` mount — set
  `resources[*].authorization_token` and `authorization_token_wo_version`.

See [Secrets management and rotation](secrets-management-and-rotation.md) for the
mechanics.

## Skill import caveat

`claude-managed-agents_skill` is driven by a local `source_dir` that the API
does not store. After import, set `source_dir` in config to the directory whose
contents match the uploaded skill. The provider hashes the directory at plan
time; if it differs from the imported `content_hash`, the next apply uploads a
new version. Import verifies cleanly only when `source_dir` reproduces the
uploaded content (the import test ignores `source_dir`, `content_hash`, and
`version_rotation`).

## Bulk import

For adopting many existing resources, generate `import` blocks (Terraform 1.5+)
instead of running the CLI per-resource:

```hcl
import {
  to = claude-managed-agents_agent.assistant
  id = "agent_01ABC..."
}
```

Run `terraform plan -generate-config-out=generated.tf` to scaffold matching HCL,
then review and edit (especially to add any write-only fields, which generation
cannot recover).

## State recovery

- **Restore a corrupted/lost state** from your backend's version history (S3/GCS
  versioning, TFC state versions) — see [State security](state-security-and-backends.md).
- **Remove a resource from state without destroying it:**
  `terraform state rm claude-managed-agents_agent.assistant` (then re-import if
  desired).
- **Archived-on-destroy is one-way.** Agents, vaults, vault credentials, memory
  stores, and deployments archive on destroy and cannot be unarchived. State
  recovery restores Terraform's record, not an archived upstream resource.

## See also

- [Secrets management and rotation](secrets-management-and-rotation.md)
- [Drift detection and remediation](drift-detection-and-remediation.md)
