# Agent with MCP servers and all three tools variants

Wires an agent up to the GitHub MCP server and shows every tools variant
on a single resource.

## What you'll learn

- How to declare `mcp_servers` and reference one from a `tools` entry.
- The three `tools` variants discriminated on `type`:
  - `agent_toolset_20260401`: the bundled Anthropic toolset.
  - `mcp_toolset`: exposes an MCP server's tools via `mcp_server_name`.
  - `custom`: user-defined tool with a JSON Schema (`input_schema`).
- The `permission_policy` toggle on `default_config` and `configs[*]`:
  - `always_allow` — run automatically.
  - `always_ask` — wait for explicit approval per call.

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported

## Run

```sh
terraform init
terraform apply
```

## Try it

- Switch the MCP toolset's `permission_policy.type` to `always_allow`,
  re-`apply`, and observe the `version` bump.
- Add a per-tool override under `configs` (e.g. force `bash` to
  `always_ask`).

## Tear down

```sh
terraform destroy
```
