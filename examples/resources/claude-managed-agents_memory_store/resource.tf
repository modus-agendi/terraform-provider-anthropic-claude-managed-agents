terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 0.4"
    }
  }
}

provider "claude-managed-agents" {}

# Default destroy behavior: archive. Preserves the memory-version audit
# trail; the agent can no longer attach the store to new sessions.
# The description is surfaced inside the agent's system prompt, so make it
# informative for both humans and the model.
resource "claude-managed-agents_memory_store" "user_preferences" {
  name        = "User Preferences"
  description = "Per-user product preferences, project context, and prior decisions."
}

# Opt into hard-delete on destroy. Use this for transient stores where the
# audit trail has no value (CI scratch space, ephemeral demos).
resource "claude-managed-agents_memory_store" "ephemeral_scratch" {
  name              = "ephemeral-scratch"
  description       = "Disposable scratch space; safe to hard-delete on teardown."
  delete_on_destroy = true
}

output "user_preferences_id" {
  value = claude-managed-agents_memory_store.user_preferences.id
}
