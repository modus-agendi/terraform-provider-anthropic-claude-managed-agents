# Makefile for terraform-provider-claude-managed-agents.
#
# Run `make` (or `make help`) for a list of targets.
#
# Conventions (also see CLAUDE.md):
#   - New developer workflow command? Add a Make target with a `## comment`
#     so it shows up in `make help`. Don't scatter raw `go test ...` calls.
#   - Tooling versions are pinned at the top. Bump in PRs; CI uses the same
#     versions.
#   - `.env` is auto-loaded if present. Keep it simple KEY=VALUE only.

SHELL          := /bin/bash
.DEFAULT_GOAL  := help
MAKEFLAGS      += --no-print-directory

# ---- Project identity -----------------------------------------------------

NAMESPACE      ?= andasv
NAME           ?= claude-managed-agents
BINARY         := terraform-provider-$(NAME)
PROVIDER_TYPE  := claude-managed-agents
MODULE         := github.com/$(NAMESPACE)/terraform-provider-$(NAME)

# ---- Versioning -----------------------------------------------------------

VERSION        ?= 0.0.1-dev
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

# ---- Paths ----------------------------------------------------------------

OS             := $(shell go env GOOS)
ARCH           := $(shell go env GOARCH)
OS_ARCH        := $(OS)_$(ARCH)
PLUGIN_DIR     := $(HOME)/.terraform.d/plugins/registry.terraform.io/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)
LOCAL_BIN      := $(abspath ./bin)
COVERAGE_OUT   ?= coverage.out
COVERAGE_HTML  ?= coverage.html

# ---- Test parameterization (override on the command line) -----------------
#   make test RUN=TestDo_429
#   make testacc TIMEOUT=10m PARALLEL=2 TESTARGS="-v"

TEST           ?= ./...
TESTARGS       ?=
RUN            ?=
PARALLEL       ?= 4
TIMEOUT        ?= 30m
COUNT          ?= 1

# Compose `-run <regex>` only when RUN is set, so an empty RUN does not
# accidentally run zero tests.
ifeq ($(strip $(RUN)),)
RUN_FLAG       :=
else
RUN_FLAG       := -run '$(RUN)'
endif

# ---- Tooling pins (keep in sync with .github/workflows/test.yml) ----------

GOLANGCI_LINT_VERSION  := v1.62.0
TFPLUGINDOCS_VERSION   := v0.20.1
TFPROVIDERDOCS_VERSION := v0.12.1

# ---- .env auto-load -------------------------------------------------------
# Format: simple KEY=VALUE per line; no quotes, no shell substitution.
# Anything more elaborate, set via your shell environment instead.

ifneq (,$(wildcard ./.env))
include .env
export
endif

# ===========================================================================
##@ General

.PHONY: help
help: ## Show this help with command descriptions
	@awk 'BEGIN { \
		FS = ":.*##"; \
		printf "\nUsage: \033[1mmake \033[36m<target>\033[0m [VAR=value …]\n" \
	} \
	/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } \
	/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@echo

# ===========================================================================
##@ Build

.PHONY: build
build: ## Compile the provider binary into ./bin
	@mkdir -p $(LOCAL_BIN)
	@echo "==> building $(BINARY) (version=$(VERSION) commit=$(COMMIT))"
	@go build \
		-trimpath \
		-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)" \
		-o $(LOCAL_BIN)/$(BINARY) \
		.

.PHONY: install
install: build ## Build + install into the local Terraform plugin cache
	@mkdir -p $(PLUGIN_DIR)
	@cp $(LOCAL_BIN)/$(BINARY) $(PLUGIN_DIR)/$(BINARY)
	@echo "==> installed to $(PLUGIN_DIR)"

# ===========================================================================
##@ Test

.PHONY: test
test: ## Run unit + integration tests with -race (override: RUN, TESTARGS, COUNT)
	go test -race -count=$(COUNT) $(RUN_FLAG) $(TESTARGS) -timeout 5m $(TEST)

.PHONY: test-no-race
test-no-race: ## Run tests without -race (for debugging schedule-sensitive bugs)
	go test -count=$(COUNT) $(RUN_FLAG) $(TESTARGS) -timeout 5m $(TEST)

.PHONY: testacc
testacc: ## Run acceptance tests against the in-process httptest fixture
	TF_ACC=1 go test \
		-count=$(COUNT) \
		-parallel $(PARALLEL) \
		$(RUN_FLAG) $(TESTARGS) \
		-timeout $(TIMEOUT) \
		./internal/provider/...

.PHONY: testacc-live
testacc-live: require-api-key ## Acceptance tests against api.anthropic.com (needs ANTHROPIC_API_KEY)
	TF_ACC=1 TF_ACC_LIVE=1 go test \
		-count=$(COUNT) \
		-parallel $(PARALLEL) \
		$(RUN_FLAG) $(TESTARGS) \
		-timeout $(TIMEOUT) \
		./internal/provider/...

.PHONY: test-compile
test-compile: ## Compile the test binary (catches compile errors without running tests)
	go test -c -o /dev/null ./internal/...

# ===========================================================================
##@ Sweep

.PHONY: sweep
sweep: require-api-key ## Archive orphan tf-acc-test-* agents older than 1h
	@echo "==> sweeping orphan test agents older than 1h"
	go test ./internal/provider/... -v \
		-sweep=anthropic \
		-sweep-run=$(PROVIDER_TYPE)_agent \
		-timeout 10m

.PHONY: sweep-allow-failures
sweep-allow-failures: require-api-key ## Sweep but keep going on errors
	go test ./internal/provider/... -v \
		-sweep=anthropic \
		-sweep-run=$(PROVIDER_TYPE)_agent \
		-sweep-allow-failures \
		-timeout 10m

