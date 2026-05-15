terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.3"
    }
  }
}

provider "claude-managed-agents" {}

# Manage a custom skill from a local directory. The directory must contain
# a `SKILL.md` at its root; sibling files (templates, prompts, code
# snippets) are bundled into the same multipart upload.
resource "claude-managed-agents_skill" "report_builder" {
  display_title = "report-builder"
  source_dir    = "${path.module}/skill-content"
}

# Cross-reference the skill from an agent. `latest_version` auto-rolls
# every time the provider uploads a new version; pin to a specific epoch
# string instead if you want immutable behavior.
resource "claude-managed-agents_agent" "writer" {
  name  = "writer"
  model = "claude-opus-4-7"

  skills = [
    {
      type     = "custom"
      skill_id = claude-managed-agents_skill.report_builder.id
      version  = claude-managed-agents_skill.report_builder.latest_version
    },
  ]
}

# Force a new version even when files are unchanged by bumping
# `version_rotation`. Useful to invalidate a downstream cache.
resource "claude-managed-agents_skill" "report_builder_rotating" {
  display_title    = "report-builder-rotating"
  source_dir       = "${path.module}/skill-content"
  version_rotation = 2
}

output "report_builder_id" {
  value = claude-managed-agents_skill.report_builder.id
}

output "report_builder_latest_version" {
  value = claude-managed-agents_skill.report_builder.latest_version
}
