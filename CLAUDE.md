# Project conventions — terraform-provider-anthropic-claude-managed-agents

Instructions for Claude when working in this repo. The goal: stay consistent
with decisions already locked in, avoid repeating the gotchas we hit during
v0.1, and keep the testing and code-quality bar steady as the surface area
grows.

---

## Mission

Unofficial community Terraform / OpenTofu provider for Anthropic Claude
Managed Agents (REST API at `https://api.anthropic.com/v1/agents` etc.).
Targets the HashiCorp Terraform Registry and OpenTofu Registry. Registry
source: `andasv/anthropic-claude-managed-agents`. License: MPL 2.0.

---

## Coverage target

**85% total coverage is the target. Don't aim higher.**

| Metric | Floor | Target | When to push above |
|---|---|---|---|
| Project coverage | 70% (codecov gate) | **85%** | never — diminishing returns kick in |
| Patch coverage | 75% (codecov gate) | 80%+ | never — same as above |

Coverage **is not the goal.** It is a gap-finder. Use `make coverage-html` to
locate uncovered branches you care about, then write a targeted test. Do not
write tests just to lift the number.

**Leave uncovered:**
- Framework `Configure` boilerplate (nil-ProviderData and wrong-type branches)
- `crypto/rand.Read` panic paths
- `json.Marshal` error paths on values that cannot fail to marshal (strings, ints)
- Initialization branches reachable only with malformed framework inputs

**Always cover:**
- Every public method in `internal/client/`
- Every CRUD branch on every resource (Create / Read / Update / Delete / Import)
- 404 → state-removal on Read
- 404 → silent success on Delete
- Optimistic-concurrency conflict on Update
- Every nullable field's null-clear path
- Every list/map field's add / remove / modify path
- All `ExpectError` paths for invalid configs

---

## Test layer model

Five layers. **Do not collapse them.**

| Layer | Where | Trigger | Real API |
|---|---|---|---|
| **L1 Unit** | `internal/client/*_test.go` | every PR, `-race` + coverage | no |
| **L2 Integration (httptest)** | `internal/provider/*_test.go` (`TF_ACC=1`) | every PR, TF 1.8/1.9/1.10 matrix | no — in-process fake API |
| **L3 Live acceptance** | same provider tests (`TF_ACC_LIVE=1`) | manual `workflow_dispatch` + weekly cron (Mon 03:00 UTC) | yes |
| **L4 Sweeper** | `internal/provider/sweep_test.go` | runs around L3 | yes (archive only) |
| **L5 Behavioral scenarios** | `internal/scenarios/*_test.go` (`TF_ACC_SCENARIOS=1`) | manual + weekly cron (Mon 04:00 UTC) + release gate | yes (sessions, full tool use, LLM-as-judge) |

The fake API lives in `internal/provider/testutil_test.go` and intentionally
diverges from real Anthropic only where the real shape is unspecified in the
docs. When the real API behavior changes, the fake must change with it —
treat L3 as the canary.

L5 catches **behavioral** regressions — does the agent we provision actually
perform tasks? Scenarios live as YAML files under
`internal/scenarios/scenarios/`. Each scenario provisions resources via
Terraform, opens a session against the real Anthropic API, captures the full
event trajectory, asserts deterministic properties on it, then feeds the
final answer to a separate `/v1/messages` call ("LLM as judge") that returns
a structured PASS/FAIL verdict. Cost is real ($0.05–$0.20 per scenario
depending on tool-use count). Triggered manually, weekly via cron, and as
a release gate — `release.yml` blocks goreleaser on a scenario FAIL unless
`workflow_dispatch.inputs.skip_scenarios=true`.

---

## Naming conventions

### Provider local name in HCL: `claude-managed-agents` (dashes only)

Terraform rejects underscores in provider local names. This was the v0.1
naming pivot — original plan said `claude_managed_agents`, Terraform refused
it at init time. Resource types accept dashes too, so:

```hcl
provider "claude-managed-agents" {}
resource "claude-managed-agents_agent" "x" {}
```

