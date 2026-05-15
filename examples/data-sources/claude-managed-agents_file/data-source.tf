terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# Look up metadata for a file uploaded via the Managed Agents Files API.
# Only metadata is exposed; the binary content endpoint is not modeled by
# this provider. Useful when a session-scoped artifact needs to be
# referenced by id from elsewhere in your Terraform config.
data "claude-managed-agents_file" "report" {
  id = "file_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "report_filename" {
  value = data.claude-managed-agents_file.report.filename
}

output "report_size_bytes" {
  value = data.claude-managed-agents_file.report.size_bytes
}

output "report_mime_type" {
  value = data.claude-managed-agents_file.report.mime_type
}
