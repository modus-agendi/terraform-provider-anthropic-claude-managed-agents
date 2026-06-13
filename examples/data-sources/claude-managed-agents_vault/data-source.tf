terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 1.0"
    }
  }
}

provider "claude-managed-agents" {}

# Look up an existing vault by id. Most useful for cross-referencing
# externally-managed user records via the vault's metadata map.
data "claude-managed-agents_vault" "alice" {
  id = "vlt_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "vault_display_name" {
  value = data.claude-managed-agents_vault.alice.display_name
}

output "vault_metadata" {
  value = data.claude-managed-agents_vault.alice.metadata
}
