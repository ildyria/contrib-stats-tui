BINARY      := contrib-stats
PKG         := github.com/ildyria/contrib-stats-tui
VERSION_PKG := $(PKG)/internal/gitstats
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -X $(VERSION_PKG).Version=$(VERSION)
BIN_DIR     := bin

# TEST_REPO is used by integration tests that scan a real git repository.
TEST_REPO   ?=

.DEFAULT_GOAL := build

.PHONY: help build run install test vet fmt tidy clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary into ./bin
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

run: ## Build and run against the current directory (use ARGS="..." to pass flags)
	go run -ldflags "$(LDFLAGS)" . $(ARGS)

install: ## Install the binary into $GOBIN / $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" .

test: ## Run tests (set TEST_REPO=/path/to/repo to enable integration tests)
	TEST_REPO=$(TEST_REPO) go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format the code
	gofmt -w -s .

tidy: ## Tidy module dependencies
	go mod tidy

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
