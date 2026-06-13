terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 1.0"
    }
  }
}

provider "claude-managed-agents" {}

# Unrestricted networking with a pip preinstall list. Suitable for
# exploratory data work where any outbound host is fair game.
# Environments are immutable: every attribute is RequiresReplace.
resource "claude-managed-agents_environment" "data_science" {
  name = "data-science-sandbox"

  config = {
    type = "cloud"

    packages = {
      pip = ["pandas==2.2.0", "numpy==2.0.0", "scikit-learn==1.4.0"]
    }

    networking = {
      type = "unrestricted"
    }
  }
}

# Locked-down environment: the agent may only reach explicit hosts and may
# not call MCP servers or run package-manager installs at session runtime.
# Use this for agents that touch production data.
resource "claude-managed-agents_environment" "production" {
  name = "production-locked-down"

  config = {
    type = "cloud"

    networking = {
      type                   = "limited"
      allowed_hosts          = ["api.example.com", "internal.example.com"]
      allow_mcp_servers      = false
      allow_package_managers = false
    }
  }
}

output "data_science_env_id" {
  value = claude-managed-agents_environment.data_science.id
}
