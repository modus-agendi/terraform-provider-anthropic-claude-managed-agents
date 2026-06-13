terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 1.0"
    }
  }
}

provider "claude-managed-agents" {}

# Restricted environment: the agent may only reach the explicit hosts
# listed below, and it cannot run package managers at session runtime.
resource "claude-managed-agents_environment" "production" {
  name = "support-locked-down"

  config = {
    type = "cloud"

    networking = {
      type                   = "limited"
      allowed_hosts          = ["mcp.linear.app", "api.example.com"]
      allow_mcp_servers      = true
      allow_package_managers = false
    }
  }
}

# Per-end-user vault. The vault tags itself with the external user id so
# audit logs can be correlated with your customer database.
resource "claude-managed-agents_vault" "end_user" {
  display_name = "End-user API tokens"

  metadata = {
    external_user_id = "usr_01HABCDEF1234567890ABCD"
  }
}

resource "claude-managed-agents_vault_credential" "linear" {
  vault_id     = claude-managed-agents_vault.end_user.id
  display_name = "End-user Linear API key"

  auth = {
    type             = "static_bearer"
    mcp_server_url   = "https://mcp.linear.app/mcp"
    token            = var.linear_token
    token_wo_version = 1
  }
}

# The agent itself: bound to the Linear MCP server and the bundled
# toolset, with web_fetch disabled because outbound HTTP should go
# through MCP (which is governed by the vault credentials).
resource "claude-managed-agents_agent" "support_agent" {
  name        = "Support Triage with Linear access"
  model       = "claude-opus-4-7"
  system      = "Look up Linear tickets and summarize them for the support team."
  description = "Production support triage with vault-managed Linear access."

  metadata = {
    team        = "support"
    environment = "prod"
  }

  mcp_servers = [
    {
      type = "url"
      name = "linear"
      url  = "https://mcp.linear.app/mcp"
    },
  ]

  tools = [
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_allow" }
      }
      configs = [
        { name = "web_fetch", enabled = false },
      ]
    },
    {
      type            = "mcp_toolset"
      mcp_server_name = "linear"
      default_config = {
        permission_policy = { type = "always_ask" }
      }
    },
  ]
}

variable "linear_token" {
  type        = string
  sensitive   = true
  description = "Linear personal access token used by the end-user vault."
}

output "agent_id" {
  value = claude-managed-agents_agent.support_agent.id
}

output "environment_id" {
  value = claude-managed-agents_environment.production.id
}

output "vault_id" {
  value = claude-managed-agents_vault.end_user.id
}
