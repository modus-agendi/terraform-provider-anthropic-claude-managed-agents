# Look up an existing sandbox environment by id. Useful when the environment
# was created outside of Terraform (e.g. via the SDK) and you want to read
# its current networking policy or package list.

data "claude-managed-agents_environment" "existing" {
  id = "env_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "existing_env_name" {
  value = data.claude-managed-agents_environment.existing.name
}

output "existing_env_networking_type" {
  value = data.claude-managed-agents_environment.existing.config.networking.type
}
