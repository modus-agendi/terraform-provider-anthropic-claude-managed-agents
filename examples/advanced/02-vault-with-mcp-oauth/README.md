# Vault with static bearer and OAuth credentials

One vault per end-user, two credentials: a static bearer token (Linear)
and a full OAuth credential with refresh (Slack).

## What you'll learn

- The `vault` → `vault_credential` parent/child structure.
- Both `auth.type` variants:
  - `static_bearer`: single bearer token, suitable for personal access tokens.
  - `mcp_oauth`: access token + optional refresh block. Anthropic refreshes
    the access token on your behalf using `refresh.token_endpoint`,
    `refresh.client_id`, and `refresh.refresh_token`.
- Write-only secrets and rotation:
  - `token`, `access_token`, `refresh_token`, and `client_secret` are
    TF 1.11 WriteOnly attributes — sent to the API but never stored in
    state.
  - Each is paired with a `*_wo_version` integer counter; increment it
    when you change the secret in your variable and the provider re-sends
    the value on the next apply.

## Prerequisites

- Terraform >= 1.11 (write-only attributes require TF 1.11)
- `ANTHROPIC_API_KEY` exported
- Real values for `linear_token`, `slack_access_token`,
  `slack_refresh_token`, `slack_client_secret`

## Run

```sh
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars and fill in the secrets.
terraform init
terraform apply
```

## Rotate a secret

1. Update the value in `terraform.tfvars`.
2. Bump the matching `*_wo_version` counter in `main.tf`.
3. `terraform apply` — the provider sends the new value to the API.

## Tear down

```sh
terraform destroy
```

By default the vault is archived (cascading to its credentials, secrets
purged, audit trail preserved). Set `delete_on_destroy = true` on the
vault to hard-delete.
