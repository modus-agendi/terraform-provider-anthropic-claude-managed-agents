# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
