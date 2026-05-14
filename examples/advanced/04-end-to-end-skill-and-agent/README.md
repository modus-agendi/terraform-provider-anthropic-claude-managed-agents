# End-to-end: custom skill + agent + memory store

A realistic four-resource composition that ties together everything v0.3
adds. Builds a "financial analyst" agent that uses both a custom skill
you author locally and Anthropic's prebuilt `xlsx` skill, plus a memory
store for multi-session context.

## What you'll learn

- How to wire a `claude-managed-agents_skill` resource to an agent's
  `skills` block via `id` and `latest_version`.
- How to mix custom skills (managed by you, via the resource) with
  Anthropic prebuilt skills (read via the data source).
- How a skill version bump flows through to an agent update: edit the
  local `skill-content/` directory → `terraform apply` → new skill
  version uploaded → agent's `skills[*].version` value changes →
  Terraform updates the agent in place.
- How a memory store fits alongside the agent for cross-session state.

## Architecture

```
+------------------------+      +---------------------------+
| skill-content/         |      | data source:              |
|   SKILL.md             |----->|   xlsx (prebuilt)         |
|   template.md          |      +---------------------------+
+------------------------+                 |
            |                              |
            v                              |
+------------------------+                 |
| skill resource         |                 |
|   tf-acc-test-...      |                 |
|   latest_version: ...  |                 |
+------------------------+                 |
            |                              |
            +----- agent.skills -----------+
                       |
                       v
            +---------------------------+
            | agent                     |
            |   Financial Analyst       |
            +---------------------------+
                       |
                       | (attached at session time, not here)
                       v
            +---------------------------+
            | memory_store              |
            |   finance-history         |
            +---------------------------+
```

## Prerequisites

- Terraform >= 1.11 or OpenTofu >= 1.8
- `ANTHROPIC_API_KEY` exported

## Run

```sh
export ANTHROPIC_API_KEY=sk-ant-...
terraform init
terraform plan
terraform apply
```

The outputs print the skill id, current skill version, agent id, and
memory store id.

## Demonstrate a skill version bump

Edit the skill content:

```sh
echo "Updated $(date)" >> skill-content/template.md
terraform plan
```

The plan shows:

1. `claude-managed-agents_skill.report_builder.content_hash` changes.
2. `claude-managed-agents_skill.report_builder.latest_version` becomes
   known-after-apply.
3. `claude-managed-agents_agent.analyst.skills[0].version` becomes
   known-after-apply (because it interpolates from the skill's
   `latest_version` attribute).

`terraform apply` uploads the new skill version, then updates the
agent's pin to match. One coordinated change across two resources.

## Pin to a frozen skill version

If you'd rather not auto-roll the agent every time the skill changes,
replace the agent's `version` interpolation with a string literal:

```hcl
skills = [
  {
    type     = "custom"
    skill_id = claude-managed-agents_skill.report_builder.id
    version  = "1759178010641129"  # frozen
  },
  # ...
]
```

Then bumping the skill creates a new version but leaves the agent
pinned to the old one until you explicitly update the literal.

## Tear down

```sh
terraform destroy
```

Order matters internally: the agent is updated to drop its skill
references first, then the skill's versions are deleted, then the skill
itself, then the memory store. The provider handles the ordering.
