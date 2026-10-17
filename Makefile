GOLANGCI_LINT_VERSION := v2.15.0

.PHONY: lint lint-version test

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

lint-version:
	@echo $(GOLANGCI_LINT_VERSION)

test:
	go test ./...
