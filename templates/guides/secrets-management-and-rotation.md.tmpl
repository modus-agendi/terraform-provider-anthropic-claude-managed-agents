---
page_title: "Secrets management and rotation"
subcategory: "Guides"
description: |-
  How write-only secret attributes work in this provider, and the rotation
  ceremony for vault credentials and deployment github tokens.
---

# Secrets management and rotation

~> **Reminder:** experimental community provider — not for production, no
warranty, use at your own risk.

## Write-only attributes (TF 1.11+)

Secret-bearing fields in this provider are **write-only**: Terraform sends them
to the API on create/update but never stores them in state and never reads them
back. This keeps secrets out of the state file entirely (see
[State security](state-security-and-backends.md)).

| Resource | Write-only field(s) | Rotation counter |
|---|---|---|
| `claude-managed-agents_vault_credential` | `auth.token`, `auth.access_token`, `auth.refresh.refresh_token`, `auth.client_secret` | `auth.*_wo_version` |
| `claude-managed-agents_deployment` | `resources[*].authorization_token` (github) | `resources[*].authorization_token_wo_version` |

Because Terraform can't see the current secret value, it cannot detect that a
secret changed on its own. You signal a change by incrementing the paired
**`*_wo_version`** integer: the version is stored in state, so bumping it
produces a diff that re-sends the secret.

## Rotation ceremony — vault credential

1. Rotate the secret in the upstream system (e.g. issue a new Linear token,
   revoke the old one).
2. Update the new value in your secret source (TF variable, `TF_VAR_...` env
   var, or a `data` source from Vault/AWS Secrets Manager).
3. Increment the matching `*_wo_version`:

   ```hcl
   resource "claude-managed-agents_vault_credential" "linear" {
     vault_id     = claude-managed-agents_vault.team.id
     display_name = "Linear"
     auth = {
       type             = "static_bearer"
       mcp_server_url   = "https://mcp.linear.app/mcp"
       token            = var.linear_token   # new value
       token_wo_version = 2                   # was 1
     }
   }
   ```

4. `terraform apply`. The provider re-sends the token; the bump is the only
   thing that changes in state.

`auth.type`, `auth.mcp_server_url`, and the OAuth `refresh.token_endpoint` /
`refresh.client_id` are immutable — changing them forces replacement, not an
in-place rotation.

## Rotation ceremony — deployment github token

Same pattern for a `github_repository` mount:

```hcl
resources = [
  {
    type                           = "github_repository"
    url                            = "https://github.com/acme/reports"
    authorization_token            = var.github_token   # new value
    authorization_token_wo_version = 2                  # was 1
  },
]
```

## Sourcing secrets without hardcoding

Prefer pulling secrets from a managed store at apply time rather than committing
them:

```hcl
# Example: AWS Secrets Manager
data "aws_secretsmanager_secret_version" "linear" {
  secret_id = "claude-agents/linear-token"
}

resource "claude-managed-agents_vault_credential" "linear" {
  # ...
  auth = {
    type             = "static_bearer"
    mcp_server_url   = "https://mcp.linear.app/mcp"
    token            = data.aws_secretsmanager_secret_version.linear.secret_string
    token_wo_version = 1
  }
}
```

Note: a value read through a `data` source flows through the plan and **can land
in state for the resource that consumes it** — but here the consuming attribute
is write-only, so the token is not persisted. Still, keep `TF_LOG` off in shared
CI to avoid debug-log exposure.

## Leak response

If a secret is exposed (committed, logged, shared):

1. **Revoke** it upstream immediately — rotation alone is not enough, the old
   value must be invalidated.
2. Issue a replacement and apply it via the rotation ceremony above.
3. If the leak was in a committed file, rewriting git history does not undo
   exposure — treat the secret as burned and revoke.

## Audit

The Anthropic API has its own audit trail for credential use; Terraform's view
is limited to "a `*_wo_version` bump re-sent the secret." For who-changed-what
on the secret value itself, rely on your upstream secret store's audit log.

## See also

- [State security and remote backends](state-security-and-backends.md)
- [Import and state recovery](import-and-state-recovery.md)
