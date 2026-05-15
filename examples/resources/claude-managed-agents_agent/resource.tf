terraform {
  required_providers {
    claude-managed-agents = {
      source  = "andasv/anthropic-claude-managed-agents"
      version = "~> 0.2"
    }
  }
}

provider "claude-managed-agents" {}

# A coding-focused agent showing every nested-config surface area:
# mcp_servers, the three tools variants, skills, and a coordinator.
resource "claude-managed-agents_agent" "code_review" {
  name        = "Code Review Assistant"
  model       = "claude-opus-4-7"
  system      = "Review diffs for correctness, style, and security. Cite filenames and line numbers."
  description = "Pairs on Go and Terraform code reviews."

  # Full-replace on update: removing a key from HCL deletes it server-side.
  metadata = {
    team        = "platform"
    environment = "prod"
  }

  # MCP servers the agent may reach at session runtime. Every server here
  # must be referenced by a matching tools[mcp_toolset].mcp_server_name.
  mcp_servers = [
    {
      type = "url"
      name = "github"
      url  = "https://api.githubcopilot.com/mcp/"
    },
  ]

  # tools is a discriminated union on `type`. The three variants below
  # cover every case the upstream API supports.
  tools = [
    # Bundled Anthropic toolset (bash, edit, web_fetch, ...).
    {
      type = "agent_toolset_20260401"
      default_config = {
        permission_policy = { type = "always_allow" }
      }
      # Per-tool override: disable web_fetch even though the toolset
      # default allows it.
      configs = [
        { name = "web_fetch", enabled = false },
      ]
    },
    # Expose the MCP server's tools to the agent. Note mcp_server_name
    # matches the github entry in mcp_servers above.
    {
      type            = "mcp_toolset"
      mcp_server_name = "github"
      default_config = {
        permission_policy = { type = "always_ask" }
      }
    },
    # User-defined tool. input_schema is a JSON Schema encoded as a JSON
    # string (the schema is recursive, so the provider keeps it flat).
    {
      type        = "custom"
      name        = "lookup_pr"
      description = "Look up a pull request by number."
      input_schema = jsonencode({
        type = "object"
        properties = {
          repo   = { type = "string" }
          number = { type = "integer" }
        }
        required = ["repo", "number"]
      })
    },
  ]

  skills = [
    # Pre-built Anthropic skill — skill_id is a short name.
    { type = "anthropic", skill_id = "xlsx" },
    # User-uploaded skill — skill_id is `skill_*`; version defaults server-side to "latest".
    { type = "custom", skill_id = "skill_01HABCDEF1234567890ABCD", version = "latest" },
  ]
}

# A coordinator agent that delegates to the reviewer above.
resource "claude-managed-agents_agent" "tech_lead" {
  name  = "Tech Lead Coordinator"
  model = "claude-sonnet-4-6"

  multiagent = {
    type = "coordinator"
    agents = [
      { type = "agent", id = claude-managed-agents_agent.code_review.id },
    ]
  }
}

output "code_review_id" {
  value = claude-managed-agents_agent.code_review.id
}

output "code_review_version" {
  value = claude-managed-agents_agent.code_review.version
}
