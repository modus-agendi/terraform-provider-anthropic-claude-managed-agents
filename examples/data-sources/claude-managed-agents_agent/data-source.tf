# Look up an existing agent by id. Useful when the agent was created outside
# of Terraform (e.g. via the SDK) and you want to reference its current state.

data "claude-managed-agents_agent" "existing" {
  id = "agent_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "existing_agent_name" {
  value = data.claude-managed-agents_agent.existing.name
}

output "existing_agent_version" {
  value = data.claude-managed-agents_agent.existing.version
}
