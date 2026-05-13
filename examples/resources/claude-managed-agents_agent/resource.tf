resource "claude-managed-agents_agent" "coding_assistant" {
  name        = "Coding Assistant"
  model       = "claude-opus-4-7"
  system      = "You are a helpful coding agent. Be concise. Cite filenames."
  description = "Pairs on Go and Terraform tasks."

  metadata = {
    team        = "platform"
    environment = "prod"
  }

  mcp_servers = [
    { type = "url", name = "github", url = "https://mcp.example.com/github" },
  ]

  # Tools: the bundled Anthropic toolset, an MCP toolset bound to the
  # github MCP server above, and a user-defined custom tool with a JSON
  # Schema for its arguments.
  tools = [
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_allow" }
      }
      configs = [
        { name = "web_fetch", enabled = false },
      ]
    },
    {
      type            = "mcp_toolset"
      mcp_server_name = "github"
      default_config = {
        permission_policy = { type = "always_ask" }
      }
    },
    {
      type        = "custom"
      name        = "lookup_user"
      description = "Look up a user by id"
      input_schema = jsonencode({
        type = "object"
        properties = {
          user_id = { type = "string" }
        }
        required = ["user_id"]
      })
    },
  ]

  skills = [
    { type = "anthropic", skill_id = "xlsx" },
    { type = "custom", skill_id = "skill_abc123", version = "latest" },
  ]
}

# Coordinator that delegates to the assistant above.
resource "claude-managed-agents_agent" "lead" {
  name  = "Engineering Lead"
  model = "claude-opus-4-7"

  multiagent = {
    type = "coordinator"
    agents = [
      { type = "agent", id = claude-managed-agents_agent.coding_assistant.id },
      { type = "self" },
    ]
  }
}

output "agent_id" {
  value = claude-managed-agents_agent.coding_assistant.id
}

output "agent_version" {
  value = claude-managed-agents_agent.coding_assistant.version
}
