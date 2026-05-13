# Skills and multi-agent coordinator

Two agents wired together: an analyst worker and a coordinator that
delegates to it.

## What you'll learn

- Both `skills` variants discriminated on `type`:
  - `anthropic`: pre-built skills (`xlsx`, `pdf`, etc.). `skill_id` is a
    short name.
  - `custom`: user-uploaded skills. `skill_id` is a `skill_*` id; `version`
    defaults server-side to `latest`.
- How to build a coordinator via `multiagent`:
  - `type = "coordinator"` is currently the only supported coordinator type.
  - Members are either `{ type = "agent", id = "agent_*" }` (delegate) or
    `{ type = "self" }` (self-invocation).

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported
- Replace `skill_01HABCDEF1234567890ABCD` in `main.tf` with a real custom
  skill id from your workspace, or drop the line if you only need
  Anthropic skills.

## Run

```sh
terraform init
terraform apply
```

## Tear down

```sh
terraform destroy
```
