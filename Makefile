GOLANGCI_LINT_VERSION := v2.15.0
BINARY := bin/metrics-generator

.PHONY: help build fmt vet lint lint-version test

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "%-14s %s\n", $$1, $$2}'

build: ## Build the CLI into bin/
	mkdir -p $(dir $(BINARY))
	go build -o $(BINARY) ./cmd/metrics-generator

fmt: ## Format Go source files
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

lint-version:
	@echo $(GOLANGCI_LINT_VERSION)

test: ## Run tests with the race detector
	go test -race ./...
