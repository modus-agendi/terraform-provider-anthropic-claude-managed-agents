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

```sh
make build         # compile the provider binary
make install       # drop it into your local plugin cache so terraform finds it
make test          # unit + integration tests with -race
make testacc       # acceptance tests against the in-process httptest server
make testacc-live  # acceptance tests against api.anthropic.com (needs ANTHROPIC_API_KEY)
make sweep         # archive orphan test agents (needs ANTHROPIC_API_KEY)
make coverage      # generate coverage.out
make coverage-html # open-able coverage.html
make lint          # golangci-lint (broad ruleset)
make docs          # regenerate docs/ via tfplugindocs
make docs-check    # validate doc structure (tfproviderdocs)
```

`make install` writes the binary to
`~/.terraform.d/plugins/registry.terraform.io/andasv/claude-managed-agents/<version>/<os_arch>/`,
which is where Terraform 1.x looks for filesystem plugins. A `main.tf` that
declares the provider with the same `<version>` will pick it up.

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
