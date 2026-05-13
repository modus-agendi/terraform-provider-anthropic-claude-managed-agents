# Look up an existing vault credential. The API never returns secret values,
# so the data source only exposes metadata + non-secret config fields.
data "claude-managed-agents_vault_credential" "linear" {
  vault_id = "vlt_01HqR2k7vXbZ9mNpL3wYcT8f"
  id       = "cred_01HABCDEF..."
}

output "linear_mcp_server_url" {
  value = data.claude-managed-agents_vault_credential.linear.auth.mcp_server_url
}
