terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# A memory store is a workspace-scoped collection of text documents that
# persists across sessions. The description is surfaced inside the agent's
# system prompt when the store is attached, so make it informative.
resource "claude-managed-agents_memory_store" "project_notes" {
  name        = "Project Notes"
  description = "Decisions, ADRs, and rolling notes for the platform team."
}

# Hard-delete on destroy is opt-in. The default (false) archives the store
# so the memory-version audit trail is preserved.
resource "claude-managed-agents_memory_store" "scratch" {
  name              = "scratch"
  description       = "Disposable space; not worth auditing."
  delete_on_destroy = true
}

output "project_notes_id" {
  value = claude-managed-agents_memory_store.project_notes.id
}
