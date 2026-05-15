terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Look up an existing memory store. The description is what gets surfaced
# in the agent's system prompt when the store is attached to a session, so
# read it to verify the model context an existing store provides.
data "claude-managed-agents_memory_store" "user_preferences" {
  id = "memstore_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "store_name" {
  value = data.claude-managed-agents_memory_store.user_preferences.name
}

output "store_description" {
  value = data.claude-managed-agents_memory_store.user_preferences.description
}
