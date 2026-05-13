<!--
Thanks for contributing. Please fill out the sections below.
Drop sections that don't apply (e.g. the new-resource checklist for a
documentation-only PR).
-->

## Summary

<!-- 1-3 bullet points describing the change and why it's needed. -->

## Type of change

- [ ] Bug fix (non-breaking)
- [ ] Feature (non-breaking)
- [ ] Breaking change (state migration, schema rename, minimum-version bump)
- [ ] Docs / examples / CI only

## Test plan

- [ ] `make test` — unit tests
- [ ] `make testacc` — fake-API acceptance tests
- [ ] `make testacc-live RUN=...` — at least one live smoke if the change
      touches the API surface (optional for docs-only PRs)
- [ ] `make docscheck` — generated docs in sync
- [ ] `make pr` — full local mirror of CI gates

## New-resource checklist

<!-- Skip if this PR does not add a new resource or data source. -->

- [ ] Client method + unit tests in `internal/client/<name>.go`
- [ ] Resource + acceptance tests in `internal/provider/<name>_resource.go`
- [ ] Data source in `internal/provider/<name>_data_source.go`
- [ ] Fake-API handlers in `internal/provider/testutil_test.go`
- [ ] Sweeper in `internal/provider/sweep_test.go` matching `tf-acc-test-` prefix
- [ ] Example in `examples/resources/...` + `examples/data-sources/...`
- [ ] Import script in `examples/resources/<name>/import.sh`
- [ ] CHANGELOG entry under `[Unreleased] / Added`
- [ ] README scope table updated

## Risks / rollback

<!-- What's the worst-case if this merges and breaks something?
     How would a user roll back? -->
