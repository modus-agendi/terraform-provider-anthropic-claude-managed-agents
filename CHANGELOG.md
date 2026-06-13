# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Deployment `schedule.expression` is now validated as a 5-field POSIX cron at
  plan time. A malformed expression or an `@daily`-style shortcut (which the
  API rejects) fails with a clear error instead of an opaque apply-time failure.

### Fixed
- An explicit empty collection on a user-settable optional attribute now fails
  at plan time with a clear, actionable error instead of the cryptic "Provider
  produced inconsistent result after apply" crash (fixes #79). The upstream API
  normalizes an empty list/map to null, which Terraform core cannot represent
  consistently, so the provider rejects the empty form and asks you to omit the
  attribute instead. Applies to `claude-managed-agents_environment`
  `config.networking.allowed_hosts`, `claude-managed-agents_deployment`
  `vault_ids` / `resources` / `metadata`, and `claude-managed-agents_agent` /
  `claude-managed-agents_vault` `metadata`. Omitting the attribute, or providing
  at least one element, both work as before.

### Testing
- Wider edge-case coverage: the previously-missing exhaustive skill no-drift
  sweep, a `vault_credential` 404-on-read (external removal → recreate) test,
  and unit + acceptance tests for the new cron validator.

## [1.0.0] - 2026-06-13

First stable release. The resource and data-source schema is now covered by the
SemVer stability guarantee: breaking changes will happen only in a future major
version (2.0.0), never in a minor or patch. There are no schema changes from
v0.5.0 — this release marks the API surface stable and folds in the pre-1.0
hardening and documentation below.

**Not production software.** Despite the 1.0.0 tag, this remains an
experimental project intended for learning and prototyping, provided **as is
with no warranty** and used **at your own risk**. The version number denotes
API stability, not production-readiness. See the README and LICENSE.

### Security
Pre-1.0.0 hardening. No known exploited issues; these are defense-in-depth
fixes plus a clean `govulncheck`.
- **Skill upload no longer follows symlinks.** A symlink inside a skill
  `source_dir` is now rejected rather than having its target read and
  uploaded. Previously `os.ReadFile` followed symlinks, so a malicious skill
  template could symlink to a file outside the directory (e.g. a private key)
  and exfiltrate it into the uploaded skill.
- **Bounded HTTP attempts.** The API client now sets a 120s per-attempt
  timeout (`retryablehttp` defaulted to none), so a stalled server cannot hang
  `terraform apply` indefinitely.
- **Dependency + toolchain bump.** `golang.org/x/net` → v0.56.0 and the build
  toolchain pinned to go1.26.4, clearing GO-2026-5039, GO-2026-5037, and
  GO-2026-5026. `govulncheck ./...` reports no vulnerabilities.
- **`authorization_token` marked `Sensitive`** (the deployment
  `github_repository` token) for consistency with the vault-credential
  secrets; it was already write-only and never persisted to state.
- **Corrected the README authentication note.** The provider `api_key` is
  never written to Terraform state (provider configuration is not persisted);
  the real exposure of hardcoding it in HCL is config files / version control
  / `TF_LOG` debug output, not state.

### Changed
- All example and documentation `required_providers` version constraints
  bumped from `~> 0.4` to `~> 1.0`.
- Minimum build toolchain is now **go1.26.4** (carries the patched standard
  library; see Security).
- Pre-release tags (`-rc`, `-beta`) now publish as GitHub pre-releases
  (`goreleaser` `prerelease: auto`), so a release candidate no longer takes
  the "latest" badge from the newest stable release.

### Documentation
- Added a prominent **"not for production / learning & prototyping only / use
  at your own risk"** disclaimer to the README and the Terraform Registry
  landing page.
- The provider overview now lists the Deployments resource and its
  `deployment` / `deployment_runs` data sources.

## [0.5.0] - 2026-06-13

### Added
- **L5 behavioral scenarios for deployments.** The scenario harness gained
  three `kind`s (`agent` — the original; `deployment`; `lifecycle`). Deployment
  scenarios fire a deployment via a manual run and judge the resulting
  autonomous session; lifecycle scenarios assert pause/resume. Four scenarios
  ship: an autonomous `user.define_outcome` task (write+verify a CSV), a
  mounted `memory_store` resource, a run-time error-taxonomy path
  (`environment_archived_error`), and a pause/resume lifecycle check. New
  `run_checks` / `lifecycle_checks` / `require_outcome_result` assertions and a
  `client.TriggerDeployment` method back them. (The manual-run endpoint
  `POST /v1/deployments/{id}/run` is undocumented; confirmed by live probe.)
