terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Shared memory store: every worker plus the coordinator can attach this
# store at session time to read project context.
resource "claude-managed-agents_memory_store" "shared_context" {
  name        = "Project Context"
  description = "Cross-team decisions, ADRs, and ongoing project notes."
}

# Worker 1: writes code.
resource "claude-managed-agents_agent" "implementer" {
  name   = "Implementer"
  model  = "claude-opus-4-7"
  system = "Implement features end-to-end. Write idiomatic Go and Terraform."

  metadata = {
    role = "implementer"
  }
}

# Worker 2: reviews code written by the implementer.
resource "claude-managed-agents_agent" "reviewer" {
  name   = "Reviewer"
  model  = "claude-opus-4-7"
  system = "Critique diffs for correctness, security, and style. Cite filenames."

  metadata = {
    role = "reviewer"
  }
}

# Worker 3: writes documentation from the diff and reviewer notes.
resource "claude-managed-agents_agent" "doc_writer" {
  name   = "Documentation Writer"
  model  = "claude-sonnet-4-6"
  system = "Translate code changes and review comments into user-facing changelog entries."

  metadata = {
    role = "doc-writer"
  }
}

# Coordinator: routes tasks across the three workers.
resource "claude-managed-agents_agent" "tech_lead" {
  name   = "Tech Lead Coordinator"
  model  = "claude-opus-4-7"
  system = "Decompose tasks across Implementer, Reviewer, and Doc Writer. Synthesize their outputs."

  multiagent = {
    type = "coordinator"
    agents = [
      { type = "agent", id = claude-managed-agents_agent.implementer.id },
      { type = "agent", id = claude-managed-agents_agent.reviewer.id },
      { type = "agent", id = claude-managed-agents_agent.doc_writer.id },
    ]
  }
}

output "tech_lead_id" {
  value = claude-managed-agents_agent.tech_lead.id
}

output "shared_context_id" {
  value = claude-managed-agents_memory_store.shared_context.id
}
