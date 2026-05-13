# Sandbox environment with unrestricted networking and a pip preinstall list.
resource "claude-managed-agents_environment" "python_dev" {
  name = "python-dev"

  config = {
    type = "cloud"

    packages = {
      pip = ["pandas==2.2.0", "numpy==2.0.0"]
    }

    networking = {
      type = "unrestricted"
    }
  }
}

# Sandbox environment with limited networking. The agent may only reach the
# two allowlisted hosts. Package-manager installs at runtime are blocked.
resource "claude-managed-agents_environment" "locked_down" {
  name = "locked-down"

  config = {
    type = "cloud"

    networking = {
      type                   = "limited"
      allowed_hosts          = ["api.example.com", "pypi.org"]
      allow_mcp_servers      = false
      allow_package_managers = false
    }
  }
}

output "python_env_id" {
  value = claude-managed-agents_environment.python_dev.id
}
