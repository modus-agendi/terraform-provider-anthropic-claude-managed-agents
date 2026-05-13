terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Look up an existing sandbox environment. Useful for reading the current
# networking policy or package list of an environment created via the SDK.
data "claude-managed-agents_environment" "production" {
  id = "env_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "env_name" {
  value = data.claude-managed-agents_environment.production.name
}

output "env_networking_type" {
  value = data.claude-managed-agents_environment.production.config.networking.type
}

output "env_allowed_hosts" {
  value = data.claude-managed-agents_environment.production.config.networking.allowed_hosts
}
