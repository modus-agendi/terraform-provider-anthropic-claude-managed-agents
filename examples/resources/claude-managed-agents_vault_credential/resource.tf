# Static bearer credential — a fixed API key bound to a specific MCP server.
resource "claude-managed-agents_vault_credential" "linear" {
  vault_id     = claude-managed-agents_vault.alice.id
  display_name = "Alice's Linear API key"

  auth = {
    type             = "static_bearer"
    mcp_server_url   = "https://mcp.linear.app/mcp"
    token            = var.linear_token        # WriteOnly — never stored in state
    token_wo_version = 1                       # increment to rotate the token
  }
}

# OAuth credential — access token + refresh block. Anthropic refreshes the
# access token on your behalf when it expires.
resource "claude-managed-agents_vault_credential" "slack" {
  vault_id     = claude-managed-agents_vault.alice.id
  display_name = "Alice's Slack"

  auth = {
    type                    = "mcp_oauth"
    mcp_server_url          = "https://mcp.slack.com/mcp"
    access_token            = var.slack_access_token      # WriteOnly
    access_token_wo_version = 1
    expires_at              = "2099-12-31T23:59:59Z"

    refresh = {
      token_endpoint           = "https://slack.com/api/oauth.v2.access"
      client_id                = "1234567890.0987654321"
      scope                    = "channels:read chat:write"
      refresh_token            = var.slack_refresh_token  # WriteOnly
      refresh_token_wo_version = 1

      token_endpoint_auth = {
        type                     = "client_secret_post"
        client_secret            = var.slack_client_secret # WriteOnly
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
  type      = string
  sensitive = true
}

variable "slack_refresh_token" {
  type      = string
  sensitive = true
}

variable "slack_client_secret" {
  type      = string
  sensitive = true
}
