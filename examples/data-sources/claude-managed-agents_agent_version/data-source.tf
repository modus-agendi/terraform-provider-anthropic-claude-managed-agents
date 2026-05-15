terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Look up a specific historical revision of an agent. The upstream API
# only exposes a list endpoint for versions, so the data source pages
# through history and returns the entry whose `version` matches.
#
# Use this to compare the live agent against a known-good baseline, or to
# feed a prior system prompt back into Terraform during a manual rollback.
data "claude-managed-agents_agent_version" "baseline" {
  agent_id = "agent_01HqR2k7vXbZ9mNpL3wYcT8f"
  version  = 3
}

output "baseline_system_prompt" {
  value = data.claude-managed-agents_agent_version.baseline.system
}

output "baseline_model" {
  value = data.claude-managed-agents_agent_version.baseline.model
}
