.PHONY: help build test test-race cover fmt lint vet verify tidy tools setup-hooks clean

GO ?= go
HOOKS_SRC := .githooks
HOOKS_DEST := .git/hooks

LOCAL_BIN := $(CURDIR)/bin
GOLANGCI_LINT_VERSION ?= v2.11.4
# Prefer the pinned local install if present; otherwise fall back to PATH.
GOLANGCI_LINT ?= $(shell test -x $(LOCAL_BIN)/golangci-lint && echo $(LOCAL_BIN)/golangci-lint || echo golangci-lint)

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Compile the package.
	$(GO) build ./...

test: ## Run unit tests.
	$(GO) test ./...

test-race: ## Run unit tests with the race detector.
	$(GO) test -race ./...

cover: ## Run tests with coverage report.
	$(GO) test -coverprofile=coverage.txt -covermode=atomic ./...
	$(GO) tool cover -func=coverage.txt | tail -1

fmt: ## Format Go source.
	$(GO) fmt ./...

vet: ## Run go vet.
	$(GO) vet ./...

lint: ## Run golangci-lint.
	$(GOLANGCI_LINT) run ./...

tidy: ## Tidy go.mod and go.sum.
	$(GO) mod tidy

tools: ## Install pinned dev tools (golangci-lint) into ./bin.
	@mkdir -p $(LOCAL_BIN)
	GOBIN=$(LOCAL_BIN) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "Installed: $(LOCAL_BIN)/golangci-lint ($(GOLANGCI_LINT_VERSION))"

verify: fmt vet lint test ## Run the full local check suite. Run before pushing.

setup-hooks: ## Install pre-push and commit-msg git hooks.
	@mkdir -p $(HOOKS_DEST)
	@cp $(HOOKS_SRC)/pre-push $(HOOKS_DEST)/pre-push
	@cp $(HOOKS_SRC)/commit-msg $(HOOKS_DEST)/commit-msg
	@chmod +x $(HOOKS_DEST)/pre-push $(HOOKS_DEST)/commit-msg
	@echo "Installed hooks: pre-push, commit-msg"

clean: ## Remove build artefacts.
	rm -f coverage.txt coverage.html
	rm -rf bin dist
