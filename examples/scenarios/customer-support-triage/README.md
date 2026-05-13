# Scenario: customer support triage

A realistic, end-to-end scenario combining every resource the provider
ships:

- `claude-managed-agents_environment` — locked-down sandbox.
- `claude-managed-agents_memory_store` — shared playbooks.
- `claude-managed-agents_agent` × 3 — triage worker, investigator worker,
  shift-lead coordinator.
- `claude-managed-agents_vault` — per-end-user credentials.
- `claude-managed-agents_vault_credential` × 2 — Linear (static bearer)
  and Slack (OAuth with refresh).

## Topology

```
                    Support Shift Lead (coordinator)
                       /                  \
            Triage Worker             Investigator
                |                          |
           [Linear MCP]               [Linear MCP]
           [Slack MCP]
                |
   Vault: Support engineer credentials
     ├── static_bearer  → mcp.linear.app
     └── mcp_oauth      → mcp.slack.com
                |
           Environment:
              networking = limited,
              allowed_hosts = [mcp.linear.app, mcp.slack.com, api.example.com],
              allow_mcp_servers = true,
              allow_package_managers = false

           Memory Store: Support Playbooks (attached at session time)
```

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported
- Real values for `linear_token`, `slack_access_token`,
  `slack_refresh_token`, `slack_client_secret`

## Run

```sh
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform apply
```

## Verify

Outputs print the four resource ids (`shift_lead_id`, `environment_id`,
`playbooks_id`, `vault_id`). Your application can now create a session
against the shift lead, attaching the memory store and the vault at
session-creation time.

## Tear down

```sh
terraform destroy
```
