---
page_title: "Upgrading and migration"
subcategory: "Guides"
description: |-
  Version constraints, the consolidated breaking-change history (registry slug
  and namespace moves), and a safe upgrade procedure.
---

# Upgrading and migration

~> **Reminder:** experimental community provider — not for production, no
warranty, use at your own risk.

## Version constraints

Pin the major line so you receive non-breaking updates automatically:

```hcl
terraform {
  required_providers {
    claude-managed-agents = {
      source  = "modus-agendi/anthropic-claude-managed-agents"
      version = "~> 1.0"   # >= 1.0.0, < 2.0.0
    }
  }
}
```

As of **v1.0.0** the resource and data-source schema is stable under SemVer:
breaking schema changes happen only in a future major (2.0.0). Use `~> 1.0` for
worry-free minor/patch upgrades; pin exactly (`= 1.x.y`) if you require
bit-for-bit reproducibility.

## Breaking-change history (pre-1.0)

Two pre-1.0 releases changed the **registry source string** (only the `source`
in `required_providers` — local provider name, resource type names, and your
HCL resource blocks were unchanged):

| Version | Change | Migration |
|---|---|---|
| **v0.4.0** | Registry slug renamed `andasv/claude-managed-agents` → `andasv/anthropic-claude-managed-agents` (the old slug was orphaned when the GitHub repo was renamed). | Update `source`, `terraform init -upgrade`. |
| **v0.4.1** | Namespace moved `andasv` → `modus-agendi`. | Update `source` to `modus-agendi/anthropic-claude-managed-agents`, `terraform init -upgrade`. |
| **v1.0.0** | First stable release; SemVer stability guarantee. No schema changes from v0.5.0. | None — additive. |

If you are coming from any `0.x` published under the old namespaces, the only
change you need is the `source`:

```hcl
# before (any old form)
source = "andasv/claude-managed-agents"            # pre-0.4.0
source = "andasv/anthropic-claude-managed-agents"  # 0.4.0
# after
source = "modus-agendi/anthropic-claude-managed-agents"  # 0.4.1+
```

Then:

```sh
terraform init -upgrade
terraform plan   # expect an empty plan — the move is source-only
```

The Go module path is `github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents`
(affects vendored consumers only).

## Safe upgrade procedure

1. **Read the [CHANGELOG](https://github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/blob/main/CHANGELOG.md)**
   for the target version.
2. **Back up state** before a major bump:
   `terraform state pull > backup-$(date +%s).tfstate`.
3. **Bump the constraint** and `terraform init -upgrade`.
4. **Plan first**: `terraform plan` and review. A minor/patch upgrade on `~> 1.0`
   should produce an empty plan.
5. **Roll out gradually** across workspaces — dev → staging → prod — rather than
   upgrading everything at once.
6. **Rollback**: if a plan looks wrong, re-pin the previous version and
   `terraform init -upgrade` again; restore the state backup only if an apply
   already ran and changed state.

## OpenTofu compatibility

The provider requires **Terraform 1.11+** or **OpenTofu 1.8+** — both support
the TF 1.11 write-only attributes used for `claude-managed-agents_vault_credential`
secrets and the deployment github token. Older engines should stay on a pre-1.11
provider line if one applies to your setup.

## See also

- [State security and remote backends](state-security-and-backends.md)
- [Drift detection and remediation](drift-detection-and-remediation.md)
