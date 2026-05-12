terraform {
  required_version = ">= 1.8"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/claude-managed-agents"
      version = "~> 0.1"
    }
  }
}

# The provider reads ANTHROPIC_API_KEY from the environment by default.
# Override via the api_key argument if you must, but prefer the env var.
provider "claude-managed-agents" {
  # api_key     = var.anthropic_api_key
  # base_url    = "https://api.anthropic.com"
  # max_retries = 3
}
