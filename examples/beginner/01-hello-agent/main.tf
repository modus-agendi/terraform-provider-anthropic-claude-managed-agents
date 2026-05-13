terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "~> 0.2"
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
