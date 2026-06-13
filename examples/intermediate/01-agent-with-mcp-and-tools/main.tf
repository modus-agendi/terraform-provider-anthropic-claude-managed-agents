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

# This example wires up all three tools variants on a single agent:
#   - agent_toolset_20260401 (the bundled Anthropic toolset)
#   - mcp_toolset             (exposes an MCP server's tools)
#   - custom                  (user-defined tool with a JSON Schema)
resource "claude-managed-agents_agent" "github_assistant" {
  name        = "GitHub Pull Request Assistant"
  model       = "claude-opus-4-7"
  system      = "Triage and label pull requests. Summarize changes for reviewers."
  description = "Routes PRs, applies labels, and drafts review comments."

  # Every MCP server here must be referenced by a tools[mcp_toolset] entry.
  mcp_servers = [
    {
      type = "url"
      name = "github"
      url  = "https://api.githubcopilot.com/mcp/"
    },
  ]

  tools = [
    # Variant 1: bundled Anthropic toolset. Allow everything except
    # web_fetch (we'd rather force MCP for outbound calls).
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_allow" }
      }
      configs = [
        { name = "web_fetch", enabled = false },
      ]
    },
    # Variant 2: expose the GitHub MCP server's tools. Default to
    # always_ask so a human approves each mutation.
    {
      type            = "mcp_toolset"
      mcp_server_name = "github"
      default_config = {
        permission_policy = { type = "always_ask" }
      }
    },
    # Variant 3: custom tool. The model calls this by name and the
    # caller (your application) implements the handler.
    {
      type        = "custom"
      name        = "summarize_diff"
      description = "Summarize a unified diff blob for review."
      input_schema = jsonencode({
        type = "object"
        properties = {
          diff      = { type = "string" }
          max_words = { type = "integer", minimum = 50, maximum = 500 }
        }
        required = ["diff"]
      })
    },
  ]
}

output "agent_id" {
  value = claude-managed-agents_agent.github_assistant.id
}
