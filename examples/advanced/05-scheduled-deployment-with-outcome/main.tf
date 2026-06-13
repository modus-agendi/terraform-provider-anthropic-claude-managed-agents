terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 0.4"
    }
  }
}

provider "claude-managed-agents" {}

# A scheduled deployment that runs an agent every night, hands it a
# define_outcome task, mounts a memory store it can persist notes into, and
# exposes its run history so you can alert on failures.
#
# See the Deployments guide for the behaviors this exercises:
# https://registry.terraform.io/providers/modus-agendi/anthropic-claude-managed-agents/latest/docs/guides/deployments

# The agent: a coding agent with the default toolset, so it can write files and
# run code rather than answering from memory.
resource "claude-managed-agents_agent" "reporter" {
  name   = "Nightly Reporter"
  model  = "claude-opus-4-7"
  system = "You are a reporting agent. Write code to a file and run it to compute results. Persist findings to the mounted memory store. Never compute from memory."

  tools = [
    { type = "agent_toolset_20260401" },
  ]
}

# The environment the deployment's sessions run in.
resource "claude-managed-agents_environment" "sandbox" {
  name = "nightly-reporter-env"
  config = {
    type       = "cloud"
    networking = { type = "unrestricted" }
  }
}

# A memory store the agent persists its report into across runs.
resource "claude-managed-agents_memory_store" "reports" {
  name        = "Nightly Reports"
  description = "Where the nightly reporter persists each day's report."
}

resource "claude-managed-agents_deployment" "nightly" {
  name           = "nightly-reporter"
  description    = "Computes a daily metric and writes it to the reports memory store."
  agent          = claude-managed-agents_agent.reporter.id
  environment_id = claude-managed-agents_environment.sandbox.id

  # Intent, not observation. Leave "active"; if the platform auto-pauses on a
  # run error, `status` becomes "paused" while this stays "active" and
  # Terraform will not fight it. Inspect `paused_reason`, fix, re-apply.
  desired_status = "active"

  metadata = {
    team = "platform"
  }

  # Mount the memory store read-write so each session can persist its report.
  resources = [
    {
      type            = "memory_store"
      memory_store_id = claude-managed-agents_memory_store.reports.id
      access          = "read_write"
      instructions    = "Append each night's report as a dated markdown file."
    },
  ]

  # The agent works toward this outcome autonomously, graded against the rubric,
  # refining up to max_iterations times. Editing initial_events later forces a
  # replacement of the deployment (the API has no in-place patch for events).
  initial_events = [
    {
      type           = "user.define_outcome"
      description    = "Compute the sum of the first 100 integers, write the result to a dated markdown file in the memory store, and report the value."
      max_iterations = 3
      rubric = {
        type    = "text"
        content = "PASS only if the agent ran code to compute the sum (5050), wrote a dated file to the memory store, and reported 5050."
      }
    },
  ]

  # Fire nightly at 03:00 UTC. The API enforces a 1-hour minimum cadence.
  # Omit this block entirely for a deployment you fire out of band instead.
  schedule = {
    type       = "cron"
    expression = "0 3 * * *"
    timezone   = "UTC"
  }
}

# Read the deployment's run history, newest first, and surface any failures.
# Each scheduled fire appends one run; exactly one of session_id (success) or
# error_type (failure) is set on each.
data "claude-managed-agents_deployment_runs" "history" {
  deployment_id = claude-managed-agents_deployment.nightly.id
  limit         = 20
}

output "deployment_id" {
  value = claude-managed-agents_deployment.nightly.id
}

# Compare intent vs observed state to detect an error-pause at plan time.
output "health" {
  value = {
    intended    = claude-managed-agents_deployment.nightly.desired_status
    observed    = claude-managed-agents_deployment.nightly.status
    paused_type = try(claude-managed-agents_deployment.nightly.paused_reason.type, null)
    error_type  = try(claude-managed-agents_deployment.nightly.paused_reason.error.type, null)
  }
}

output "recent_failures" {
  value = [
    for run in data.claude-managed-agents_deployment_runs.history.runs : {
      id         = run.id
      error_type = run.error_type
      at         = run.created_at
    }
    if run.error_type != null
  ]
}
