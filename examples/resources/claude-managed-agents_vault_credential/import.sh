#!/usr/bin/env bash
# Import an existing vault credential. The import id is the composite
# "vault_id:credential_id" — both `vlt_*` and `cred_*` strings from the API.
#
# Secrets are not part of the API response, so the WriteOnly attributes
# (token, access_token, refresh_token, client_secret) start as null. Set the
# matching *_wo_version after import to push your secret values to the API.

terraform import claude-managed-agents_vault_credential.linear vlt_01HqR2k7vXbZ9mNpL3wYcT8f:cred_01HABCDEF...
