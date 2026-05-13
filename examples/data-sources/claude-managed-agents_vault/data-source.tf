# Look up an existing vault by id.
data "claude-managed-agents_vault" "alice" {
  id = "vlt_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "alice_metadata" {
  value = data.claude-managed-agents_vault.alice.metadata
}
