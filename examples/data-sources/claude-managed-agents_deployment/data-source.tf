terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 0.4"
    }
  }
}

provider "claude-managed-agents" {}

# Look up an existing deployment by id. Useful when the deployment was created
# outside Terraform (via the SDK or console) and you want to read its current
# configuration and observed status.
data "claude-managed-agents_deployment" "nightly_digest" {
  id = "deployment_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "deployment_status" {
  # Observed run state: "active" or "paused".
  value = data.claude-managed-agents_deployment.nightly_digest.status
}

output "deployment_agent_version" {
  # The concrete agent version the deployment pinned.
  value = data.claude-managed-agents_deployment.nightly_digest.agent_version
}

output "deployment_schedule" {
  value = data.claude-managed-agents_deployment.nightly_digest.schedule
}
