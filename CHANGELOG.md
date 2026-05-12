# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial scaffold of the provider.
- Resource `claude-managed-agents_agent` (flat fields: `name`, `model`,
  `system`, `description`, `metadata`).
- Data source `claude-managed-agents_agent`.
- API client with retry, typed errors, and request ID capture.
- Unit and acceptance test suites (acceptance tests run against an in-process
  httptest server by default; live API runs are opt-in via `TF_ACC_LIVE=1`).
- GitHub Actions workflows for CI and release.

[Unreleased]: https://github.com/andasv/terraform-provider-claude-managed-agents/compare/HEAD...HEAD
