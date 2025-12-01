.PHONY: help fmt lint lint-fast test bench tidy tools

GO ?= go
GOIMPORTS ?= $(shell command -v goimports 2>/dev/null)

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"} {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

fmt: tools ## Format Go sources with gofmt + goimports
	@find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w
	@if [ -n "$(GOIMPORTS)" ]; then \
		echo "goimports"; \
		find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 $(GOIMPORTS) -w; \
	else \
		echo "goimports not found; skipping import formatting"; \
	fi

lint: ## Run golangci-lint for the whole repo
	golangci-lint run ./...

lint-fast: ## Run golangci-lint only for changes vs origin/main
	golangci-lint run --new-from-rev=origin/main

test: ## Run all Go tests
	$(GO) test ./...

bench: ## Run every benchmark across the repo
	$(GO) test ./... -run ^$$ -bench . -benchmem

tidy: ## Run go mod tidy + verify
	$(GO) mod tidy
	$(GO) mod verify

tools:
	@command -v gofmt >/dev/null || (echo "gofmt not found" && exit 1)
