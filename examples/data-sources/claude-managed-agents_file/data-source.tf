# Look up metadata for a file uploaded to the Managed Agents Files API.
# Use this when you need to reference a session-scoped artifact by its
# fixed id from another part of your Terraform config.

data "claude-managed-agents_file" "uploaded" {
  id = "file_01HqR2k7vXbZ9mNpL3wYcT8f"
}

output "uploaded_filename" {
  value = data.claude-managed-agents_file.uploaded.filename
}

output "uploaded_size_bytes" {
  value = data.claude-managed-agents_file.uploaded.size_bytes
}
