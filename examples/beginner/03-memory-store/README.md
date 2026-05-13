# Memory store

Create two memory stores side-by-side to contrast the archive vs.
hard-delete lifecycle.

## What you'll learn

- How to create a `claude-managed-agents_memory_store`.
- That `description` is surfaced in the agent's system prompt when the
  store is attached at session runtime.
- The `delete_on_destroy` boolean: default `false` (archive on destroy,
  preserving the audit trail) vs. `true` (hard-delete, no recovery).

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported

## Run

```sh
terraform init
terraform apply
```

## Verify

After `apply`, two memory store ids should print — both `memstore_*`.

## Tear down

```sh
terraform destroy
```

`project_notes` is archived (audit trail preserved). `scratch` is hard-deleted.
