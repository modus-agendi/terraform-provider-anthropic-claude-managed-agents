# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
