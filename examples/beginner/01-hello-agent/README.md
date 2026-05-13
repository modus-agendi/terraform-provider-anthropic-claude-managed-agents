# Hello Agent

The smallest agent that runs — two required fields and nothing else.

## What you'll learn

- How to declare the `claude-managed-agents` provider.
- How to create a `claude-managed-agents_agent` resource with the minimum
  set of fields (`name` + `model`).
- That every other agent attribute (`system`, `description`, `metadata`,
  `tools`, `mcp_servers`, `skills`, `multiagent`) is optional.

## Prerequisites

- Terraform >= 1.11 or OpenTofu >= 1.8
- `ANTHROPIC_API_KEY` exported in your shell

## Run

```sh
export ANTHROPIC_API_KEY=sk-ant-...
terraform init
terraform plan
terraform apply
```

## Verify

The apply output should print the agent's `agent_id` (a `agent_*` string)
and a `version` of `1`.

## Tear down

```sh
terraform destroy
```

`destroy` archives the agent (`POST /v1/agents/{id}/archive`). The
upstream API does not expose a hard-delete for agents.
