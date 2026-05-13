# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **Agent multiagent `self` entries**: real API rewrites `{type: "self"}`
  members of `multiagent.agents` to `{type: "agent", id: <parent_agent_id>}`
  on response. The provider now detects this by comparing each entry's id
  to the parent agent's own id and normalizes back to `{type: "self",
  id: null}`, eliminating the "Provider produced inconsistent result"
  error on apply. The L2 fake API (`testutil_test.go`) mirrors the same
  normalization so future regressions are caught without an L3 run.

### Fixed (live-API divergences caught by full smoke run)
- **Environment**: provider no longer sends `allow_mcp_servers` /
  `allow_package_managers` when `networking.type = "unrestricted"`. Real
  API rejects them; the `false` defaults from the schema were forcing
  them on every request.
- **Environment**: `allowed_hosts` documented as requiring bare hostnames
  (no URL schemes) — the upstream API rejects entries containing `://`.
- **Environment**: `packages` returned non-null from the API even when
  unset; normalized in the read mapping so plans stay clean.
- **Memory store**: API returns `description: ""` (empty string) when
  not set; provider now treats empty-string the same as null in state.
- **Vault / Agent metadata**: real API uses merge semantics rather than
  full-replace. Provider now sends JSON null for keys that should be
  deleted and string values for keys that should be set or updated.
  Metadata payload types changed from `map[string]string` to
  `map[string]any` in `AgentUpdateRequest` and `VaultUpdateRequest`.
- Test-only: `claude-managed-agents_vault_credential` id regex relaxed
  to match either `cred_*` (fake) or `vcrd_*` (real). `agent_version`
  fake added so `agent_version` data source tests are deterministic.

### Added
- Data source `claude-managed-agents_agent_version` — look up a specific
  historical version of an agent by id + version number. The upstream
  API only exposes a list endpoint, so this data source pages through
  the version history and filters.
- Data source `claude-managed-agents_file` — read file metadata
  (filename, size_bytes, mime_type, scope_id, created_at). Binary
  content download is not modeled.
- `claude-managed-agents_agent` resource and data source now expose three
  of the four nested-config fields as first-class HCL:
  - `mcp_servers` (list of `{type, name, url}`) — MCP server roster.
  - `skills` (list of `{type, skill_id, version}`) — both `anthropic`
    pre-built skills and `custom` user-uploaded skills.
  - `multiagent` (single nested `{type, agents}`) — coordinator config
    with `agent` and `self` member types.
- Existing v0.1 agents that have server-side state in these fields will
  see Terraform plan to set them on the next refresh. Adding the matching
  HCL declaration is a no-op.

### Added (tools as first-class HCL)
- `claude-managed-agents_agent.tools` is now first-class HCL covering all
  three variants:
  - `agent_toolset_20260401` — the bundled Anthropic toolset.
  - `mcp_toolset` — exposes an MCP server's tools, bound via
    `mcp_server_name` to an entry of `mcp_servers`.
  - `custom` — user-defined tools with `name`, `description`, and a
    JSON-encoded `input_schema`.
- `default_config` + `configs[*]` carry per-toolset and per-tool overrides
  (`enabled`, `permission_policy.type` = `always_allow` | `always_ask`).
  These fields are user-controlled: the provider preserves the HCL-declared
  value in state and ignores API-side enrichment to keep plans clean.
- The data source `claude-managed-agents_agent` exposes the same fields
  as Computed.
- Previously-skipped live tests (`mcp_servers`, nested-all-at-once) now
  pass against the real API since `tools[mcp_toolset]` is configurable.

### Changed (BREAKING)
- Minimum Terraform raised to **1.11** (OpenTofu 1.8). The provider now uses
  TF 1.11 write-only attributes for the four secret-bearing fields on
  `claude-managed-agents_vault_credential` (`token`, `access_token`,
  `refresh_token`, `client_secret`). Users on TF 1.8-1.10 should stay on
  v0.1.x; upgrading without bumping the engine will fail at plan time.

### Added
- Resource `claude-managed-agents_vault` for end-user-scoped credential
  containers. Mutable `display_name` and `metadata`; destroy archives by
  default and hard-deletes when `delete_on_destroy = true`. Archive
  cascades through credentials server-side.
- Data source `claude-managed-agents_vault`.
- Resource `claude-managed-agents_vault_credential` with both auth types
  (`static_bearer`, `mcp_oauth`). Secret fields are TF 1.11 write-only;
  rotate by incrementing the matching `*_wo_version` integer. `auth.type`,
  `auth.mcp_server_url`, `auth.refresh.token_endpoint`, and
  `auth.refresh.client_id` are immutable (RequiresReplace).
- Data source `claude-managed-agents_vault_credential` (secret fields are
  never populated; the API does not return them).
- Sweeper for `claude-managed-agents_vault` matching the `tf-acc-test-`
  display-name prefix and the shared 1-hour age threshold. Vault archive
  cascades through credentials, so no separate credential sweeper is
  needed.
- Resource `claude-managed-agents_memory_store` for managing persistent
  memory stores. Supports `name` and `description` (both mutable) plus a
  `delete_on_destroy` provider-side flag. Destroy archives the store by
  default; setting `delete_on_destroy = true` hard-deletes the store and
  cascades through every memory and memory version.
- Data source `claude-managed-agents_memory_store`.
- Sweeper for `claude-managed-agents_memory_store` matching the
  `tf-acc-test-` prefix and the shared 1-hour age threshold.
- Resource `claude-managed-agents_environment` for managing sandbox
  environments (cloud config, package preinstall lists, and
  unrestricted/limited networking policies). All attributes require
  replacement on change: the upstream API has no environment update
  endpoint.
- Data source `claude-managed-agents_environment` for reading existing
  environments by id.
- `destroy` on `claude-managed-agents_environment` issues
  `DELETE /v1/environments/{id}` first and falls back to
  `POST /archive` if the API returns 409 (active session reference).
- Sweeper for `claude-managed-agents_environment` that matches the
  `tf-acc-test-` prefix and the 1-hour age threshold shared with the
  agent sweeper.

## [0.1.0] - 2026-05-13

### Added
- Initial scaffold of the provider.
- Resource `claude-managed-agents_agent` (flat fields: `name`, `model`,
  `system`, `description`, `metadata`).
- Data source `claude-managed-agents_agent`.
- API client with retry, typed errors, and request ID capture.
- Unit and acceptance test suites (acceptance tests run against an in-process
  httptest server by default; live API runs are opt-in via `TF_ACC_LIVE=1`).
- GitHub Actions workflows for CI and release.

[Unreleased]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/andasv/terraform-provider-claude-managed-agents/releases/tag/v0.1.0
