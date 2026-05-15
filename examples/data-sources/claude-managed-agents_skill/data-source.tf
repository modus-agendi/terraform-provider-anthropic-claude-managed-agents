terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.3"
    }
  }
}

provider "claude-managed-agents" {}

# Reference a prebuilt Anthropic skill. Prebuilt skills cannot be managed
# as a resource (the API does not allow create/delete on them), but the
# data source resolves their current latest_version so an agent can pin
# to it.
data "claude-managed-agents_skill" "xlsx" {
  skill_id = "xlsx"
}

# Reference a custom skill by its `skill_*` id. Use this when the skill is
# owned by another module or when you only need read-only access.
data "claude-managed-agents_skill" "report_builder" {
  skill_id = "skill_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "xlsx_latest_version" {
  value = data.claude-managed-agents_skill.xlsx.latest_version
}

output "report_builder_source" {
  value = data.claude-managed-agents_skill.report_builder.source
}
