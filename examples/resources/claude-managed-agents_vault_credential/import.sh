#!/usr/bin/env bash
# Import a vault credential using a composite "vault_id:credential_id" id —
# both ids are `vlt_*` and `cred_*` strings returned by the API.
#
# Secrets are never returned by the API, so the WriteOnly attributes
# (token, access_token, refresh_token, client_secret) start as null after
# import. Bump the matching *_wo_version after import to re-send the secret
# value from your variable on the next apply.

terraform import claude-managed-agents_vault_credential.linear vlt_01HqR2k7vXbZ9mNpL3wYcT8f:cred_01HFEDCBA9876543210ABCD
