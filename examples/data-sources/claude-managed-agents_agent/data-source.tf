terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Look up an existing agent by id. Useful when the agent was created
# outside of Terraform (via the SDK or console) and you want to reference
# its current state — for example, to drive other resources from its model
# or system prompt.
data "claude-managed-agents_agent" "code_review" {
  id = "agent_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "agent_name" {
  value = data.claude-managed-agents_agent.code_review.name
}

output "agent_model" {
  value = data.claude-managed-agents_agent.code_review.model
}

output "agent_version" {
  value = data.claude-managed-agents_agent.code_review.version
}
