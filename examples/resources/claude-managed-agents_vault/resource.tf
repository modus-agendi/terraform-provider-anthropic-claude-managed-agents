terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# A vault models one end-user's set of MCP credentials. Tag it with metadata
# that maps it back to your own user records so support workflows can
# correlate Anthropic-side audit logs with your customer database.
resource "claude-managed-agents_vault" "alice" {
  display_name = "Alice Anderson's API tokens"

  metadata = {
    external_user_id = "usr_01HABCDEF1234567890ABCD"
    cohort           = "beta"
  }
}

# Default destroy archives the vault (cascades to credentials but preserves
# the audit record). Set delete_on_destroy = true on a vault that holds
# only test data.
resource "claude-managed-agents_vault" "ci_test" {
  display_name      = "CI test fixtures"
  delete_on_destroy = true
}

output "alice_vault_id" {
  value = claude-managed-agents_vault.alice.id
}
