BINARY    := mpc_editor
CMD       := ./cmd/mpc_editor
GOFLAGS   := -trimpath
LDFLAGS   := -s -w
LINT_VER  := v2.11.4

.PHONY: all build run test lint vet fmt check clean install dev generate help

## —— Primary targets ——

all: check build  ## Run all checks then build

build:  ## Compile the binary
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) $(CMD)

run:  ## Build and run the server
	go run $(CMD)

install:  ## Install to $GOPATH/bin
	go install $(GOFLAGS) -ldflags '$(LDFLAGS)' $(CMD)

## —— Quality ——

test:  ## Run all tests
	go test ./...

test-v:  ## Run all tests with verbose output
	go test -v ./...

test-race:  ## Run tests with race detector
	go test -race ./...

test-cover:  ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "HTML report: go tool cover -html=coverage.out"

lint:  ## Run golangci-lint (installs if missing)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint $(LINT_VER)..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VER); \
	}
	golangci-lint run ./...

vet:  ## Run go vet
	go vet ./...

fmt:  ## Format code and check for drift
	gofmt -l -w .
	@test -z "$$(git diff --name-only)" || { echo "gofmt produced changes:"; git diff --name-only; exit 1; }

check: vet lint test  ## Run vet + lint + tests

generate:  ## Regenerate sqlc code from SQL definitions
	@command -v sqlc >/dev/null 2>&1 || { \
		echo "Installing sqlc..."; \
		go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest; \
	}
	cd internal/db && sqlc generate

## —— Development ——

dev:  ## Run with live rebuild on file changes (requires watchexec)
	@command -v watchexec >/dev/null 2>&1 || { echo "Install watchexec: brew install watchexec"; exit 1; }
	watchexec -r -e go,html,css,js -- go run $(CMD)

clean:  ## Remove build artifacts
	rm -f $(BINARY) coverage.out
	go clean -cache -testcache

## —— Help ——

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
