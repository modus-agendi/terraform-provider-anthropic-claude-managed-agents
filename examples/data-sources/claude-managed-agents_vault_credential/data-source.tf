terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Look up an existing vault credential. The API never returns secret
# values; this data source only exposes metadata and non-secret config
# such as the bound MCP server URL and OAuth refresh shape.
data "claude-managed-agents_vault_credential" "linear" {
  vault_id = "vlt_01HqR2k7vXbZ9mNpL3wYcT8f"
  id       = "cred_01HFEDCBA9876543210ABCD"
}

output "credential_display_name" {
  value = data.claude-managed-agents_vault_credential.linear.display_name
}

output "credential_mcp_server_url" {
  value = data.claude-managed-agents_vault_credential.linear.auth.mcp_server_url
}

output "credential_auth_type" {
  value = data.claude-managed-agents_vault_credential.linear.auth.type
}
