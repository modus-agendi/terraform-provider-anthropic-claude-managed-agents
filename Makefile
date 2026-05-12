NAMESPACE   ?= asvirida
NAME        ?= claude-managed-agents
BINARY      := terraform-provider-$(NAME)
VERSION     ?= 0.0.1-dev
OS_ARCH     ?= $(shell go env GOOS)_$(shell go env GOARCH)
PLUGIN_DIR  := $(HOME)/.terraform.d/plugins/registry.terraform.io/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

.PHONY: build install test testacc testacc-live lint fmt vet docs clean

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(PLUGIN_DIR)
	mv $(BINARY) $(PLUGIN_DIR)/$(BINARY)

test:
	go test ./... -count=1

testacc:
	TF_ACC=1 go test ./internal/provider/... -v -count=1 -timeout 30m

testacc-live:
	TF_ACC=1 TF_ACC_LIVE=1 go test ./internal/provider/... -v -count=1 -timeout 30m

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

docs:
	go generate ./...

clean:
	rm -f $(BINARY)
	rm -rf dist
