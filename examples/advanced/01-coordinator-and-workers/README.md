# Coordinator and workers

A four-agent system: three specialized workers plus a coordinator that
routes tasks across them. Includes a shared memory store the agents can
attach at session time.

## What you'll learn

- How to compose multiple agents in one Terraform config.
- How `multiagent.agents` references peer agents by `id` (the `agent_*`
  string from each worker's `id` output).
- That memory stores live outside any single agent — your session-creation
  code attaches them at runtime; they are not part of the agent resource.

## Topology

```
        Tech Lead Coordinator
       /         |         \
Implementer   Reviewer   Doc Writer
       \         |         /
        Shared Memory Store
```

## Prerequisites

- Terraform >= 1.11
- `ANTHROPIC_API_KEY` exported

## Run

```sh
terraform init
terraform apply
```

## Verify

`apply` should print the `tech_lead_id` and `shared_context_id`. The four
agents and the memory store are all created in one run.

## Tear down

```sh
terraform destroy
```

Every agent is archived (one-way). The memory store is archived (the
default — preserves the audit trail).
