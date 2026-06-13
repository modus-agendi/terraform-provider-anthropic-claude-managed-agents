---
page_title: "State security and remote backends"
subcategory: "Guides"
description: |-
  Where this provider's secrets do and do not live in Terraform state, and how
  to configure an encrypted, locked remote backend (S3+KMS, GCS, Terraform
  Cloud) for shared use.
---

# State security and remote backends

~> **Reminder:** this is an experimental, community provider — not for
production use, no warranty, use at your own risk. The guidance below is
general Terraform good practice applied to this provider.

## What is and isn't in state

Terraform state records the attributes of every managed resource. For this
provider:

- **True secrets are NOT in state.** The `claude-managed-agents_vault_credential`
  secret fields (`token`, `access_token`, `refresh_token`, `client_secret`) and
  the `claude-managed-agents_deployment` `resources[*].authorization_token` are
  TF 1.11 **write-only** attributes — sent to the API but never written to state
  or returned on read.
- **The provider `api_key` is NOT in state.** Provider configuration is not
  persisted to the state file. (Avoid hardcoding it in HCL anyway — prefer the
  `ANTHROPIC_API_KEY` environment variable — because HCL lands in version
  control and can surface in `TF_LOG` debug output.)
- **Everything else IS in state**, in cleartext: resource ids, names,
  `metadata` labels, models, system prompts, agent/skill configuration, and so
  on. None of that is a credential, but it can be sensitive (a system prompt may
  encode business logic; metadata may carry internal identifiers).

The practical conclusion: **treat the state file as sensitive** and store it in
an encrypted, access-controlled, locked remote backend for any shared or
long-lived use. Local `terraform.tfstate` on a laptop is fine only for throwaway
experimentation.

## S3 + KMS (encrypted, locked)

```hcl
terraform {
  backend "s3" {
    bucket       = "my-tf-state"
    key          = "claude-managed-agents/prod.tfstate"
    region       = "us-east-1"
    encrypt      = true                       # server-side encryption
    kms_key_id   = "arn:aws:kms:us-east-1:111122223333:key/abcd-..."
    use_lockfile = true                       # S3-native state locking (TF 1.11+)
  }
}
```

- `encrypt = true` + `kms_key_id` gives you a customer-managed KMS key with its
  own access policy and audit trail (CloudTrail logs every state read/write).
- Turn on **bucket versioning** so a corrupted or accidentally-deleted state can
  be restored to a prior version.
- Scope the bucket policy so only the CI principal and named operators can
  `s3:GetObject` / `s3:PutObject` the state key.

## GCS + Cloud KMS

```hcl
terraform {
  backend "gcs" {
    bucket         = "my-tf-state"
    prefix         = "claude-managed-agents/prod"
    encryption_key = ""  # or use a Cloud KMS CMEK on the bucket
  }
}
```

Enable object versioning on the bucket and a Cloud KMS CMEK for at-rest
encryption with a managed key and audit logging. GCS provides state locking
automatically.

## Terraform Cloud / Enterprise (or OpenTofu state encryption)

```hcl
terraform {
  cloud {
    organization = "my-org"
    workspaces { name = "claude-managed-agents-prod" }
  }
}
```

Terraform Cloud stores state encrypted at rest, locks runs, keeps a state
version history, and centralizes access control and audit. Set
`ANTHROPIC_API_KEY` as a **sensitive** workspace environment variable so it
never appears in run logs. OpenTofu users can alternatively use built-in
[state encryption](https://opentofu.org/docs/language/state/encryption/).

## Backup and recovery

- Rely on backend **versioning** (S3/GCS) or TFC **state versions** as the
  primary backup; both let you roll back a bad apply.
- Before a risky change (a provider major upgrade, a large refactor), snapshot
  with `terraform state pull > backup-$(date +%s).tfstate` and store it
  securely.
- Several resources **archive on destroy and cannot be unarchived** (agents,
  vaults, vault credentials, memory stores, deployments). State backup does not
  recover an archived upstream resource — it only recovers Terraform's record.
  Plan destroys deliberately.

## See also

- [Secrets management and rotation](secrets-management-and-rotation.md)
- [Import and state recovery](import-and-state-recovery.md)
- [Drift detection and remediation](drift-detection-and-remediation.md)
