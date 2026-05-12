resource "claude-managed-agents_agent" "coding_assistant" {
  name        = "Coding Assistant"
  model       = "claude-opus-4-7"
  system      = "You are a helpful coding agent. Be concise. Cite filenames."
  description = "Pairs on Go and Terraform tasks."

  metadata = {
    team        = "platform"
    environment = "prod"
  }
}

output "agent_id" {
  value = claude-managed-agents_agent.coding_assistant.id
}

output "agent_version" {
  value = claude-managed-agents_agent.coding_assistant.version
}
