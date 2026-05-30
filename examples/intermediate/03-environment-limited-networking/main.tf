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

# Locked-down cloud environment:
#   - allowed_hosts is the explicit egress allowlist (bare hostnames, no
#     scheme; the upstream API rejects URL-shaped values).
#   - allow_mcp_servers=true keeps MCP outbound traffic available even
#     though general egress is restricted.
#   - allow_package_managers=true is required for the npm preinstall list
#     below to actually run.
resource "claude-managed-agents_environment" "node_locked" {
  name = "node-locked"

  config = {
    type = "cloud"

    packages = {
      npm = ["typescript@5.4.5", "tsx@4.7.2"]
    }

    networking = {
      type = "limited"
      allowed_hosts = [
        "registry.npmjs.org",
        "api.example.com",
      ]
      allow_mcp_servers      = true
      allow_package_managers = true
    }
  }
}

output "env_id" {
  value = claude-managed-agents_environment.node_locked.id
}
