# Look up an existing memory store by id. Useful when the store was created
# outside of Terraform and you want to read its description (which surfaces
# in agent system prompts when the store is attached to a session).

data "claude-managed-agents_memory_store" "existing" {
  id = "memstore_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "existing_store_name" {
  value = data.claude-managed-agents_memory_store.existing.name
}

output "existing_store_description" {
  value = data.claude-managed-agents_memory_store.existing.description
}
