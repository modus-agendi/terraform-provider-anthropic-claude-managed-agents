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
make test          # unit tests only
make testacc       # acceptance tests against the in-process httptest server
make testacc-live  # acceptance tests against api.anthropic.com (needs ANTHROPIC_API_KEY)
make lint          # golangci-lint
make docs          # regenerate docs/ via tfplugindocs
```

`make install` writes the binary to
`~/.terraform.d/plugins/registry.terraform.io/andasv/claude-managed-agents/<version>/<os_arch>/`,
which is where Terraform 1.x looks for filesystem plugins. A `main.tf` that
declares the provider with the same `<version>` will pick it up.

## Test layers

| Layer | When it runs | Cost |
|---|---|---|
| Unit (`go test ./internal/client/...`) | Always | Free |
| Acceptance, httptest mode (`TF_ACC=1`) | CI on every PR | Free |
| Acceptance, live mode (`TF_ACC=1 TF_ACC_LIVE=1`) | Manually, before release tags | Real API calls |

The httptest fixture in `internal/provider/testutil_test.go` keeps an in-memory
map of agents and serves canned JSON responses. The same test cases run
unmodified against the real API when `TF_ACC_LIVE=1` is set.

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