- **`claude-managed-agents_deployment` resource** plus
  `claude-managed-agents_deployment` and `claude-managed-agents_deployment_runs`
  data sources, covering the Deployments beta API
  (`managed-agents-2026-04-01`). A deployment binds an agent to an
  environment, vaults, mounted resources, initial events, and an optional
  cron schedule.
  - Pause/resume is modelled with a writable `desired_status`
    (`active` / `paused`) decoupled from the observed `status`, so an
    automatic error-pause does not cause Terraform to fight the API.
    Inspect `paused_reason` (a typed discriminated union), fix the cause,
    then re-apply to resume.
  - `initial_events` (1-50) supports the `user.message`, `system.message`,
    and `user.define_outcome` variants; editing it forces replacement (the
    API does not patch events in place).
  - `resources` (max 500) supports `github_repository`, `file`, and
    `memory_store` mounts. The github `authorization_token` is a TF 1.11
    write-only attribute — never stored in state; bump
    `authorization_token_wo_version` to re-send it on rotation.
  - Destroy archives the deployment (one-way; the API has no DELETE).
    Updates are last-write-wins `PATCH` (no optimistic-concurrency version).
  - The `deployment_runs` data source lists append-only run audit records
    with filters (`deployment_id`, `trigger_type`, `has_error`) and surfaces
    the typed run-error taxonomy.
- **Deployments documentation.** A narrative
  [Deployments guide](docs/guides/deployments.md) (Terraform Registry "Guides"
  tab) covering scheduled vs manual runs, the `desired_status`/`status` split,
  resource mounts, run history, and the caveats (no Terraform-level trigger,
  undocumented manual-run endpoint, 1-hour minimum cadence, replace-on-events).
  A worked
  [`advanced/05-scheduled-deployment-with-outcome`](examples/advanced/05-scheduled-deployment-with-outcome)
  example, and a README "Testing & quality" section documenting the five test
  layers and the live-validated L5 cost.

### Fixed
- Corrected the `claude-managed-agents_deployment` `id` attribute description:
  ids use the `depl_…` prefix (confirmed against the live API), not
  `deployment_…` as the schema text previously stated. Docs-only.

## [0.4.1] - 2026-05-30

### Fixed
- Sweepers now run for every registered resource type (agent, environment,
  memory_store, vault, skill) in CI workflows and the `make sweep` target.
  Previously only `claude-managed-agents_agent` was swept, letting test-only
  memory stores accumulate until the per-org 200-store cap blocked the live
  and scenarios cron runs.
- `sweepMemoryStores` now hard-deletes orphan stores instead of archiving
  them. Archived memory stores still count against the 200-store cap, so
  archive alone never freed quota. The sweeper also lists with
  `include_archived=true` so previously-archived orphans become visible.

### Changed
- **Breaking: registry namespace moved to `modus-agendi`.** The GitHub
  repository and Terraform Registry listing moved from the `andasv`
  namespace to `modus-agendi`. Update your `required_providers` source:

  ```hcl
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"  # was: andasv/anthropic-claude-managed-agents
      version = "~> 0.4"
    }
  }
  ```

  The local provider name (`claude-managed-agents`) and all resource type
  names are **unchanged** — only the registry source string moves. The Go
  module path is now
  `github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents`
  (affects vendored consumers only).
- Live and scenarios cron schedules moved from daily to weekly (Mondays
  03:00 / 04:00 UTC).
- Registry landing page (`docs/index.md`) rewritten via a new
  `templates/index.md.tmpl` — provider overview, what it manages,
  authentication, requirements, and repository links — so the Terraform
  Registry shows more than a one-line description. All example
  `required_providers` version constraints normalized to `~> 0.4` (the
  floor after the v0.4.0 registry-slug change).

## [0.4.0] - 2026-05-15

**Breaking: registry slug renamed.** The provider is now published as
`andasv/anthropic-claude-managed-agents` (was
`andasv/claude-managed-agents`). The original slug was lost when the
underlying GitHub repository was renamed to
`terraform-provider-anthropic-claude-managed-agents`; the Terraform
Registry did not follow the rename and removed the old listing.

