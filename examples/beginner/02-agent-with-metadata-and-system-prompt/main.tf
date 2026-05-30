terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 0.4"
    }
  }
}

provider "claude-managed-agents" {}

# Adds three optional fields to the minimal agent:
#   - system: the prompt the model receives on every turn.
#   - description: free-form text shown in the agent listing UI.
#   - metadata: key/value labels for cross-referencing your own systems.
resource "claude-managed-agents_agent" "support_triage" {
  name        = "Customer Support Triage"
  model       = "claude-sonnet-4-6"
  system      = "Classify incoming tickets by urgency (low/medium/high) and route to the matching queue."
  description = "First-line triage for the support inbox."

  metadata = {
    team        = "support"
    environment = "prod"
    owner       = "andrea@example.com"
  }
}

output "agent_id" {
  value = claude-managed-agents_agent.support_triage.id
}
