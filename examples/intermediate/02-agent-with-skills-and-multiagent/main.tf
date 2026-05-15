terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Worker agent: handles spreadsheet analysis. Includes both kinds of skills:
#   - anthropic: pre-built skills shipped by Anthropic.
#   - custom:    user-uploaded skills referenced by `skill_*` id.
resource "claude-managed-agents_agent" "analyst" {
  name   = "Spreadsheet Analyst"
  model  = "claude-sonnet-4-6"
  system = "Analyze uploaded spreadsheets. Compute summary statistics."

  skills = [
    { type = "anthropic", skill_id = "xlsx" },
    { type = "anthropic", skill_id = "pdf" },
    # Custom skill — replace with a real `skill_*` id from your workspace.
    { type = "custom", skill_id = "skill_01HABCDEF1234567890ABCD", version = "latest" },
  ]
}

# Coordinator agent: delegates to the analyst above and may also invoke
# itself (the `self` member) when no delegation is needed.
resource "claude-managed-agents_agent" "research_lead" {
  name  = "Research Lead"
  model = "claude-opus-4-7"

  multiagent = {
    type = "coordinator"
    agents = [
      { type = "agent", id = claude-managed-agents_agent.analyst.id },
    ]
  }
}

output "analyst_id" {
  value = claude-managed-agents_agent.analyst.id
}

output "research_lead_id" {
  value = claude-managed-agents_agent.research_lead.id
}
