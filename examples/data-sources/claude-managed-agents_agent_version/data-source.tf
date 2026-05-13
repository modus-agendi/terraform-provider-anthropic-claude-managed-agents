# Look up a specific historical version of an agent. Useful for comparing
# the live agent to a known-good baseline, or feeding the prior config back
# into terraform when manually rolling back.

data "claude-managed-agents_agent_version" "v3" {
  agent_id = claude-managed-agents_agent.coding_assistant.id
  version  = 3
}

output "v3_system_prompt" {
  value = data.claude-managed-agents_agent_version.v3.system
}
