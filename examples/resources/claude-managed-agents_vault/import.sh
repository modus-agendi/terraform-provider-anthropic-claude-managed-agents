#!/usr/bin/env bash
# Import an existing vault by its `vlt_*` id.
#
# delete_on_destroy has no upstream representation: imports always start
# at the default (false). Reapply if you want hard-delete on destroy.

terraform import claude-managed-agents_vault.alice vlt_01HqR2k7vXbZ9mNpL3wYcT8f
