#!/usr/bin/env bash
# Cloud Environment setup script for the bug-fix routine.
#
# Paste the contents of this file into the "Setup script" field of the
# Environment attached to this routine at claude.ai/code/routines. The cloud
# caches the result between runs, so this only re-executes when the
# environment is rebuilt.
#
# Goal: install everything the routine's gates need so `make test`,
# `make lint`, `make docs`, and (optionally) `TF_ACC_LIVE=1 make testacc`
# all run without "command not found".
#
# Versions below intentionally mirror what the project's own Makefile pins
# under `make tools`. Keep them in sync when you bump tooling in the repo.

set -euo pipefail

# ---- Go ---------------------------------------------------------------------
# Required by every Make target. Version should match the `go` directive in
# go.mod or the project's CI matrix, whichever is newer.
GO_VERSION="1.23.4"
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
  | sudo tar -xz -C /usr/local
echo 'export PATH=$PATH:/usr/local/go/bin:$(go env GOPATH 2>/dev/null)/bin' \
  | sudo tee -a /etc/profile.d/claude-routine.sh
export PATH="$PATH:/usr/local/go/bin"
export PATH="$PATH:$(go env GOPATH)/bin"

# ---- Terraform --------------------------------------------------------------
# Acceptance tests and tfproviderdocs both shell out to `terraform`.
TF_VERSION="1.10.0"
curl -fsSL "https://releases.hashicorp.com/terraform/${TF_VERSION}/terraform_${TF_VERSION}_linux_amd64.zip" \
  -o /tmp/terraform.zip
sudo unzip -o /tmp/terraform.zip -d /usr/local/bin
rm /tmp/terraform.zip

# ---- golangci-lint (make lint) ---------------------------------------------
# Mirror GOLANGCI_LINT_VERSION from the project Makefile. Bump in both places
# in the same PR when upgrading.
GOLANGCI_VERSION="v1.62.0"
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sudo sh -s -- -b /usr/local/bin "${GOLANGCI_VERSION}"

# ---- tfplugindocs / tfproviderdocs (make docs, make docs-check) ------------
# Mirror TFPLUGINDOCS_VERSION and TFPROVIDERDOCS_VERSION from the project
# Makefile.
go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.20.1
go install github.com/bflad/tfproviderdocs@v0.12.1

# ---- gh CLI -----------------------------------------------------------------
# Usually pre-installed in the routine image, but pin defensively.
if ! command -v gh >/dev/null 2>&1; then
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null
  sudo apt-get update -y
  sudo apt-get install -y gh
fi

echo "env-setup.sh: tooling install complete"
