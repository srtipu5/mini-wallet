.DEFAULT_GOAL := help

TEST_DATABASE_URL ?= postgres://wallet:wallet@localhost:$(or $(DB_PORT),5432)/wallet_test?sslmode=disable

.PHONY: help up up-d down restart logs ps db test test-local test-unit fmt vet psql clean

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

up: ## Build and run the API + Postgres (foreground)
	docker compose up --build

up-d: ## Build and run the API + Postgres in the background
	docker compose up --build -d

down: ## Stop and remove containers
	docker compose down

restart: down up-d ## Restart everything in the background

logs: ## Follow API logs
	docker compose logs -f api

ps: ## Show container status
	docker compose ps

db: ## Start only Postgres in the background
	docker compose up -d db

test: ## Run all tests in Docker (no local Go needed)
	docker compose --profile test run --rm tests

test-local: db ## Run all tests with local Go against the Docker Postgres
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -count=1 ./...

test-unit: ## Run unit tests only (no database)
	go test -race -count=1 ./internal/...

fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

psql: ## Open a psql shell in the database container
	docker compose exec db psql -U wallet -d wallet

clean: ## Stop containers and delete volumes (wipes the database)
	docker compose --profile test down -v
