# terraform-provider-claude-managed-agents

[![CI](https://github.com/andasv/terraform-provider-claude-managed-agents/actions/workflows/test.yml/badge.svg)](https://github.com/andasv/terraform-provider-claude-managed-agents/actions/workflows/test.yml)
[![Release](https://github.com/andasv/terraform-provider-claude-managed-agents/actions/workflows/release.yml/badge.svg)](https://github.com/andasv/terraform-provider-claude-managed-agents/actions/workflows/release.yml)
[![License: MPL 2.0](https://img.shields.io/badge/license-MPL%202.0-blue.svg)](https://www.mozilla.org/MPL/2.0/)

A Terraform / OpenTofu provider for **Anthropic Claude Managed Agents** — declaratively manage agents, environments, vaults, and memory stores from Terraform configurations.

> **Unofficial.** This is a community project. It is not maintained by, endorsed by, or affiliated with Anthropic. Upstream API: [Claude Managed Agents docs](https://platform.claude.com/docs/en/managed-agents).

---

## What this provider does

Claude Managed Agents is Anthropic's hosted runtime for long-running, tool-using agents. The platform exposes a REST API for creating and configuring those agents and their supporting resources (environments, vaults, memory stores).

This provider lets you put that configuration under Terraform so you can:

- Review changes in pull requests instead of via ad-hoc API calls.
- Version-control your agent definitions alongside the rest of your infra.
- Promote agents across workspaces with the same workflow you use for everything else.

## Status / scope

**v0.1 is intentionally narrow** so the release surface is something the maintainer can support. Additional resources will land in follow-up minor versions.

| Capability | v0.1 | Planned |
|---|---|---|
| Resource `claude-managed-agents_agent` (flat fields) | yes | — |
| Data source `claude-managed-agents_agent` | yes | — |
| Resource `claude-managed-agents_environment` | yes (unreleased) | — |
| Data source `claude-managed-agents_environment` | yes (unreleased) | — |
| Resource `claude-managed-agents_vault` | yes (unreleased) | — |
| Data source `claude-managed-agents_vault` | yes (unreleased) | — |
| Resource `claude-managed-agents_vault_credential` | yes (unreleased) | — |
| Data source `claude-managed-agents_vault_credential` | yes (unreleased) | — |
| Resource `claude-managed-agents_memory_store` | yes (unreleased) | — |
| Data source `claude-managed-agents_memory_store` | yes (unreleased) | — |
| Nested blocks on agent (`mcp_servers`, `skills`, `multiagent`, `tools`) | yes (unreleased) | — |
| Data source `claude-managed-agents_agent_version` | yes (unreleased) | — |
| Data source `claude-managed-agents_file` | yes (unreleased) | — |
| Data source for skills | — | follow-up (no API endpoint yet) |

Existing v0.1 agents that have server-side state in `tools`, `mcp_servers`, `skills`, or `multiagent` will see Terraform plan to set them on the next refresh. Adding the matching HCL declaration is a no-op.

## Quickstart

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "~> 0.1"
    }
  }
}

provider "claude-managed-agents" {
  # api_key defaults to the ANTHROPIC_API_KEY environment variable.
}

resource "claude-managed-agents_agent" "coding_assistant" {
  name        = "Coding Assistant"
  model       = "claude-opus-4-7"
  system      = "You are a helpful coding agent."
  description = "Pairs on Go and Terraform tasks."

  metadata = {
    team = "platform"
  }
}

output "agent_id" {
  value = claude-managed-agents_agent.coding_assistant.id
}
```

```sh
export ANTHROPIC_API_KEY="sk-ant-..."
terraform init
terraform apply
```

## Authentication

The provider reads credentials from, in order of precedence:

1. The `api_key` argument on the `provider` block.
2. The `ANTHROPIC_API_KEY` environment variable.

`api_key` is marked `Sensitive` in the schema, so it will not appear in plan diffs. It is still written to state if you set it in HCL — prefer the environment variable in production and CI.

```hcl
provider "claude-managed-agents" {
  api_key     = var.anthropic_api_key   # only set this if env var is impractical
  base_url    = "https://api.anthropic.com"
  max_retries = 3
}
```

## Resources and data sources

| Kind | Name | Docs |
|---|---|---|
| Resource | `claude-managed-agents_agent` | [docs/resources/agent.md](docs/resources/agent.md) |
| Data source | `claude-managed-agents_agent` | [docs/data-sources/agent.md](docs/data-sources/agent.md) |

## Lifecycle gotchas

- **Destroy maps to archive.** The upstream API has no `DELETE /v1/agents/{id}` endpoint. `terraform destroy` issues `POST /v1/agents/{id}/archive`. Archived agents are read-only and cannot be unarchived.
- **`version` is server-managed.** The provider passes it along on update for optimistic concurrency. If you see version conflicts in plans, refresh the state with `terraform apply -refresh-only`.
- **`metadata` is key-level merged.** Removing a key from your HCL causes the provider to send that key with an empty-string value, which the API treats as a delete.

## OpenTofu compatibility

The provider builds on protocol v6 and uses the Plugin Framework. As of v0.2, it requires Terraform 1.11+ or OpenTofu 1.8+ — both engines support TF 1.11 write-only attributes, which the provider uses to handle secrets in `claude-managed-agents_vault_credential`.

## Local development

Pre-requisites: Go 1.23+, Terraform 1.11+ (or OpenTofu 1.8+), `golangci-lint`.

```sh
git clone https://github.com/andasv/terraform-provider-claude-managed-agents
cd terraform-provider-claude-managed-agents
go mod download

make test          # unit tests
make testacc       # acceptance tests, httptest fixture (free)
make testacc-live  # acceptance tests against api.anthropic.com (requires ANTHROPIC_API_KEY)
make lint
make docs          # regenerate docs/ via tfplugindocs

make install       # build + install into ~/.terraform.d/plugins/...
```

Once installed locally, pin the dev version in your test `main.tf`:

```hcl
terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "0.0.1-dev"
    }
  }
}
```

## Versioning

The provider follows [Semantic Versioning](https://semver.org). Pre-1.0 releases reserve the right to make minor breaking changes between minor versions, documented in `CHANGELOG.md`. Post-1.0 will respect semver strictly.

A breaking change is one of:

- Removing or renaming a schema attribute.
- Changing the type of a schema attribute.
- Changing the required/optional/computed status of an attribute in a way that affects existing configurations.
- Changing the default value of an attribute.

## Roadmap

All v0.2 surface (environments, vaults + vault credentials, memory stores, agent nested blocks, agent_version + file data sources) is implemented and unreleased pending tagging.

Sessions, dreams, memory contents/versions, and webhook endpoints are not currently on the roadmap. The skills data source is deferred until upstream documents a REST lookup endpoint. Open an issue if you'd like to discuss.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Distributed under the Mozilla Public License 2.0. See `LICENSE` for the full text (add the file via the GitHub UI's license picker if it's missing from a fresh checkout).

## Disclaimer

This is a community-maintained project. It is not affiliated with Anthropic. "Claude" is a trademark of Anthropic; usage here is descriptive, identifying the API this provider integrates with.
