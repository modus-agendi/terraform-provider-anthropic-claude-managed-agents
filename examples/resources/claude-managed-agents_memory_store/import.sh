#!/usr/bin/env bash
# Import an existing Claude Managed Agents memory store by its server-assigned
# id. The id is the `memstore_*` string returned by the API on create.
#
# Note: delete_on_destroy is provider-only state with no upstream
# representation, so imports always set it to its default (false). Reapply
# after import if you want delete_on_destroy = true.

terraform import claude-managed-agents_memory_store.user_prefs memstore_01HqR2k7vXbZ9mNpL3wYcT8f
