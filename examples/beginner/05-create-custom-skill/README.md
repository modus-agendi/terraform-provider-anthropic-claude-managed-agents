# Create a custom skill

The smallest end-to-end use of `claude-managed-agents_skill`: a single
resource that uploads a `skill-content/` directory to the Skills API.

## What you'll learn

- How `claude-managed-agents_skill` packages a local directory and
  uploads it as a versioned bundle.
- What `display_title`, `source_dir`, and the computed `content_hash` /
  `latest_version` attributes do.
- How drift detection works: edit any file under `skill-content/` and
  Terraform will plan a new version on the next `apply`.
- How destroy cascades through every version before deleting the skill.

## Prerequisites

- Terraform >= 1.11 or OpenTofu >= 1.8
- `ANTHROPIC_API_KEY` exported in your shell

## Files

- `main.tf` — provider block + the skill resource
- `skill-content/SKILL.md` — required top-level metadata file
- `skill-content/notes.md` — sibling file Claude can read on demand

## Run

```sh
export ANTHROPIC_API_KEY=sk-ant-...
terraform init
terraform plan
terraform apply
```

The apply output prints `skill_id`, `latest_version`, and
`content_hash`. Take note of `latest_version` — it's the value you'd
plug into an agent's `skills` block (see
`../../advanced/04-end-to-end-skill-and-agent/`).

## Trigger a new version

Edit `skill-content/notes.md` (or any file under the directory) and
re-apply:

```sh
echo "Updated $(date)" >> skill-content/notes.md
terraform plan
```

The plan should show `content_hash` changing and `latest_version`
becoming known-after-apply. `terraform apply` pushes a new immutable
version to the API.

## Tear down

```sh
terraform destroy
```

`destroy` lists every version of the skill, deletes each one (the
Skills API requires versions be removed before the skill itself), then
deletes the parent skill record.
