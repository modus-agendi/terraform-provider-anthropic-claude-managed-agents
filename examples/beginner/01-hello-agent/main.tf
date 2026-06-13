terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 1.0"
    }
  }
}

provider "claude-managed-agents" {}

# The smallest possible agent: name + model. Everything else is optional.
resource "claude-managed-agents_agent" "hello" {
  name  = "Hello Agent"
  model = "claude-haiku-4-5-20251001"
}

output "agent_id" {
  value = claude-managed-agents_agent.hello.id
}

output "agent_version" {
  value = claude-managed-agents_agent.hello.version
}
