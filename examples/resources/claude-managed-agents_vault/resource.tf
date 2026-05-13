# A vault models one end-user's set of MCP credentials. Tag it with metadata
# that maps it back to your own user records.
resource "claude-managed-agents_vault" "alice" {
  display_name = "Alice"

  metadata = {
    external_user_id = "usr_abc123"
    cohort           = "beta"
  }
}

output "alice_vault_id" {
  value = claude-managed-agents_vault.alice.id
}
