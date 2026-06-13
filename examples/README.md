# Examples

Three ways to use this directory.

## Quick start (5 min)

- [beginner/01-hello-agent](beginner/01-hello-agent) — the smallest agent
  that runs.

## Onboarding tutorials (progressive)

Each tutorial is a self-contained Terraform config with its own README.

| Path | What you'll learn | Resources used | Time |
|---|---|---|---|
| [beginner/01-hello-agent](beginner/01-hello-agent) | Provider setup + a minimal agent | `agent` | 5 min |
| [beginner/02-agent-with-metadata-and-system-prompt](beginner/02-agent-with-metadata-and-system-prompt) | Optional flat fields and metadata semantics | `agent` | 5 min |
| [beginner/03-memory-store](beginner/03-memory-store) | Archive vs. hard-delete lifecycle | `memory_store` | 5 min |
| [beginner/04-environment-cloud-unrestricted](beginner/04-environment-cloud-unrestricted) | Cloud sandbox with package preinstall | `environment` | 5 min |
| [beginner/05-create-custom-skill](beginner/05-create-custom-skill) | Upload a custom skill from a local directory; trigger a version bump | `skill` | 5 min |
| [intermediate/01-agent-with-mcp-and-tools](intermediate/01-agent-with-mcp-and-tools) | All three `tools` variants on one agent | `agent` | 10 min |
| [intermediate/02-agent-with-skills-and-multiagent](intermediate/02-agent-with-skills-and-multiagent) | Skills + a coordinator with workers | `agent` × 2 | 10 min |
| [intermediate/03-environment-limited-networking](intermediate/03-environment-limited-networking) | Locked-down egress with `allowed_hosts` | `environment` | 10 min |
| [advanced/01-coordinator-and-workers](advanced/01-coordinator-and-workers) | Multi-worker topology + shared memory store | `agent` × 4, `memory_store` | 15 min |
| [advanced/02-vault-with-mcp-oauth](advanced/02-vault-with-mcp-oauth) | Vault with `static_bearer` + `mcp_oauth` credentials | `vault`, `vault_credential` × 2 | 15 min |
| [advanced/03-secure-agent-with-vault](advanced/03-secure-agent-with-vault) | End-to-end secure agent composition | every resource | 20 min |
| [advanced/04-end-to-end-skill-and-agent](advanced/04-end-to-end-skill-and-agent) | Custom skill + Anthropic prebuilt skill on an agent + memory store | `skill`, `agent`, `memory_store` | 15 min |
| [advanced/05-scheduled-deployment-with-outcome](advanced/05-scheduled-deployment-with-outcome) | Scheduled deployment, `define_outcome`, mounted memory store, run history + error-pause detection | `deployment`, `agent`, `environment`, `memory_store`, data: `deployment_runs` | 20 min |

## Canonical examples (Terraform Registry docs)

These get rendered into the Terraform Registry pages by `tfplugindocs`.
Compact, idiomatic, exhaustive for that one resource.

| Resource | Canonical example | Notes |
|---|---|---|
| `claude-managed-agents_agent` | [resources/claude-managed-agents_agent/resource.tf](resources/claude-managed-agents_agent/resource.tf) | Shows all three `tools` variants and a coordinator |
| `claude-managed-agents_environment` | [resources/claude-managed-agents_environment/resource.tf](resources/claude-managed-agents_environment/resource.tf) | Both `networking.type` variants side-by-side |
| `claude-managed-agents_memory_store` | [resources/claude-managed-agents_memory_store/resource.tf](resources/claude-managed-agents_memory_store/resource.tf) | Archive vs. hard-delete |
| `claude-managed-agents_vault` | [resources/claude-managed-agents_vault/resource.tf](resources/claude-managed-agents_vault/resource.tf) | Metadata + lifecycle modes |
| `claude-managed-agents_vault_credential` | [resources/claude-managed-agents_vault_credential/resource.tf](resources/claude-managed-agents_vault_credential/resource.tf) | Both `auth.type` variants with WriteOnly secrets |
| `claude-managed-agents_deployment` | [resources/claude-managed-agents_deployment/resource.tf](resources/claude-managed-agents_deployment/resource.tf) | Schedule, `initial_events`, write-only github token; see the [Deployments guide](../docs/guides/deployments.md) |
| data: `agent` | [data-sources/claude-managed-agents_agent/data-source.tf](data-sources/claude-managed-agents_agent/data-source.tf) | Read an externally-created agent |
| data: `agent_version` | [data-sources/claude-managed-agents_agent_version/data-source.tf](data-sources/claude-managed-agents_agent_version/data-source.tf) | Look up a historical revision |
| data: `deployment` | [data-sources/claude-managed-agents_deployment/data-source.tf](data-sources/claude-managed-agents_deployment/data-source.tf) | Read an existing deployment by id |
| data: `deployment_runs` | [data-sources/claude-managed-agents_deployment_runs/data-source.tf](data-sources/claude-managed-agents_deployment_runs/data-source.tf) | List run audit records; filter + alert on `error_type` |
| data: `environment` | [data-sources/claude-managed-agents_environment/data-source.tf](data-sources/claude-managed-agents_environment/data-source.tf) | Inspect an existing sandbox |
| data: `file` | [data-sources/claude-managed-agents_file/data-source.tf](data-sources/claude-managed-agents_file/data-source.tf) | File metadata lookup |
| data: `memory_store` | [data-sources/claude-managed-agents_memory_store/data-source.tf](data-sources/claude-managed-agents_memory_store/data-source.tf) | Read a memory store |
| data: `vault` | [data-sources/claude-managed-agents_vault/data-source.tf](data-sources/claude-managed-agents_vault/data-source.tf) | Vault metadata lookup |
| data: `vault_credential` | [data-sources/claude-managed-agents_vault_credential/data-source.tf](data-sources/claude-managed-agents_vault_credential/data-source.tf) | Non-secret credential lookup |

## End-to-end scenarios

Realistic, multi-resource compositions.

| Scenario | What it builds | Key patterns |
|---|---|---|
| [scenarios/customer-support-triage](scenarios/customer-support-triage) | Triage + investigator + coordinator, locked-down env, playbooks store, per-engineer vault | every resource, both `auth.type` variants, restricted networking |

## Prerequisites

- Terraform >= 1.11 (write-only attributes require 1.11) or OpenTofu >= 1.8
- `ANTHROPIC_API_KEY` exported as an environment variable
- For OAuth examples: a real MCP server URL, OAuth client_id, and
  refresh token

## Running an example

```sh
cd beginner/01-hello-agent
cp terraform.tfvars.example terraform.tfvars  # if present
terraform init
terraform plan
terraform apply
# ... use the agent ...
terraform destroy
```

## Versioning

Examples target provider `~> 1.0` (the first stable line). The full resource
and data-source surface — agents with nested `tools` / `mcp_servers` / `skills`
/ `multiagent`, environments, vaults, memory stores, skills, and deployments —
is available on 1.x. For older `0.x` history see `CHANGELOG.md`.
