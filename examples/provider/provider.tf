terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

# The provider reads ANTHROPIC_API_KEY from the environment by default.
# Prefer the env var over committing keys to HCL — required.
provider "claude-managed-agents" {
  # api_key     = var.anthropic_api_key
  # base_url    = "https://api.anthropic.com"
  # max_retries = 3
}
