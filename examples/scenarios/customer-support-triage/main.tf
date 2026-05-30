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

# Locked-down environment: outbound HTTP only to the three hosts the
# triage workflow actually needs.
resource "claude-managed-agents_environment" "support" {
  name = "customer-support-sandbox"

  config = {
    type = "cloud"

    networking = {
      type = "limited"
      allowed_hosts = [
        "mcp.linear.app",
        "mcp.slack.com",
        "api.example.com",
      ]
      allow_mcp_servers      = true
      allow_package_managers = false
    }
  }
}

# Memory store with the team's playbooks. The description surfaces in the
# agent system prompt, so it should reflect what the model can rely on
# from the store.
resource "claude-managed-agents_memory_store" "playbooks" {
  name        = "Support Playbooks"
  description = "Canonical incident playbooks, runbooks, and SLAs for the support team."
}

# Worker 1: the triage agent. Reads the ticket, assigns severity, and
# (optionally) drafts a Slack message for the on-call.
resource "claude-managed-agents_agent" "triage" {
  name        = "Support Triage Worker"
  model       = "claude-sonnet-4-6"
  system      = "Classify incoming Linear issues by severity. Draft a Slack message for the on-call when severity >= high."
  description = "First-line classifier and notifier for the support inbox."

  metadata = {
    team        = "support"
    environment = "prod"
    role        = "triage"
  }

  mcp_servers = [
    { type = "url", name = "linear", url = "https://mcp.linear.app/mcp" },
    { type = "url", name = "slack", url = "https://mcp.slack.com/mcp" },
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
    {
      type            = "mcp_toolset"
      mcp_server_name = "slack"
      default_config = {
        permission_policy = { type = "always_ask" }
      }
    },
  ]

  skills = [
    { type = "anthropic", skill_id = "pdf" },
  ]
}

# Worker 2: the investigator. Pulls more context from Linear + the
# playbooks store, summarizes for human review.
resource "claude-managed-agents_agent" "investigator" {
  name        = "Support Investigator"
  model       = "claude-opus-4-7"
  system      = "For high-severity tickets, gather related issues, consult playbooks, and produce a short root-cause hypothesis."
  description = "Second-line investigator for high-severity tickets."

  metadata = {
    team        = "support"
    environment = "prod"
    role        = "investigator"
  }

  mcp_servers = [
    { type = "url", name = "linear", url = "https://mcp.linear.app/mcp" },
  ]

  tools = [
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_allow" }
      }
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

# Coordinator: routes per-ticket work between triage and investigator.
resource "claude-managed-agents_agent" "shift_lead" {
  name        = "Support Shift Lead"
  model       = "claude-opus-4-7"
  system      = "Decompose tickets across Triage and Investigator. Synthesize their findings for the on-call summary."
  description = "Coordinator over the triage and investigator workers."

  metadata = {
    team        = "support"
    environment = "prod"
    role        = "coordinator"
  }

  multiagent = {
    type = "coordinator"
    agents = [
      { type = "agent", id = claude-managed-agents_agent.triage.id },
      { type = "agent", id = claude-managed-agents_agent.investigator.id },
    ]
  }
}

# Per-end-user vault for the support engineer running the session. Secrets
# come from variables and are write-only.
resource "claude-managed-agents_vault" "support_engineer" {
  display_name = "Support engineer credentials"

  metadata = {
    external_user_id = "usr_01HABCDEF1234567890ABCD"
    role             = "support"
  }
}

resource "claude-managed-agents_vault_credential" "linear" {
  vault_id     = claude-managed-agents_vault.support_engineer.id
  display_name = "Support engineer Linear key"

  auth = {
    type             = "static_bearer"
    mcp_server_url   = "https://mcp.linear.app/mcp"
    token            = var.linear_token
    token_wo_version = 1
  }
}

resource "claude-managed-agents_vault_credential" "slack" {
  vault_id     = claude-managed-agents_vault.support_engineer.id
  display_name = "Support engineer Slack"

  auth = {
    type                    = "mcp_oauth"
    mcp_server_url          = "https://mcp.slack.com/mcp"
    access_token            = var.slack_access_token
    access_token_wo_version = 1
    expires_at              = "2099-12-31T23:59:59Z"

    refresh = {
      token_endpoint           = "https://slack.com/api/oauth.v2.access"
      client_id                = "1234567890.0987654321"
      scope                    = "channels:read chat:write"
      refresh_token            = var.slack_refresh_token
      refresh_token_wo_version = 1

      token_endpoint_auth = {
        type                     = "client_secret_post"
        client_secret            = var.slack_client_secret
        client_secret_wo_version = 1
      }
    }
  }
}

variable "linear_token" {
  type        = string
  sensitive   = true
  description = "Linear personal access token for the support engineer."
}

variable "slack_access_token" {
  type        = string
  sensitive   = true
  description = "Slack OAuth access token."
}

variable "slack_refresh_token" {
  type        = string
  sensitive   = true
  description = "Slack OAuth refresh token."
}

variable "slack_client_secret" {
  type        = string
  sensitive   = true
  description = "Slack OAuth app client secret."
}

output "shift_lead_id" {
  value = claude-managed-agents_agent.shift_lead.id
}

output "environment_id" {
  value = claude-managed-agents_environment.support.id
}

output "playbooks_id" {
  value = claude-managed-agents_memory_store.playbooks.id
}

output "vault_id" {
  value = claude-managed-agents_vault.support_engineer.id
}
