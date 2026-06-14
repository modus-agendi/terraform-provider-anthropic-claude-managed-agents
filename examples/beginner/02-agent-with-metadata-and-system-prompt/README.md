# Agent with metadata and system prompt

Adds optional flat fields to the minimal agent.

## What you'll learn

- How to set `system`, `description`, and `metadata` on an agent.
- That `metadata` is Terraform-authoritative: after apply the server matches
  your HCL, so removing a key deletes it server-side and a key added
  out-of-band is removed on the next apply.
- Setting `system` or `description` to `null` (or removing the line)
  clears the field on the next apply.

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported

## Run

```sh
terraform init
terraform apply
```

## Try it

After `apply`, edit `main.tf`:

1. Drop one key from the `metadata` map and re-`apply` — the key
   disappears from the server.
2. Change the `system` prompt — the agent's `version` increments to `2`.
3. Set `description = null` (or remove the line entirely) and re-`apply`
   — the description clears.

## Tear down

```sh
terraform destroy
```