This is unusual (most providers use a single-word local name like `aws`,
`google`, `azurerm`). It is intentional: the verbose form prevents
brand-confusion with hypothetical future Anthropic-affiliated providers.

### Live-mode test resource naming: `tf-acc-test-<purpose>-<random8>`

Use `testAgentName("purpose")` in `acceptance_helpers_test.go`. In fake-API
mode the name is stable (`tf-acc-test-purpose`); in live mode it's random.
The sweeper matches the `tf-acc-test-` prefix and archives anything older
than 1 hour.

### Module path: `github.com/andasv/terraform-provider-anthropic-claude-managed-agents`

NOT `asvirida/...` — that was the original plan, but the active gh account
is `andasv`. The whole codebase was renamed in v0.1.

---

## When adding a new resource (checklist)

Roughly in this order:

1. **Read the upstream API docs** at `/Users/andreisvirida/dev/education/anthropic/managed_agents/docs/managed-agents/` (start at `INDEX.md`).
2. **Write the client method** in `internal/client/<name>.go` mirroring `agent.go`:
   - Request struct (Create/Update split if shapes differ)
   - Response struct in `types.go`
   - Methods: `Create`, `Get`, `Update`, `Archive`, `Delete` (whichever the API supports), `List`
3. **Write client unit tests** in `<name>_test.go` covering happy path + 401/403/404/409/429/500 + retry + cancellation + malformed body + pagination if applicable. Aim for ≥85% on the file.
4. **Write the resource** in `internal/provider/<name>_resource.go`:
   - Schema with `Description` AND `MarkdownDescription` on every attribute
   - `Create` / `Read` / `Update` / `Delete` / `ImportState`
   - Optimistic concurrency via a `version` field if the API supports it
   - Empty maps/lists normalized to null in `helpers.go`-style helpers
   - `lifecycle.ignore_changes` guidance in MarkdownDescription for any
     fields that drift legitimately
5. **Write the data source** in `<name>_data_source.go` — strip the
   Create/Update/Delete; reuse the same model + `*FromAPI` helper.
6. **Add fake-API handlers** to `testutil_test.go` (Create/Get/Update/Archive/Delete routes).
7. **Write acceptance tests** in `<name>_resource_test.go`:
   - Basic create + verify computed fields
   - Update each mutable scalar individually
   - Clear nullable fields (set to null)
   - Add/modify/remove map keys (if metadata-like)
   - `ImportStateVerify`
   - Drift refresh (out-of-band server edit via `MutateAgent`-style helper)
   - Destroy with already-missing resource (404 → silent)
   - `ExpectError` for at least one invalid config
   - At least one `plancheck.ExpectEmptyPlan` post-apply
   - At least one `statecheck.ExpectKnownValue` for a typed assertion
8. **Add a no-drift sweep** in `nodrift_resource_test.go`:
   - `TestAcc<Name>Resource_noDrift` exercising every nullable, every
     list/map, every nested block in a single exhaustive config.
   - Uses the shared `runNoDrift(t, buildCfg)` helper which applies the
     config and asserts `plancheck.ExpectEmptyPlan` on a second apply.
   - The test must NOT skip in live mode — the whole point is to make
     fake and real API agree on round-trip shape.
   - When the fake passes but the live API surfaces drift, the bug is in
     `testutil_test.go` (fake too permissive). Add the missing
     server-side normalization there so the fake fails in the same way
     the real API does.
9. **Add the sweeper** in `sweep_test.go` matching the prefix convention.
10. **Add examples** in `examples/resources/claude-managed-agents_<name>/` and
    `examples/data-sources/claude-managed-agents_<name>/`.
11. **Run `make docs`** to regenerate doc pages. Commit them.
12. **Add an entry** to the CHANGELOG under `[Unreleased] / Added`.
13. **Bump the README scope table** — mark the new resource as `yes` instead
    of `Planned`.

---

## Lessons learned (do not repeat)

### Local provider names cannot have underscores

Terraform refuses to init a provider whose local name in `required_providers`
contains underscores. Use dashes. See "Naming conventions" above.

