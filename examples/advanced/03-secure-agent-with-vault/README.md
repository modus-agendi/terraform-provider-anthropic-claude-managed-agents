# Secure agent with restricted environment and vault credentials

End-to-end composition: a single agent that talks to Linear via an MCP
toolset, scoped to a locked-down sandbox, with credentials pulled from a
per-end-user vault.

## What you'll learn

- How to compose all four resources types in one config:
  `environment`, `vault`, `vault_credential`, and `agent`.
- A realistic security pattern:
  - Outbound HTTP is constrained by the environment's `allowed_hosts`.
  - The agent's `web_fetch` tool is disabled so outbound HTTP must go
    through MCP (which is governed by vault credentials).
  - The `linear` MCP toolset defaults to `always_ask` so a human approves
    each tool call.
- That the agent resource itself does not reference the vault — the
  binding happens at session-creation time in your application code.
  Terraform's role here is to provision the building blocks.

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported
- A real Linear personal access token for `var.linear_token`

## Run

```sh
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform apply
```

## Tear down

```sh
terraform destroy
```

The environment is deleted (or archived if sessions still reference it).
The vault and credential are archived (audit trail preserved). The agent
is archived (one-way).
