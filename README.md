# terraform-provider-anthropic-claude-managed-agents

[![CI](https://github.com/andasv/terraform-provider-anthropic-claude-managed-agents/actions/workflows/test.yml/badge.svg)](https://github.com/andasv/terraform-provider-anthropic-claude-managed-agents/actions/workflows/test.yml)
[![Release](https://github.com/andasv/terraform-provider-anthropic-claude-managed-agents/actions/workflows/release.yml/badge.svg)](https://github.com/andasv/terraform-provider-anthropic-claude-managed-agents/actions/workflows/release.yml)
[![codecov](https://codecov.io/gh/andasv/terraform-provider-anthropic-claude-managed-agents/graph/badge.svg)](https://codecov.io/gh/andasv/terraform-provider-anthropic-claude-managed-agents)
[![License: MPL 2.0](https://img.shields.io/badge/license-MPL%202.0-blue.svg)](https://www.mozilla.org/MPL/2.0/)

A Terraform / OpenTofu provider for **Anthropic Claude Managed Agents** — declaratively manage agents (with tools, MCP servers, skills, and multiagent coordination), environments, vaults, custom skills, and memory stores from Terraform configurations.

> **Unofficial.** This is a community project. It is not maintained by, endorsed by, or affiliated with Anthropic. Upstream API: [Claude Managed Agents docs](https://platform.claude.com/docs/en/managed-agents).

---

### Anthropic Terraform provider — what this covers

A community-maintained Terraform / OpenTofu provider for the **Anthropic Claude Managed Agents** API. Resources and data sources:

- **Agents** — with nested HCL for `tools` (`agent_toolset_20260401`, `mcp_toolset`, `custom`), `mcp_servers`, `skills`, and `multiagent` coordination
- **Environments** — cloud and dedicated, with `apt` / `npm` / `pip` package install
- **Vaults** and **vault credentials** — secrets handled via Terraform 1.11 write-only attributes (not persisted to state)
- **Custom skills** — multipart upload, content-hash drift detection, immutable versions
- **Memory stores** — persistent agent memory, attached at session time

The Managed Agents API is distinct from the standard Anthropic Messages API. This provider does not manage the Messages API or generic `/v1/messages` calls.

---

## What this provider does

Claude Managed Agents is Anthropic's hosted runtime for long-running, tool-using agents. The platform exposes a REST API for creating and configuring those agents and their supporting resources (environments, vaults, memory stores).

This provider lets you put that configuration under Terraform so you can:

- Review changes in pull requests instead of via ad-hoc API calls.
- Version-control your agent definitions alongside the rest of your infra.
- Promote agents across workspaces with the same workflow you use for everything else.

## Status / scope

| Capability | Status |
|---|---|
| Resource + data source `claude-managed-agents_agent` (flat fields: name, model, system, description, metadata) | shipped |
| Agent nested HCL: `tools` (`agent_toolset_20260401`, `mcp_toolset`, `custom`), `mcp_servers`, `skills`, `multiagent` | shipped |
| Resource + data source `claude-managed-agents_environment` | shipped |
| Resource + data source `claude-managed-agents_vault` | shipped |
| Resource + data source `claude-managed-agents_vault_credential` (TF 1.11 write-only secrets) | shipped |
| Resource + data source `claude-managed-agents_memory_store` | shipped |
| Resource `claude-managed-agents_skill` (multipart upload, content-hash drift detection, 30 MB cap) | shipped |
| Data source `claude-managed-agents_skill` (prebuilt + custom) | shipped |
| Data source `claude-managed-agents_agent_version` | shipped |
| Data source `claude-managed-agents_file` | shipped |

For per-release change history (including deprecations and breaking changes), see [CHANGELOG.md](CHANGELOG.md).

## Quickstart

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.3"
    }
  }
}
```

> **Don't rename `claude-managed-agents` to something shorter.** That string is the prefix on every resource type (`claude-managed-agents_agent`, `claude-managed-agents_vault`, …). If you alias it (e.g. to `anthropic`), every resource block must add an explicit `provider = claude-managed-agents` argument — noisier than keeping the literal name.

```hcl
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

`claude-managed-agents_vault_credential` uses Terraform 1.11 write-only attributes for its secret fields (`token`, `access_token`, `refresh_token`, `client_secret`): these values are never persisted to state. Rotate by incrementing the matching `*_wo_version` integer.

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
| Resource | `claude-managed-agents_environment` | [docs/resources/environment.md](docs/resources/environment.md) |
| Resource | `claude-managed-agents_memory_store` | [docs/resources/memory_store.md](docs/resources/memory_store.md) |
| Resource | `claude-managed-agents_skill` | [docs/resources/skill.md](docs/resources/skill.md) |
| Resource | `claude-managed-agents_vault` | [docs/resources/vault.md](docs/resources/vault.md) |
| Resource | `claude-managed-agents_vault_credential` | [docs/resources/vault_credential.md](docs/resources/vault_credential.md) |
| Data source | `claude-managed-agents_agent` | [docs/data-sources/agent.md](docs/data-sources/agent.md) |
| Data source | `claude-managed-agents_agent_version` | [docs/data-sources/agent_version.md](docs/data-sources/agent_version.md) |
| Data source | `claude-managed-agents_environment` | [docs/data-sources/environment.md](docs/data-sources/environment.md) |
| Data source | `claude-managed-agents_file` | [docs/data-sources/file.md](docs/data-sources/file.md) |
| Data source | `claude-managed-agents_memory_store` | [docs/data-sources/memory_store.md](docs/data-sources/memory_store.md) |
| Data source | `claude-managed-agents_skill` | [docs/data-sources/skill.md](docs/data-sources/skill.md) |
| Data source | `claude-managed-agents_vault` | [docs/data-sources/vault.md](docs/data-sources/vault.md) |
| Data source | `claude-managed-agents_vault_credential` | [docs/data-sources/vault_credential.md](docs/data-sources/vault_credential.md) |

## Lifecycle gotchas

- **Agent destroy maps to archive.** The upstream API has no `DELETE /v1/agents/{id}` endpoint. `terraform destroy` issues `POST /v1/agents/{id}/archive`. Archived agents are read-only and cannot be unarchived.
- **`version` is server-managed.** The provider passes it along on update for optimistic concurrency. If you see version conflicts in plans, refresh the state with `terraform apply -refresh-only`.
- **`metadata` is key-level merged.** The upstream API uses merge semantics: removing a key from your HCL causes the provider to send JSON null for that key, which the API treats as a delete. Applies to both `claude-managed-agents_agent.metadata` and `claude-managed-agents_vault.metadata`.
- **Environments are immutable.** The API has no environment update endpoint, so every attribute on `claude-managed-agents_environment` is `ForceNew`. Any change triggers replacement. Destroy issues `DELETE /v1/environments/{id}` and falls back to archive if the API returns 409 (active session reference).
- **Vault and memory_store destroy archives by default.** Set `delete_on_destroy = true` on the resource to hard-delete instead. Vault archive cascades through credentials; memory_store hard-delete cascades through memories and memory versions.
- **Vault-credential secrets are write-only.** Secret fields are never read back from the API. Rotate by incrementing the corresponding `token_wo_version`, `access_token_wo_version`, `refresh_token_wo_version`, or `client_secret_wo_version` integer. `auth.type`, `auth.mcp_server_url`, `auth.refresh.token_endpoint`, and `auth.refresh.client_id` are immutable (RequiresReplace).

## OpenTofu compatibility

The provider builds on protocol v6 and uses the Plugin Framework. It requires Terraform 1.11+ or OpenTofu 1.8+ — both engines support TF 1.11 write-only attributes, which the provider uses to handle secrets in `claude-managed-agents_vault_credential`.

## Local development

Pre-requisites: Go 1.23+, Terraform 1.11+ (or OpenTofu 1.8+), `golangci-lint`.

```sh
git clone https://github.com/andasv/terraform-provider-anthropic-claude-managed-agents
cd terraform-provider-anthropic-claude-managed-agents
go mod download

make test            # unit tests
make testacc         # acceptance tests, httptest fixture (free)
make testacc-live    # acceptance tests against api.anthropic.com (requires ANTHROPIC_API_KEY)

# Behavioral scenarios (L5) — opens real sessions and grades them via an
# LLM-as-judge call. Bills real inference tokens; see
# internal/scenarios/README.md for cost notes.
TF_ACC_SCENARIOS=1 go test ./internal/scenarios/... -count=1 -timeout 10m -v

make lint
make docs            # regenerate docs/ via tfplugindocs
make coverage-html   # full coverage profile + HTML report
make coverage-split  # mirrors CI: split unit + acceptance profiles
make pr              # full local PR-check pipeline (mirrors CI)

make install         # build + install into ~/.terraform.d/plugins/...
```

Once installed locally, pin the dev version in your test `main.tf`:

```hcl
terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "0.0.1-dev"
    }
  }
}
```

> **Gotcha — `make install` shadows registry releases.** `make install`
> copies the binary into `~/.terraform.d/plugins/registry.terraform.io/andasv/anthropic-claude-managed-agents/`,
> which Terraform treats as the authoritative version source. Any
> external config that pins a different version (e.g. `version = "~> 0.2"`)
> will fail `terraform init` with "no available releases match the given
> constraints". Run `rm -rf ~/.terraform.d/plugins/registry.terraform.io/andasv`
> to drop back to registry-pulled releases.

## Versioning

The provider follows [Semantic Versioning](https://semver.org). Pre-1.0 releases reserve the right to make minor breaking changes between minor versions, documented in `CHANGELOG.md`. Post-1.0 will respect semver strictly.

A breaking change is one of:

- Removing or renaming a schema attribute.
- Changing the type of a schema attribute.
- Changing the required/optional/computed status of an attribute in a way that affects existing configurations.
- Changing the default value of an attribute.

## Roadmap

Under consideration for the next minor: an ephemeral resource for vault-credential validation, optional skill version retention attributes, and broadening the L5 behavioral scenario catalog. Sessions, dreams, memory contents/versions, and webhook endpoints are not currently on the roadmap. Open an issue if you'd like to discuss.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Community

- [SECURITY.md](SECURITY.md) — how to report vulnerabilities privately.
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — expected conduct for contributors and maintainers.
- [Issue templates](.github/ISSUE_TEMPLATE) — structured bug reports and feature requests.
- [Pull request template](.github/PULL_REQUEST_TEMPLATE.md) — checklist for opening a PR.
- [CODEOWNERS](.github/CODEOWNERS) — review routing.

## License

Distributed under the Mozilla Public License 2.0. See `LICENSE` for the full text (add the file via the GitHub UI's license picker if it's missing from a fresh checkout).

## Disclaimer

This is a community-maintained project. It is not affiliated with Anthropic. "Claude" is a trademark of Anthropic; usage here is descriptive, identifying the API this provider integrates with.
