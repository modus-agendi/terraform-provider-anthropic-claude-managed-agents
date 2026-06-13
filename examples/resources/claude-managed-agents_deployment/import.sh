#!/usr/bin/env bash
# Import an existing deployment by its `deployment_*` id (returned by the API
# on create). Replace the example id below with your real one.
#
# Note: the write-only github `authorization_token` is never returned by the
# API, so after import you must set it (and bump authorization_token_wo_version)
# in config before the next apply if the deployment mounts a github_repository.

terraform import claude-managed-agents_deployment.nightly_digest deployment_01HqR2k7vXbZ9mNpL3wYcT8f
