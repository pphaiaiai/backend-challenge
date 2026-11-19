.DEFAULT_GOAL := help

.PHONY: help run build up down logs test test-race test-integration cover vet fmt proto tools

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

run: ## Run the API locally, loading .env if present
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/api

build: ## Build the API binary into bin/api
	go build -o bin/api ./cmd/api

up: ## Start MongoDB and the API with Docker Compose
	docker compose up --build -d

down: ## Stop the containers (MongoDB data is kept in a volume)
	docker compose down

logs: ## Follow the API logs
	docker compose logs -f api

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

proto: ## Regenerate gRPC code from proto/ (needs protoc and the plugins from `make tools`)
	protoc -I proto \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		proto/user/v1/user.proto

tools: ## Install the protoc plugins used by `make proto`
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
