terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 0.4"
    }
  }
}

provider "claude-managed-agents" {}

# List the append-only run records for a deployment. Each scheduled or manual
# fire produces one run; exactly one of `session_id` (success) or `error_type`
# (failure) is set on each.
data "claude-managed-agents_deployment_runs" "failures" {
  deployment_id = "deployment_01HqR2k7vXbZ9mNpL3wYcT8f"

  # Optional filters (all may be combined):
  trigger_type = "schedule" # "schedule" or "manual"
  has_error    = true       # true = failed runs only, false = successful only
  limit        = 50         # default 20, max 1000 (first page only)
}

# Surface the typed failure reasons so you can alert on them.
output "recent_failures" {
  value = [
    for run in data.claude-managed-agents_deployment_runs.failures.runs : {
      id         = run.id
      error_type = run.error_type
      at         = run.created_at
    }
  ]
}
