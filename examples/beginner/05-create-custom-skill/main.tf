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

# Upload a custom skill from a local directory. The provider walks every
# file under `source_dir`, validates that `SKILL.md` exists at the top
# level, computes a deterministic sha256 of the contents, and uploads the
# bundle via the Skills API. Editing any file under skill-content/ on
# disk and re-running `terraform apply` triggers a new immutable version.
resource "claude-managed-agents_skill" "hello" {
  display_title = "hello-skill"
  source_dir    = "${path.module}/skill-content"
}

output "skill_id" {
  description = "The skill_* identifier returned by the API."
  value       = claude-managed-agents_skill.hello.id
}

output "latest_version" {
  description = "The current version (epoch-timestamp string)."
  value       = claude-managed-agents_skill.hello.latest_version
}

output "content_hash" {
  description = "sha256 of the canonicalized skill contents."
  value       = claude-managed-agents_skill.hello.content_hash
}
