NAMESPACE   ?= andasv
NAME        ?= claude-managed-agents
BINARY      := terraform-provider-$(NAME)
VERSION     ?= 0.0.1-dev
OS_ARCH     ?= $(shell go env GOOS)_$(shell go env GOARCH)
PLUGIN_DIR  := $(HOME)/.terraform.d/plugins/registry.terraform.io/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

.PHONY: build install test test-no-race testacc testacc-live sweep coverage coverage-html lint fmt vet docs docs-check clean

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(PLUGIN_DIR)
	mv $(BINARY) $(PLUGIN_DIR)/$(BINARY)

# Race detector on by default for unit tests. Use test-no-race when
# debugging a flaky test where -race would change scheduling.
test:
	go test -race ./... -count=1

test-no-race:
	go test ./... -count=1

# Acceptance against the in-process httptest fixture (free, no API calls).
testacc:
	TF_ACC=1 go test ./internal/provider/... -v -count=1 -timeout 30m

# Acceptance against the real Anthropic API. Requires ANTHROPIC_API_KEY.
testacc-live:
	TF_ACC=1 TF_ACC_LIVE=1 go test ./internal/provider/... -v -count=1 -timeout 30m

# Archive any agents whose name starts with `tf-acc-test-` and are older
# than 1 hour. Requires ANTHROPIC_API_KEY. Safe to run repeatedly.
sweep:
	go test ./internal/provider/... -v \
		-sweep=anthropic \
		-sweep-run=claude-managed-agents_agent \
		-timeout 10m

# Coverage across the whole module (unit + httptest acceptance).
coverage:
	TF_ACC=1 go test \
		-coverprofile=coverage.out \
		-coverpkg=./internal/... \
		-timeout 30m \
		./...
	go tool cover -func=coverage.out | tail -1

coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser."

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

docs:
	go generate ./...

# Validate doc structure: every resource/data source has a page and
# documented attributes. Run after `make docs`.
docs-check:
	go run github.com/bflad/tfproviderdocs check \
		-provider-name claude-managed-agents \
		./

clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf dist
