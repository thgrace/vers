.PHONY: lint test

GOLANGCI_LINT_VERSION := v2.12.2
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint:
	go vet ./...
	$(GOLANGCI_LINT) run ./...

test:
	go test ./...
