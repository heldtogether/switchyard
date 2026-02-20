.PHONY: help build test clean docker-build migrate-up migrate-down sqlc-generate dev-up dev-down

# Variables
BINARY_API=bin/api
BINARY_WORKER=bin/worker
BINARY_MIGRATE=bin/migrate
DOCKER_REGISTRY?=ghcr.io/heldtogether
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build all binaries
	@echo "Building binaries..."
	@mkdir -p bin
	go build -ldflags="-X github.com/heldtogether/switchyard/internal/version.Version=$(VERSION)" -o $(BINARY_API) ./cmd/api
	go build -ldflags="-X github.com/heldtogether/switchyard/internal/version.Version=$(VERSION)" -o $(BINARY_WORKER) ./cmd/worker
	go build -o $(BINARY_MIGRATE) ./cmd/migrate

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out

docker-build: ## Build Docker images
	docker build -f build/api.Dockerfile -t $(DOCKER_REGISTRY)/switchyard-api:$(VERSION) .
	docker build -f build/worker.Dockerfile -t $(DOCKER_REGISTRY)/switchyard-worker:$(VERSION) .
	docker build -f build/example-job/Dockerfile -t $(DOCKER_REGISTRY)/switchyard-example-job:$(VERSION) ./build/example-job

docker-push: ## Push Docker images
	docker push $(DOCKER_REGISTRY)/switchyard-api:$(VERSION)
	docker push $(DOCKER_REGISTRY)/switchyard-worker:$(VERSION)
	docker push $(DOCKER_REGISTRY)/switchyard-example-job:$(VERSION)

migrate-up: ## Run database migrations up
	@echo "Running migrations..."
	go run ./cmd/migrate -dir ./migrations -action up

migrate-down: ## Rollback last migration
	@echo "Rolling back migration..."
	go run ./cmd/migrate -dir ./migrations -action down

migrate-create: ## Create new migration (usage: make migrate-create NAME=add_users_table)
	@if [ -z "$(NAME)" ]; then echo "NAME is required. Usage: make migrate-create NAME=your_migration_name"; exit 1; fi
	@echo "Creating migration: $(NAME)"
	migrate create -ext sql -dir migrations -seq $(NAME)

sqlc-generate: ## Generate sqlc code (if using sqlc)
	@echo "Generating sqlc code..."
	sqlc generate -f internal/storage/postgres/sqlc.yaml

dev-up: ## Start local development environment
	docker-compose -f deployments/docker-compose.yml up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Running migrations..."
	@$(MAKE) migrate-up

dev-down: ## Stop local development environment
	docker-compose -f deployments/docker-compose.yml down -v

dev-logs: ## Show logs from development environment
	docker-compose -f deployments/docker-compose.yml logs -f

fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

lint: ## Run linter
	golangci-lint run ./... || true

tidy: ## Tidy dependencies
	go mod tidy

.DEFAULT_GOAL := help
