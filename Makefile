.DEFAULT_GOAL := help

.PHONY: help run build test test-race test-integration cover vet fmt

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

run: ## Run the API locally, loading .env if present
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/api

build: ## Build the API binary into bin/api
	go build -o bin/api ./cmd/api

test: ## Run unit tests
	go test ./...

test-race: ## Run unit tests with the race detector
	go test -race ./...

test-integration: ## Run MongoDB integration tests (needs MONGO_URI)
	go test -race -tags=integration ./internal/adapters/outbound/mongodb/...

cover: ## Run tests and print total coverage
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -s -w .
