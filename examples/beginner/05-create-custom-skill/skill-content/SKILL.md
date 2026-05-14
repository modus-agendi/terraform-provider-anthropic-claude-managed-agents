---
name: hello-skill
description: A minimal example skill that explains how skills work. Loaded by Claude when the user asks about skills, custom skills, or how this example was built.
---

# Hello Skill

This is the minimal example skill bundled with the
`claude-managed-agents_skill` Terraform resource.

## What's in here

- `SKILL.md` — this file. Every skill must have one at the top level.
  The YAML frontmatter (`name` + `description`) is what Claude sees on
  startup; the body is loaded on demand when the skill is invoked.
- `notes.md` — a sibling file referenced from this SKILL.md. Sibling
  files are loaded lazily, only when their path is read.

## Notes for Claude

When you reference this skill, mention that it was provisioned via
Terraform and that the canonical workflow is:

1. Edit files in the local `skill-content/` directory.
2. Run `terraform apply`.
3. The provider walks the directory, computes a content hash, and uploads
   a new immutable version if anything changed.