# ===========================================================================
##@ Coverage

.PHONY: coverage
coverage: ## Generate coverage profile (uses TF_ACC=1 so all layers count)
	TF_ACC=1 go test \
		-coverprofile=$(COVERAGE_OUT) \
		-coverpkg=./internal/... \
		-covermode=atomic \
		-timeout $(TIMEOUT) \
		./...
	@go tool cover -func=$(COVERAGE_OUT) | tail -1

.PHONY: coverage-html
coverage-html: coverage ## Generate coverage profile + render HTML report
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "==> open $(COVERAGE_HTML) in your browser"

.PHONY: coverage-show
coverage-show: ## Print per-function coverage breakdown (uses existing coverage.out)
	@test -f $(COVERAGE_OUT) || { echo "no $(COVERAGE_OUT); run 'make coverage' first"; exit 1; }
	@go tool cover -func=$(COVERAGE_OUT)

.PHONY: coverage-split
coverage-split: ## Mirror CI: produce coverage-unit.out (client only) + coverage-acc.out (all internal w/ TF_ACC=1)
	go test -race \
		-coverprofile=coverage-unit.out \
		-coverpkg=./internal/client/... \
		-covermode=atomic \
		-count=1 \
		-timeout 5m \
		./internal/client/...
	@go tool cover -func=coverage-unit.out | tail -1
	TF_ACC=1 go test \
		-coverprofile=coverage-acc.out \
		-coverpkg=./internal/... \
		-covermode=atomic \
		-count=1 \
		-parallel 4 \
		-timeout $(TIMEOUT) \
		./internal/provider/...
	@go tool cover -func=coverage-acc.out | tail -1

# ===========================================================================
##@ Code quality

.PHONY: fmt
fmt: ## Format Go code with gofmt -s
	gofmt -s -w .

.PHONY: fmtcheck
fmtcheck: ## Verify gofmt cleanliness (CI gate)
	@diff=$$(gofmt -s -d .) ; \
	if [ -n "$$diff" ] ; then \
		echo "$$diff" ; \
		echo "" ; \
		echo "gofmt found issues; run 'make fmt' to fix." ; \
		exit 1 ; \
	fi

.PHONY: vet
vet: ## Run go vet across all packages
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint with the project ruleset
	@if command -v golangci-lint >/dev/null 2>&1 ; then \
		golangci-lint run ./... ; \
	elif [ -x "$(LOCAL_BIN)/golangci-lint" ] ; then \
		$(LOCAL_BIN)/golangci-lint run ./... ; \
	else \
		echo "golangci-lint not found on PATH or in $(LOCAL_BIN); run 'make tools'" ; \
		exit 1 ; \
	fi

.PHONY: depscheck
depscheck: ## Verify go.mod / go.sum are tidy
	go mod tidy
	@if ! git diff --exit-code -- go.mod go.sum ; then \
		echo "" ; \
		echo "go.mod / go.sum drift; run 'go mod tidy' and commit." ; \
		exit 1 ; \
	fi

# ===========================================================================
##@ Docs

.PHONY: docs
docs: ## Regenerate docs via tfplugindocs
	go generate ./...

.PHONY: docs-check
docs-check: ## Validate doc structure with tfproviderdocs
	go run github.com/bflad/tfproviderdocs check -provider-name $(PROVIDER_TYPE) ./

.PHONY: docscheck
docscheck: docs docs-check ## Regen docs + verify no diff + tfproviderdocs (CI gate)
	@if ! git diff --exit-code -- docs/ ; then \
		echo "" ; \
		echo "docs/ is out of date; run 'make docs' and commit." ; \
		exit 1 ; \
	fi

# ===========================================================================
##@ Tooling

.PHONY: tools
tools: ## Install dev tooling at pinned versions into ./bin
	@mkdir -p $(LOCAL_BIN)
	@echo "==> installing golangci-lint $(GOLANGCI_LINT_VERSION)"
	@GOBIN=$(LOCAL_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "==> installing tfplugindocs $(TFPLUGINDOCS_VERSION)"
	@GOBIN=$(LOCAL_BIN) go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)
	@echo "==> installing tfproviderdocs $(TFPROVIDERDOCS_VERSION)"
	@GOBIN=$(LOCAL_BIN) go install github.com/bflad/tfproviderdocs@$(TFPROVIDERDOCS_VERSION)
	@echo ""
	@echo "==> tools installed into $(LOCAL_BIN)"
	@echo "    add to PATH: export PATH=\"$(LOCAL_BIN):\$$PATH\""

# ===========================================================================
##@ Composite

.PHONY: pr
pr: depscheck fmtcheck vet lint test testacc docscheck ## Run all PR checks locally (mirrors CI)
	@echo ""
	@echo "==> all PR checks passed"

.PHONY: clean
clean: ## Remove build artifacts, coverage files, and ./bin
	rm -rf $(LOCAL_BIN) dist $(COVERAGE_OUT) $(COVERAGE_HTML)
	@echo "==> cleaned"

# ===========================================================================
# Internal helpers (no help comment → not listed in `make help`)

.PHONY: require-api-key
require-api-key:
	@if [ -z "$${ANTHROPIC_API_KEY}" ] ; then \
		echo "" ; \
		echo "ANTHROPIC_API_KEY is not set." ; \
		echo "" ; \
		echo "Options:" ; \
		echo "  - put 'ANTHROPIC_API_KEY=sk-...' in .env (auto-loaded)" ; \
		echo "  - or export it in your shell before running make" ; \
		echo "" ; \
		exit 1 ; \
	fi
