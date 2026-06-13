terraform {
  required_version = ">= 1.11"

  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 1.0"
    }
  }
}

provider "claude-managed-agents" {}

# A persistent memory store the analyst attaches at session time. Lets the
# agent retain context about past quarters across separate sessions.
resource "claude-managed-agents_memory_store" "finance" {
  name        = "tf-acc-test-finance-history"
  description = "Multi-quarter financial context for the report builder."
}

# Custom skill: a directory of files (SKILL.md + a markdown template).
# Editing anything under skill-content/ on disk causes the next apply
# to upload a new immutable version. The skill resource exposes
# `latest_version` as Computed, so downstream consumers can pin to it.
resource "claude-managed-agents_skill" "report_builder" {
  display_title = "tf-acc-test-financial-report-builder"
  source_dir    = "${path.module}/skill-content"
}

# Anthropic prebuilt skill for working with .xlsx files. Looked up via
# the data source so we depend on the API's notion of "latest" rather
# than hardcoding a version string.
data "claude-managed-agents_skill" "xlsx" {
  skill_id = "xlsx"
}

# The analyst agent: opus-class model, both skills pinned to the
# data-source / resource attributes. When the custom skill bumps a
# version on the next apply, the agent's pin updates automatically and
# Terraform plans an in-place agent update.
resource "claude-managed-agents_agent" "analyst" {
  name        = "Financial Analyst"
  model       = "claude-opus-4-7"
  system      = "You produce quarterly financial summaries from raw transaction data. Use the report-builder skill for output structure and the xlsx skill when input is a spreadsheet."
  description = "Generates quarterly P&L reports from transaction CSVs."

  metadata = {
    team        = "finance"
    environment = "prod"
  }

  skills = [
    {
      type     = "custom"
      skill_id = claude-managed-agents_skill.report_builder.id
      version  = claude-managed-agents_skill.report_builder.latest_version
    },
    {
      type     = "anthropic"
      skill_id = data.claude-managed-agents_skill.xlsx.skill_id
      version  = data.claude-managed-agents_skill.xlsx.latest_version
    },
  ]
}

output "skill_id" {
  description = "The custom skill's `skill_*` identifier."
  value       = claude-managed-agents_skill.report_builder.id
}

output "skill_latest_version" {
  description = "The version the agent is currently pinned to."
  value       = claude-managed-agents_skill.report_builder.latest_version
}

output "agent_id" {
  description = "The analyst agent's id."
  value       = claude-managed-agents_agent.analyst.id
}

output "memory_store_id" {
  value = claude-managed-agents_memory_store.finance.id
}