### User migration

Update your `required_providers` block:

```hcl
required_providers {
  claude-managed-agents = {
    source  = "andasv/anthropic-claude-managed-agents"  # was: andasv/claude-managed-agents
    version = "~> 0.4"
  }
}
```

The local provider name (`claude-managed-agents`) and all resource
type names (`claude-managed-agents_agent`, etc.) are **unchanged**.
Only the registry source string moves. Resource definitions in your
HCL do not need to change.

### Changed
- **Module path**: `github.com/andasv/terraform-provider-claude-managed-agents`
  → `github.com/andasv/terraform-provider-anthropic-claude-managed-agents`.
  Affects vendored consumers only.
- **Binary name**: `terraform-provider-claude-managed-agents` →
  `terraform-provider-anthropic-claude-managed-agents`. Goreleaser
  artifacts are renamed accordingly.
- **Provider Address constant** in `main.go` updated to the new
  `registry.terraform.io/andasv/anthropic-claude-managed-agents` path.
- **Examples, docs, README**: all `source = "andasv/claude-managed-agents"`
  references updated to the new slug.
- **Pre-0.4 GitHub releases were deleted** as part of this cutover.
  They referenced the old (now-orphaned) registry slug and would have
  caused confusion. Sources for those versions remain available via
  the git tags `v0.1.0` through `v0.3.2`.

### Inherits from the unpublished v0.3.3
- L5 Fibonacci scenario aligned with what `agent_toolset_20260401`
  actually exposes — `bash` is the execution path, not the
  nonexistent `code_execution` tool. Trajectory check is now
  `require_tool_use_named: bash` (deterministic).
- Judge response parser tolerates reasoning prose before the JSON
  verdict; extracts the first balanced `{...}` object from the body
  with awareness of string literals.

## [0.3.3] - 2026-05-14

This release supersedes the unpublished v0.3.2 tag (release artifacts
never shipped because the L5 Fibonacci gate failed; the underlying
issues are fixed here).

### Fixed
- **L5 Fibonacci scenario**: aligned the rubric and trajectory check
  with what the bundled toolset actually exposes.
  `agent_toolset_20260401` does NOT include a `code_execution` tool —
  the bundled set is `bash`, `read`, `write`, `edit`, `glob`, `grep`,
  `web_fetch`, `web_search`. The previous rubric required a tool that
  could never be invoked. The agent's only execution path is via
  `bash` (write the code file with `write`, run it with `bash python
  fib.py`); the scenario now requires that exact sequence via
  `require_tool_use_named: bash` (deterministic) instead of the lax
  `require_event: agent.tool_use`.
- **Judge response parser** (`internal/client/judge.go`): the parser
  was strict about JSON-only output. Judge models occasionally lead
  with reasoning prose before the JSON verdict despite the
  system-prompt instruction. The parser now extracts the first
  balanced `{...}` JSON object from the response body, with awareness
  of string literals so a `}` inside a value doesn't unbalance.
  Backward-compatible: pure-JSON responses parse identically.

## [0.3.2] - 2026-05-14

No provider behavior changes since v0.3.1. This release exists primarily
to probe whether the Terraform Registry refreshes the cached provider
description (currently empty since v0.1.0) on a new version publish.

### Changed
- **Repo metadata**: GitHub description now enumerates the covered
  resources (agents, MCP servers, skills, vaults, memory stores) and
  the TF 1.11 write-only secret handling, instead of a single-line
  summary. Topics extended to include `claude-api` and `hashicorp`
  (14 topics total).
- **Docs**: refreshed `SECURITY.md` supported-versions matrix to
  reflect v0.3.x being the only supported line. Added
  `internal/scenarios/` to the in-scope list since L5 scenarios
  execute real inference.
- **Docs**: `CONTRIBUTING.md` test-layer table now includes the L5
  row, the CI matrix is corrected to TF 1.11/1.12/latest, and the
  live-tests section is split between `live.yml` (L3 CRUD) and
  `scenarios.yml` (L5 behavioral) with their actual triggers and
  release-gate role.
- **README**: added an "Anthropic Terraform provider — what this
  covers" section near the top that enumerates each resource and
  clarifies the scope is the Managed Agents API (not the Messages
  API). Helps the Terraform Registry and GitHub search land readers
  on the relevant section.

## [0.3.1] - 2026-05-14