### plugin-testing builds source as `<HOST>/<NAMESPACE>/<factory_key>`

`testAccProtoV6ProviderFactories` map keys are NOT full source addresses.
`plugin-testing` prepends `registry.terraform.io/hashicorp/` to whatever
key you give it, then uses that as the source in an auto-injected
`required_providers` block. To override the namespace, set
`TF_ACC_PROVIDER_NAMESPACE` env var (we do this in
`testutil_test.go:init()`).

### Empty maps from the API trigger "provider produced inconsistent result"

If `metadata` is `null` in the plan and the API returns `{}` (empty), and
the provider sets state to an empty `types.Map`, Terraform errors. Fix:
normalize empty maps to null in `stringMapToMap` (`helpers.go`).

### Server-side nested fields must round-trip via raw JSON

The agent resource has `tools`, `mcp_servers`, `skills`, `multiagent` server
fields that v0.1 does NOT expose as HCL. The client struct keeps them as
`json.RawMessage` and the resource preserves them through Update. Do not
drop them — users who set them via the API directly would lose them on
every `terraform apply`.

### Do not inline LICENSE text

`Write`-ing the full MPL 2.0 (or other long standard licenses) trips the
content filter. Add the LICENSE file via the GitHub web UI's license picker
or `curl https://www.mozilla.org/media/MPL/2.0/index.txt -o LICENSE`. The
README references MPL 2.0 by name; that's enough.

### Sweeper age threshold prevents racing live tests

Sweeper only archives agents older than 1 hour. Without this, parallel CI
runs would archive each other's in-progress test resources. If you ever
parallelize live tests within a single run, raise this floor.

### `tfproviderdocs` doesn't take `-providers-prefix`

The flag does not exist (despite what some tutorials say). Just
`-provider-name claude-managed-agents` is enough.

### Codecov split flags

The `test.yml` workflow uploads unit and acceptance coverage as separate
Codecov reports via `flags: unit` and `flags: acceptance`. Keep them split
so it's obvious which layer dropped when coverage regresses.

### Always create new commits, never amend pushed history

The PR-and-feature-branch workflow makes amending tempting (cleaner
history). Don't. Fix-forward with a new commit. Saves all the rebasing-
gotchas when someone has the branch checked out.

---

## Bias toward …

- **Make targets, not raw shell.** Every developer-facing command lives in
  the Makefile with a `## comment` so `make help` is the discovery surface.
  CI calls the same targets — local + CI parity. Don't paste `go test …` in
  CONTRIBUTING.md or the README; add a Make target instead. Tooling
  versions are pinned at the top of the Makefile, installed by `make tools`
  into `./bin/`.
- **Simple HTTP and stdlib.** Hand-rolled `net/http` via `retryablehttp`
  beats any code-gen client for a single-purpose provider. Resist adding a
  full SDK.
- **Tests against shapes the user sees.** `statecheck.ExpectKnownValue` and
  `plancheck.ExpectEmptyPlan` test what the user observes. Prefer them over
  `resource.TestCheckResourceAttr` when the assertion is meaningful.
- **Documented gotchas in `MarkdownDescription`.** Every attribute that has
  surprising behavior (destroy → archive, metadata key-level merge,
  null-clears) explains it inline.
- **One PR, one concern.** Don't bundle "add resource X" with "tighten
  golangci config". Reviewers can reason about diffs more easily that way.

## Bias against …

- **Schema fields the API doesn't surface.** Don't add fake validation
  attributes hoping for niceness. Surface API errors as-is.
- **Conditional logic in tests.** If you need an `if liveMode()` branch
  inside a test step, split it into two test functions.
- **Pre-1.0 churn that breaks state.** Every breaking change before v1.0 is
  noted in CHANGELOG. After v1.0, breaks happen only at major bumps.

---

## Reference paths

- Upstream API docs (local mirror): `/Users/andreisvirida/dev/education/anthropic/managed_agents/docs/managed-agents/` → start at `INDEX.md`
- Project plans archive: `.claude/plans/` (gitignored, local only)
- API reference distilled for this provider: `.claude/plans/api-reference.md`
