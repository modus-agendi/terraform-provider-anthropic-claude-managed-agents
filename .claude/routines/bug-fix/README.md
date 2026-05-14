# Bug fix routine — operations runbook

This directory persists the configuration of the **Bug fix routine** that runs
in Anthropic's cloud against this repository. The routine itself lives in the
maintainer's claude.ai account (not in this repo) — these files are the
versioned, reviewable source of truth for what it does, so it can be
recreated or migrated.

## What this routine does

Each scheduled tick (and on-demand via `Run now`):

1. Lists open issues labeled `bug` with no assignee.
2. Picks up to 5, oldest first, and clusters related ones.
3. For each cluster: self-assigns to `andasv`, branches, makes a surgical
   fix, runs the project's `make test` / `make lint` / `make docs` gates
   (plus L3 `TF_ACC_LIVE=1 make testacc` when `internal/provider/` is
   touched and `ANTHROPIC_API_KEY` is set in the environment), pushes, and
   opens a PR with `Fixes #N` trailers.
4. On any gate failure: comments the reason on each clustered issue and
   un-assigns so a human can take over.

The full instruction text is in [`prompt.md`](./prompt.md). Edit that file
when you want to change the routine's behavior — it is the source of truth.

## Files

| File | Purpose |
|---|---|
| `prompt.md` | The instruction text the routine runs every tick. Edit here, then re-sync. |
| `routine.json` | API-recreatable config: name, cron, repo, model, allowed tools. Comments use `_`-prefixed keys (stripped at deploy time). |
| `env-setup.sh` | The setup script that needs to be pasted into the routine's cloud Environment. Installs Go, Terraform, golangci-lint, tfplugindocs, tfproviderdocs. |
| `recreate.sh` | Re-syncs `prompt.md` + `routine.json` into the cloud routine via the Claude Code CLI. |

## What this directory does NOT capture

Some routine state lives in the cloud only and cannot be expressed in code
today:

- **Environment variables** (e.g., `ANTHROPIC_API_KEY` for the L3 gate)
- **The Environment's setup-script field** (the cached install of tooling)
- **GitHub event triggers** (e.g., `issues.opened`)
- **The per-routine API bearer token** (shown once, can't be re-fetched)

All of those are configured via the web UI. The "First-time setup" and
"Migration" sections below document every click.

---

## First-time setup (run-book for a new maintainer)

You need this once per claude.ai account that will own the routine.

1. **Generate or reuse a Claude API key** with adequate quota.
   - https://console.anthropic.com → API keys.
   - Keep it handy; you'll paste it into the Environment in step 4.

2. **Create the routine via the CLI** (or run `recreate.sh` if the files
   in this directory are up to date):
   ```bash
   make routine-sync
   ```
   Note the `trigger_id` printed at the end — it's needed for in-place
   updates later.

3. **Open the routine in the web UI** at
   https://claude.ai/code/routines and click the routine's row.

4. **Configure the Environment** (click the Environment → Edit):
   - Add env var: `ANTHROPIC_API_KEY=<your-key>` from step 1.
   - Paste the contents of [`env-setup.sh`](./env-setup.sh) into the
     "Setup script" field. The cloud caches the result, so this only
     re-runs when the environment is rebuilt.

5. **Trim the MCP connectors** down to what the routine actually needs
   (probably none). Every connector your claude.ai account has is
   attached by default and bloats every run's context window.

6. **(Optional) Add a GitHub event trigger** in addition to the cron:
   - In the routine's edit form, scroll to "Select a trigger".
   - Click "Add another trigger" → GitHub.
   - Install the Claude GitHub App on this repository when prompted.
     `/web-setup` from the CLI grants clone access only; the event trigger
     requires the separate App install.
   - Event: `issues.opened`. Filter: `label:bug`.
   - This makes the routine react to new bug reports within seconds, in
     addition to the daily sweep.

7. **Test with "Run now"**. Open the session in the web UI and watch the
   gate output. Most first-run failures are setup-script issues
   (missing tools); fix `env-setup.sh` and re-paste.

8. **(Optional) Generate an API bearer token** if you want to fire the
   routine from CI or external tooling. Web UI only; **store the token
   immediately — it cannot be re-fetched after the modal closes.**

---

## Updating the routine

To change behavior:

1. Edit `prompt.md` (the source of truth for what the routine does), or
   `routine.json` (for schedule, model, tools, etc.). Submit a PR.
2. After merge, re-sync:
   ```bash
   make routine-sync TRIGGER_ID=trig_01XYZ...
   ```
   With `TRIGGER_ID` set, the existing routine is updated in place.
   Without it, a NEW routine is created (so you'd have two — be careful).

Environment changes (env vars, setup script, GitHub triggers) require web
UI edits — re-walk steps 4–6 above when they change.

---

## Migrating to a different account

If the routine needs to move to a different claude.ai account (new
maintainer, account rotation, etc.):

1. **Old account**: optionally delete the routine via the web UI to avoid
   double-runs. (There is no CLI delete.)
2. **New account**: walk through "First-time setup" from step 1. The repo
   files mean the routine's behavior is identical on the new account.
3. **External callers**: regenerate the API bearer token (step 8) and
   rotate it in any external system that calls the `/fire` endpoint.

---

## Why this hybrid pattern

There is no native "routines as code" today — no Terraform provider, no
Anthropic-blessed `routine.yaml` schema. The pieces above are the
consensus best practice as of 2026-05:

- **Version the prompt** in `prompt.md` so behavior changes go through PR
  review and are recoverable from git history.
- **Capture API-recreatable config** in `routine.json` so a single command
  re-syncs the cloud state.
- **Document web-UI-only steps** in this README so they're not lost when
  the maintainer or the account changes.

If Anthropic ships an official "routine-as-code" mechanism, migrate to it
and delete this directory.

---

## Related

- Anthropic's routines documentation:
  https://code.claude.com/docs/en/routines
- The routines API reference (`/fire` endpoint):
  https://platform.claude.com/docs/en/api/claude-code/routines-fire
- Project conventions: [`../../../CLAUDE.md`](../../../CLAUDE.md)
