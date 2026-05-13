#!/usr/bin/env bash
# Import an existing environment by its `env_*` id.
# Note: every config attribute is RequiresReplace, so imports are useful
# mostly to bring an externally-created environment under Terraform state.

terraform import claude-managed-agents_environment.data_science env_01HqR2k7vXbZ9mNpL3wYcT8f
