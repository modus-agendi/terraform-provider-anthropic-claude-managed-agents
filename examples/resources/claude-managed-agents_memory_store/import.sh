#!/usr/bin/env bash
# Import an existing memory store by its `memstore_*` id.
#
# delete_on_destroy has no upstream representation: imports always start
# at the default (false). Reapply with delete_on_destroy = true if needed.

terraform import claude-managed-agents_memory_store.user_preferences memstore_01HqR2k7vXbZ9mNpL3wYcT8f
