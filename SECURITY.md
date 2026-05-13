# Security Policy

## Supported versions

This is a community-maintained provider. While the public surface is pre-1.0,
only the **latest minor release** receives security fixes. After the v1.0.0 tag,
the policy will shift to "last two minor lines."

| Version | Supported          |
|---------|--------------------|
| 0.2.x   | :white_check_mark: |
| 0.1.x   | :x:                |

## Reporting a vulnerability

**Please do not open public GitHub issues for security problems.**

Report privately via GitHub Security Advisories:
[https://github.com/andasv/terraform-provider-claude-managed-agents/security/advisories/new](https://github.com/andasv/terraform-provider-claude-managed-agents/security/advisories/new).

If that route is unavailable, email `asvirida123@gmail.com` with the subject
line `terraform-provider-claude-managed-agents: SECURITY`.

Include in the report:

- a description of the issue and the impact you can demonstrate,
- a minimal Terraform configuration or Go reproducer,
- the provider version, Terraform/OpenTofu version, and OS you used.

You should receive an acknowledgement within **72 hours**. We aim to ship a
fix or formal mitigation within **30 days** of triage, depending on severity.
Coordinated disclosure: we will credit reporters in the release notes unless
you ask us not to.

## Scope

In scope:

- the provider binary (`terraform-provider-claude-managed-agents`),
- the Go modules under `internal/client/` and `internal/provider/`,
- the release pipeline in `.github/workflows/release.yml`,
- examples and documentation that, if followed verbatim, would lead a user
  into an insecure configuration.

Out of scope:

- bugs in HashiCorp Terraform or OpenTofu themselves,
- bugs in the upstream Anthropic Managed Agents API,
- denial-of-service issues that only affect the local Terraform process,
- security findings already disclosed in a public release note.
