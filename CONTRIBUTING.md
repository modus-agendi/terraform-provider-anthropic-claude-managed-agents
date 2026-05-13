# Contributing

Thanks for your interest in contributing to the unofficial Terraform provider for
Claude Managed Agents. The provider is small and pragmatic — most contributions
will be a focused PR with code, a unit test, and a doc snippet.

## Ground rules

- Open an issue first for anything bigger than a typo fix or a one-line bug fix.
- Match the existing style. No surprise refactors in feature PRs.
- Every new schema attribute needs:
  - `Description` + `MarkdownDescription`
  - an example in `examples/`
  - test coverage in the relevant `*_test.go`

## Local development loop

Run `make` (or `make help`) to see every target with a one-line description.
The categorized output is the source of truth — this section is a primer.

Common workflows:

```sh
# First-time setup — install pinned dev tools (golangci-lint, tfplugindocs,
# tfproviderdocs) into ./bin
make tools

# Iteration loop — runs unit + integration tests with -race
make test

# Run a single test by name
make test RUN=TestDo_429RetriesAndEventuallySucceeds

# Acceptance tests against the in-process fake API
make testacc

# Acceptance tests against api.anthropic.com — needs ANTHROPIC_API_KEY
# (auto-loaded from .env if present)
make testacc-live

# Clean up orphan tf-acc-test-* agents older than 1h
make sweep

# Coverage profile + HTML report (open coverage.html in browser)
make coverage-html

# Run everything CI runs, in order — pass this before pushing
make pr

# Install the provider locally so `terraform init` picks it up
make install
```

`make install` writes the binary to
`~/.terraform.d/plugins/registry.terraform.io/andasv/claude-managed-agents/<version>/<os_arch>/`,
which is where Terraform 1.x looks for filesystem plugins. A `main.tf` that
declares the provider with the same `<version>` will pick it up.

### `.env` for local API keys

The Makefile auto-loads `.env` if it exists in the project root, so
`testacc-live`, `sweep`, and any other target that needs `ANTHROPIC_API_KEY`
just works. Keep the file simple — `KEY=VALUE` per line, no quotes or shell
substitution. `.env` is gitignored.

### Tool versions

`golangci-lint`, `tfplugindocs`, and `tfproviderdocs` versions are pinned at
the top of the Makefile. `make tools` installs those exact versions into
`./bin/`. CI uses the same versions, so a green `make pr` locally means a
green CI run.

### Adding a new command

Don't add raw `go test ...` invocations to the README or your shell history.
Add a Make target:

```makefile
.PHONY: new-thing
new-thing: ## Do the new thing
	go run ./cmd/...
```

The `## comment` is what shows up in `make help`. Keep it under 70 chars.

## Test layers

| Layer | Where | When in CI | Hits real API | Cost |
|---|---|---|---|---|
| **L1 — Unit** | `internal/client/*_test.go` | every PR/push (race + coverage) | no | free |
| **L2 — Integration (httptest)** | `internal/provider/*_test.go` (with `TF_ACC=1`) | every PR/push, TF 1.8/1.9/1.10 matrix | no, uses in-process fake | free |
| **L3 — Live acceptance** | same files (with `TF_ACC_LIVE=1`) | manual `workflow_dispatch` only | yes | per-agent API calls |
| **L4 — Sweeper** | `internal/provider/sweep_test.go` | runs around L3 | yes (archive only) | minimal |

The httptest fixture in `internal/provider/testutil_test.go` keeps an in-memory
map of agents and serves canned JSON responses. The same test cases run
unmodified against the real API when `TF_ACC_LIVE=1` is set.

## Live tests

Live tests are gated to manual dispatch:

```sh
# from anywhere with `gh` configured:
gh workflow run live.yml

# or with sweeper only (no actual tests, just clean up old test agents):
gh workflow run live.yml -f sweep_only=true
```

The live workflow needs `ANTHROPIC_API_KEY` configured as a repo secret. It
runs:

1. **Pre-sweep** — archive agents older than 1 hour whose name starts with
   `tf-acc-test-`.
2. **Acceptance tests** — same suite as the httptest layer, but with the
   fixture bypassed.
3. **Post-sweep** — best-effort cleanup of anything the run created.

If you want a snapshot of what's about to be swept without acting, run
`make sweep` locally with a short threshold:

```sh
ANTHROPIC_API_KEY=... make sweep
```

The sweeper only archives agents whose name starts with `tf-acc-test-` —
real (non-test) agents are never touched.

## Test naming convention

Live mode test resources MUST be named with the `tf-acc-test-` prefix plus
a random suffix. The provided helper does both:

```go
name := testAgentName("basic")   // → tf-acc-test-basic-3f2c (live) or tf-acc-test-basic (fake)
```

Tests that hardcode names like `"Acc Test"` will run only against the fake
fixture; live mode will use `randomTestName` to avoid collisions across
concurrent CI runs.

## Releases

The release workflow is triggered by pushing a tag of the form `vX.Y.Z`. The
maintainer is responsible for:

1. Bumping `CHANGELOG.md` (move items from Unreleased into a new dated section).
2. Tagging: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. Watching the GitHub Action — it imports the GPG key from repo secrets,
   runs GoReleaser, and produces signed artifacts the Terraform/OpenTofu
   registries can ingest.

## Reporting bugs

Open a GitHub issue with:

- Provider version
- Terraform or OpenTofu version
- Minimal `.tf` reproduction
- Output of `TF_LOG=DEBUG terraform apply` with `x-api-key` redacted
- Anything you saw in the Anthropic API response headers (especially
  `request-id`)
