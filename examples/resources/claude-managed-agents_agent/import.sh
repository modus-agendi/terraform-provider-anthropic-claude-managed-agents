#!/usr/bin/env bash
# Import an existing Claude Managed Agents agent by its server-assigned id.
# The id is the `agent_*` string returned by the API on create.

terraform import claude-managed-agents_agent.coding_assistant agent_01HqR2k7vXbZ9mNpL3wYcT8f
