terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 0.4"
    }
  }
}

provider "claude-managed-agents" {}

# A deployment binds an agent to an environment, vaults, mounted resources,
# initial events, and an optional cron schedule. Each scheduled (or manual)
# fire produces a deployment_run audit record.
resource "claude-managed-agents_agent" "digest" {
  name   = "Nightly Digest"
  model  = "claude-opus-4-7"
  system = "Summarize the day's activity into a concise digest."
}

resource "claude-managed-agents_environment" "default" {
  name = "digest-env"
  config = {
    type       = "cloud"
    networking = { type = "unrestricted" }
  }
}

resource "claude-managed-agents_deployment" "nightly_digest" {
  name           = "nightly-digest"
  description    = "Runs every night at 03:00 UTC and emails a summary."
  agent          = claude-managed-agents_agent.digest.id
  environment_id = claude-managed-agents_environment.default.id

  # Intent: "active" (default) or "paused". Changing this pauses/resumes the
  # deployment. It is decoupled from the observed `status`, so an automatic
  # error-pause does NOT make Terraform fight the API — inspect
  # `paused_reason`, fix the cause, then re-apply to resume.
  desired_status = "active"

  metadata = {
    team = "platform"
  }

  # 1-50 events sent to each session on start. Changing this list forces a
  # replacement (the API does not patch events in place). `content` is a
  # JSON-encoded array of content blocks.
  initial_events = [
    {
      type    = "user.message"
      content = jsonencode([{ type = "text", text = "Run the nightly digest." }])
    },
    {
      type           = "user.define_outcome"
      description    = "Produce a one-page digest of today's activity."
      max_iterations = 3
      rubric         = { type = "text", content = "Cite every source. No speculation." }
    },
  ]

  # Optional resources mounted into each session. The github_repository
  # token is write-only: it is sent to the API but never stored in state.
  resources = [
    {
      type                           = "github_repository"
      url                            = "https://github.com/acme/reports"
      authorization_token            = var.github_token
      authorization_token_wo_version = 1
      checkout                       = { type = "branch", name = "main" }
    },
  ]

  # Optional 5-field POSIX cron schedule (no "@daily"-style shortcuts).
  # Omit for a manually-triggered deployment.
  schedule = {
    type       = "cron"
    expression = "0 3 * * *"
    timezone   = "UTC"
  }
}

variable "github_token" {
  type      = string
  sensitive = true
}
