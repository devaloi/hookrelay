.PHONY: build run test lint clean fmt vet help

# Variables
BINARY_NAME=hookrelay
BUILD_DIR=build
GO=go

# Default target
all: lint test build

## Build commands
build: ## Build the binary
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build ## Build and run the server
	$(BUILD_DIR)/$(BINARY_NAME)

## Development commands
fmt: ## Format code
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: fmt vet ## Run linters
	golangci-lint run

## Test commands
test: ## Run tests
	$(GO) test -v -race ./...

test-coverage: ## Run tests with coverage
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

## Cleanup
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -f *.db *.db-journal *.db-shm *.db-wal

## Dependencies
deps: ## Download dependencies
	$(GO) mod download

tidy: ## Tidy go modules
	$(GO) mod tidy

## Help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
