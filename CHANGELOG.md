# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Resource `claude-managed-agents_skill` for managing custom skill content
  end-to-end via the Skills API beta (`skills-2025-10-02`). Walks a local
  `source_dir`, computes a sha256 content hash combined with an optional
  `version_rotation` counter, and uploads a new immutable version whenever
  the hash changes. The 30 MB upload cap is enforced provider-side so the
  error surfaces before any network call. `display_title` is immutable
  (RequiresReplace) — the Skills API has no PATCH endpoint for it.
  Destroy cascades through every version then deletes the skill itself,
  tolerating 404 at each step.
- Data source `claude-managed-agents_skill` for reading either prebuilt
  Anthropic skills (`xlsx`, `pptx`, `docx`, `pdf`) or custom `skill_*`
  ids. Returns `display_title`, `latest_version`, `source`, and
  `created_at`.
- Sweeper `sweep_skills` matching the `tf-acc-test-` prefix and the
  shared 1-hour age threshold. Cascades through versions before deleting.

### Changed
- **CI**: `live.yml` schedule promoted from weekly (Monday 03:00 UTC) to
  nightly (every day 03:00 UTC). Live-API divergences (the kind that
  caused 17 fixes at the v0.2 cutover) now surface within 24h instead of
  up to 7 days. Sweeper's 1h age threshold keeps cost bounded.

## [0.2.2] - 2026-05-13

### Fixed
- **CI / release**: SLSA provenance reusable workflow must be pinned by
  tag (not commit SHA). The generator introspects its own ref and rejects
  anything that isn't `refs/tags/vX.Y.Z` — its Sigstore builder-identity
  attestation is bound to the tag. Reverted that one `uses:` line to
  `@v2.0.0`. v0.2.0 / v0.2.1 release artifacts shipped fine; v0.2.2
  re-releases the same code with provenance now actually attached.

## [0.2.1] - 2026-05-13

### Fixed
- **CI / release**: SLSA L3 provenance generator job was aborting with
  "Repository is private" on this public repo due to a detection edge case.
  Added `private-repository: true` to the reusable workflow call; the flag
  acknowledges that the repo name appears in the Sigstore public
  transparency log (which is fine for a public repo). v0.2.0 artifacts
  were released correctly (GPG-signed checksums + manifest); only the
  provenance attestation was missing. v0.2.1 re-releases the same code
  with provenance attached.

## [0.2.0] - 2026-05-13

### Fixed
- **Agent multiagent `self` entries**: real API rewrites `{type: "self"}`
  members of `multiagent.agents` to `{type: "agent", id: <parent_agent_id>}`
  on response. The provider now detects this by comparing each entry's id
  to the parent agent's own id and normalizes back to `{type: "self",
  id: null}`, eliminating the "Provider produced inconsistent result"
  error on apply. The L2 fake API (`testutil_test.go`) mirrors the same
  normalization so future regressions are caught without an L3 run.

### Testing
- Added `nodrift_resource_test.go` with one `TestAcc<Name>Resource_noDrift`
  per resource (agent, environment, vault, vault_credential,
  memory_store). Each sweep applies an exhaustive config exercising every
  nullable / list / map / nested block, then re-applies the same config
  with `plancheck.ExpectEmptyPlan` to assert no drift after server-side
  normalization. The multiagent, metadata-merge, environment-flags, and
  empty-string-description divergences caught during the v0.2 live smoke
  run would each have surfaced here in PR CI.

### CI
- `live.yml` now runs weekly (Mondays 03:00 UTC) in addition to
  `workflow_dispatch`. Picks up drift between fake and real API on a
  predictable cadence without burning daily API budget.
- `release.yml` now has a `live` job that runs L3 against the real API
  before `goreleaser` publishes. Releases on `v*` tags only ship if live
  tests pass. Removes the failure mode where a known live-API
  divergence reached the registry.

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

[Unreleased]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/andasv/terraform-provider-claude-managed-agents/releases/tag/v0.1.0
