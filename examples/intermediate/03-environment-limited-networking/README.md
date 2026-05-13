# Environment with limited networking

A cloud environment that restricts egress to an explicit allowlist while
still permitting MCP traffic and package-manager installs.

## What you'll learn

- The `networking.type = "limited"` policy and its three companion fields:
  - `allowed_hosts`: bare hostnames the agent may reach. No URL schemes.
  - `allow_mcp_servers`: gate MCP outbound traffic (default `false`).
  - `allow_package_managers`: gate runtime package installs (default `false`).
- That you typically need `allow_package_managers = true` when you also
  define `config.packages.*` lists; the preinstall step itself counts as
  a package-manager invocation.

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported

## Run

```sh
terraform init
terraform apply
```

## Try it

- Drop `registry.npmjs.org` from `allowed_hosts` and re-`apply`. Terraform
  destroys and re-creates the environment (every config field is immutable).
- Switch `type = "unrestricted"` and observe the plan removing
  `allowed_hosts`, `allow_mcp_servers`, and `allow_package_managers`.

## Tear down

```sh
terraform destroy
```
