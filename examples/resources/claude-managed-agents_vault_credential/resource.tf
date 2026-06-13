terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 1.0"
    }
  }
}

provider "claude-managed-agents" {}

resource "claude-managed-agents_vault" "alice" {
  display_name = "Alice Anderson's API tokens"
}

# Static bearer credential — a fixed API key bound to a specific MCP
# server. Use this for tools that issue long-lived personal access tokens
# (Linear, GitHub fine-grained PATs, etc.).
#
# `token` is WriteOnly: the value is sent on Create/Update but is never
# persisted to state. To rotate the token, set a new value in
# var.linear_token AND increment token_wo_version.
resource "claude-managed-agents_vault_credential" "linear" {
  vault_id     = claude-managed-agents_vault.alice.id
  display_name = "Alice's Linear API key"

  auth = {
    type             = "static_bearer"
    mcp_server_url   = "https://mcp.linear.app/mcp"
    token            = var.linear_token
    token_wo_version = 1
  }
}

# OAuth credential with a refresh block. Anthropic refreshes the access
# token on your behalf using the supplied refresh_token + token_endpoint.
# Every secret-bearing field is WriteOnly and paired with a *_wo_version
# rotation counter.
resource "claude-managed-agents_vault_credential" "slack" {
  vault_id     = claude-managed-agents_vault.alice.id
  display_name = "Alice's Slack workspace"

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
  description = "Linear personal access token."
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
  description = "Slack OAuth client secret."
}
