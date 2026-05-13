# Environment with unrestricted networking

A simple cloud sandbox with a pip preinstall list and outbound networking
wide open.

## What you'll learn

- How to declare a `claude-managed-agents_environment` with `type = "cloud"`.
- How to preinstall packages via `config.packages.pip` (also supports
  `apt`, `cargo`, `gem`, `go`, `npm`).
- That `networking.type = "unrestricted"` allows any outbound host —
  appropriate for prototypes, not for production.
- That every config attribute is `RequiresReplace`: editing the pip list
  destroys and re-creates the environment.

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported

## Run

```sh
terraform init
terraform apply
```

## Try it

Edit the pip list and re-`apply`. Terraform plans a destroy + create
because environments are immutable.

## Tear down

```sh
terraform destroy
```

`destroy` issues `DELETE /v1/environments/{id}`. If active sessions still
reference the environment, the provider falls back to archive.
