terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# A sandbox environment with unrestricted outbound networking.
# Suitable for prototypes; not appropriate for agents that touch
# production data.
#
# Environments are immutable upstream: every config attribute is
# RequiresReplace. Changing the pip list below destroys and re-creates the
# environment.
resource "claude-managed-agents_environment" "prototyping" {
  name = "prototyping-sandbox"

  config = {
    type = "cloud"

    packages = {
      pip = ["requests==2.31.0", "rich==13.7.0"]
    }

    networking = {
      type = "unrestricted"
    }
  }
}

output "env_id" {
  value = claude-managed-agents_environment.prototyping.id
}