No provider behavior changes since v0.3.0. This release packages
documentation and contributor-tooling updates only; upgrading from
v0.3.0 is a no-op for users.

### Changed
- **Docs**: `README.md` no longer carries the v0.1 / v0.2 retrospective
  table. Status section now shows only the current shipped surface;
  per-release history continues to live in this file.

### Internal
- **Routines**: codified the cloud Claude Code bug-fix routine config
  under `.claude/routines/bug-fix/` (prompt, env-setup script,
  recreate script, run-book). `make routine-sync` re-syncs the cloud
  routine from the persisted prompt + JSON. Tool versions in the
  env-setup script now match the Makefile pins
  (`golangci-lint v1.62.0`, `tfplugindocs v0.20.1`,
  `tfproviderdocs v0.12.1`).

## [0.3.0] - 2026-05-14

### Added
- **L5 behavioral test layer**. New `internal/scenarios/` package that
  loads YAML-defined scenarios, provisions agents via Terraform, opens
  real sessions against the Anthropic API, captures event trajectories,
  and grades final answers via a separate `/v1/messages` LLM-as-judge
  call. Gated behind `TF_ACC_SCENARIOS=1`; runs manually, nightly via
  cron, and as a release gate.
- L5 scenario `fibonacci_default_toolset` — agent with the default
  Anthropic toolset computes the 10th Fibonacci number; passes if 55
  appears in the response and the trajectory shows a tool was used.
- L5 scenario `multi_capability_research` — exercises five advanced
  capabilities in one session: public MCP (DeepWiki), custom skill,
  multiagent dispatch (researcher + verifier sub-agents), rare-pkg
  environment (`pip: [tabulate]`), and an auto-attached memory store.
  Costs ~$0.50/run; ~$0.60/run nightly when combined with Fibonacci.
- Cost reporter on the L5 suite. Prints a per-scenario summary table
  with token breakdown, an aggregate, and a USD estimate keyed off a
  local `pricing.go` table. Cache reads are priced at 10% of input,
  cache writes at 125%, matching Anthropic's published rates.
- YAML loader substitution: `${SCENARIO_DIR}` in `terraform_config`
  resolves to the absolute path of the YAML's directory at load time,
  letting scenarios reference fixture dirs (e.g. skill `source_dir`)
  portably across machines.
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
- Client transport for the Sessions, Session Events, and Judge endpoints
  (`internal/client/session.go`, `internal/client/judge.go`). Used by
  the L5 harness; not exposed as Terraform resources because session
  lifecycle is per-test, not infrastructure-managed.
- `SessionCreateRequest.Resources` for attaching memory stores at session
  create-time. The upstream API does not support attach-on-the-fly; the
  L5 runner auto-discovers every `claude-managed-agents_memory_store.*`
  resource in Terraform state and threads it through.

### Changed
- **CI**: `live.yml` schedule promoted from weekly (Monday 03:00 UTC) to
  nightly (every day 03:00 UTC). Live-API divergences (the kind that
  caused 17 fixes at the v0.2 cutover) now surface within 24h instead of
  up to 7 days. Sweeper's 1h age threshold keeps cost bounded.
- **CI**: Release workflow (`release.yml`) now blocks goreleaser on both
  `live` (L3) and `scenarios` (L5) passing. A hotfix bypass is available
  via `workflow_dispatch.inputs.skip_scenarios=true`.
- **Repo merge strategy**: default switched from squash to rebase-merge
  so individual commits land on `main` with their original messages
  intact. Squash and merge-commit are disabled at the repo level.

### Fixed
- Skill multipart uploads now wrap files under a top-level folder named
  after `display_title`, matching the live API's expected shape
  (`<folder>/SKILL.md` rather than bare `SKILL.md`). The fake-API test
  fixture did not enforce the same shape, hiding the divergence until
  the L5 scenario first exercised the skill resource against
  api.anthropic.com. Fixes #31, #32.
- `ListSessionEvents` now paginates by `created_at[gt]` timestamp
  instead of an `after=<event_id>` cursor. The upstream events
  endpoint never accepted `after`; the bug surfaced the first time
  the L5 polling loop ran live.

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

[Unreleased]: https://github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/compare/v0.5.0...v1.0.0
[0.5.0]: https://github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/compare/v0.4.0...v0.4.1
[0.2.2]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/andasv/terraform-provider-claude-managed-agents/releases/tag/v0.1.0
