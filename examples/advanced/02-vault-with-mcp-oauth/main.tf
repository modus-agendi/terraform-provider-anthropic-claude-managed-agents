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

# One vault per end-user. Metadata maps the vault back to your customer
# database so support workflows can correlate Anthropic-side audit logs
# with your internal user records.
resource "claude-managed-agents_vault" "alice" {
  display_name = "Alice Anderson's API tokens"

  metadata = {
    external_user_id = "usr_01HABCDEF1234567890ABCD"
    cohort           = "beta"
  }
}

# Static bearer credential. `token` is WriteOnly: the value is sent to the
# API but never persisted to Terraform state. To rotate, update the
# variable value AND increment token_wo_version.
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

# OAuth credential with refresh. Anthropic uses the refresh_token to mint
# fresh access tokens on the agent's behalf when the current one expires.
# Every secret-bearing field is WriteOnly + paired with a rotation counter.
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
  description = "Slack OAuth app client secret."
}

output "vault_id" {
  value = claude-managed-agents_vault.alice.id
}
