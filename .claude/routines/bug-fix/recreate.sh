#!/usr/bin/env bash
# Recreate (or update) the bug-fix routine from the files in this directory.
#
# What this script does:
#   1. Reads prompt.md and routine.json from this directory.
#   2. Splices the prompt into the routine.json body and generates a fresh UUID.
#   3. Calls Claude Code in headless mode (`claude -p`) and asks it to
#      use the RemoteTrigger tool to POST the body to claude.ai's routines API.
#
# What this script does NOT do (web-UI-only — see README.md):
#   - Set environment variables (ANTHROPIC_API_KEY) on the Environment.
#   - Install env-setup.sh into the Environment's setup-script field.
#   - Add the `issues.opened` GitHub event trigger.
#   - Generate / rotate the per-routine API bearer token.
#
# Requirements:
#   - `claude` CLI installed and authenticated to the same claude.ai account
#     that owns the routine.
#   - `jq` for JSON manipulation.
#
# Usage:
#   ./recreate.sh                      # creates a NEW routine; prints the trigger_id
#   ./recreate.sh trig_01XYZ...        # UPDATES an existing routine in-place
#
# Exit codes:
#   0  success (routine created/updated)
#   1  prerequisites missing
#   2  body construction failed
#   3  Claude CLI invocation failed

set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMPT_FILE="$DIR/prompt.md"
ROUTINE_JSON="$DIR/routine.json"
TRIGGER_ID="${1:-}"

# ---- prerequisites ---------------------------------------------------------
command -v jq >/dev/null 2>&1 || {
  echo "error: jq is required (brew install jq / apt install jq)" >&2
  exit 1
}
command -v claude >/dev/null 2>&1 || {
  echo "error: claude CLI is required and must be authenticated" >&2
  exit 1
}
[[ -f "$PROMPT_FILE" ]] || { echo "error: missing $PROMPT_FILE" >&2; exit 1; }
[[ -f "$ROUTINE_JSON" ]] || { echo "error: missing $ROUTINE_JSON" >&2; exit 1; }

# ---- construct body --------------------------------------------------------
# Strip _-prefixed comment keys from routine.json (jq doesn't allow comments
# natively; we use leading underscores as a convention). Then splice in the
# prompt text and a fresh UUID.
PROMPT_CONTENT="$(cat "$PROMPT_FILE")"
UUID="$(
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
  else
    python3 -c 'import uuid; print(uuid.uuid4())'
  fi
)"

BODY="$(jq \
  --arg prompt "$PROMPT_CONTENT" \
  --arg uuid "$UUID" '
  walk(
    if type == "object" then
      with_entries(select(.key | startswith("_") | not))
    else . end
  )
  | .job_config.ccr.events[0].data.uuid = $uuid
  | .job_config.ccr.events[0].data.message.content = $prompt
' "$ROUTINE_JSON")" || { echo "error: jq body construction failed" >&2; exit 2; }

if [[ -z "$BODY" ]]; then
  echo "error: empty body after construction" >&2
  exit 2
fi

# ---- invoke Claude CLI -----------------------------------------------------
# RemoteTrigger is a deferred tool — Claude needs to load it via ToolSearch
# before calling it. The instruction text below tells it to do that, then
# pass the body verbatim.
if [[ -n "$TRIGGER_ID" ]]; then
  ACTION="update existing routine with trigger_id=\"$TRIGGER_ID\""
  EXTRA_JSON_HINT=", \"trigger_id\": \"$TRIGGER_ID\""
else
  ACTION="create a NEW routine"
  EXTRA_JSON_HINT=""
fi

INSTRUCTION=$(cat <<INSTR
Load the RemoteTrigger tool via ToolSearch (query: "select:RemoteTrigger"), then $ACTION using exactly this body — do not modify any field, do not add any field, do not "improve" anything. After the call returns, print the routine's id and the next_run_at value on a single line, nothing else.

Body to pass to RemoteTrigger:
$BODY

Hint for the call: action="$( [[ -n "$TRIGGER_ID" ]] && echo update || echo create )"$EXTRA_JSON_HINT.
INSTR
)

echo "==> invoking claude CLI to ${ACTION}..."
echo "$INSTRUCTION" | claude -p --output-format text || {
  echo "" >&2
  echo "error: claude CLI invocation failed" >&2
  echo "" >&2
  echo "fallback: paste the body below into the web UI at https://claude.ai/code/routines" >&2
  echo "" >&2
  echo "$BODY" >&2
  exit 3
}
