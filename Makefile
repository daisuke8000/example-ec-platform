# ==============================================================================
# EC-Platform Makefile
# Connect-go BFF + gRPC Microservices
# ==============================================================================

.PHONY: all build clean test lint proto deps docker-up docker-down help

# ------------------------------------------------------------------------------
# Variables
# ------------------------------------------------------------------------------
GO := go
BUF := buf
DOCKER_COMPOSE := docker compose -f deployments/docker-compose.yml

# Services
SERVICES := bff user product order
BFF_DIR := bff
USER_DIR := services/user
PRODUCT_DIR := services/product
ORDER_DIR := services/order

# Build output
BIN_DIR := bin

# ------------------------------------------------------------------------------
# Default target
# ------------------------------------------------------------------------------
all: proto build

# ------------------------------------------------------------------------------
# Proto Generation (Buf v2)
# ------------------------------------------------------------------------------
.PHONY: proto proto-lint proto-breaking proto-clean

proto: ## Generate Go code from proto files
	$(BUF) generate

proto-lint: ## Lint proto files
	$(BUF) lint

proto-breaking: ## Check for breaking changes
	$(BUF) breaking --against '.git#branch=main'

proto-clean: ## Clean generated proto files
	rm -rf gen/*

proto-update: ## Update buf dependencies
	$(BUF) dep update

# ------------------------------------------------------------------------------
# Build
# ------------------------------------------------------------------------------
.PHONY: build build-bff build-user build-product build-order

build: build-bff build-user build-product build-order ## Build all services

build-bff: ## Build BFF service
	$(GO) build -o $(BIN_DIR)/bff ./$(BFF_DIR)/cmd/server

build-user: ## Build User service
	$(GO) build -o $(BIN_DIR)/user ./$(USER_DIR)/cmd/server

build-product: ## Build Product service
	$(GO) build -o $(BIN_DIR)/product ./$(PRODUCT_DIR)/cmd/server

build-order: ## Build Order service
	$(GO) build -o $(BIN_DIR)/order ./$(ORDER_DIR)/cmd/server

# ------------------------------------------------------------------------------
# Run Services (Development)
# ------------------------------------------------------------------------------
.PHONY: run-bff run-user run-product run-order

run-bff: ## Run BFF service
	$(GO) run ./$(BFF_DIR)/cmd/server

run-user: ## Run User service
	$(GO) run ./$(USER_DIR)/cmd/server

run-product: ## Run Product service
	$(GO) run ./$(PRODUCT_DIR)/cmd/server

run-order: ## Run Order service
	$(GO) run ./$(ORDER_DIR)/cmd/server

# ------------------------------------------------------------------------------
# Test
# ------------------------------------------------------------------------------
.PHONY: test test-bff test-user test-product test-order test-coverage

test: ## Run all tests
	$(GO) test -race ./...

test-bff: ## Run BFF tests
	$(GO) test -race ./$(BFF_DIR)/...

test-user: ## Run User service tests
	$(GO) test -race ./$(USER_DIR)/...

test-product: ## Run Product service tests
	$(GO) test -race ./$(PRODUCT_DIR)/...

test-order: ## Run Order service tests
	$(GO) test -race ./$(ORDER_DIR)/...

test-coverage: ## Run tests with coverage
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# ------------------------------------------------------------------------------
# Lint & Format
# ------------------------------------------------------------------------------
.PHONY: lint fmt vet

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format Go code
	$(GO) fmt ./...
	gofumpt -l -w .

vet: ## Run go vet
	$(GO) vet ./...

# ------------------------------------------------------------------------------
# Dependencies
# ------------------------------------------------------------------------------
.PHONY: deps deps-tidy deps-download deps-verify

deps: deps-tidy deps-download ## Install and tidy dependencies

deps-tidy: ## Tidy all modules
	$(GO) work sync
	cd $(BFF_DIR) && $(GO) mod tidy
	cd $(USER_DIR) && $(GO) mod tidy
	cd $(PRODUCT_DIR) && $(GO) mod tidy
	cd $(ORDER_DIR) && $(GO) mod tidy
	cd gen && $(GO) mod tidy

deps-download: ## Download dependencies
	$(GO) mod download

deps-verify: ## Verify dependencies
	$(GO) mod verify

# ------------------------------------------------------------------------------
# Docker Infrastructure
# ------------------------------------------------------------------------------
.PHONY: docker-up docker-down docker-logs docker-ps

docker-up: ## Start infrastructure (PostgreSQL, Redis, Hydra)
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop infrastructure
	$(DOCKER_COMPOSE) down

docker-logs: ## Show infrastructure logs
	$(DOCKER_COMPOSE) logs -f

docker-ps: ## Show running containers
	$(DOCKER_COMPOSE) ps

docker-clean: ## Remove volumes and containers
	$(DOCKER_COMPOSE) down -v --remove-orphans

# ------------------------------------------------------------------------------
# Database Migrations
# ------------------------------------------------------------------------------
.PHONY: migrate-up migrate-down migrate-create

MIGRATE := migrate

migrate-up: ## Run all migrations
	$(MIGRATE) -path $(USER_DIR)/migrations -database "$(DATABASE_URL)" up
	$(MIGRATE) -path $(PRODUCT_DIR)/migrations -database "$(DATABASE_URL)" up
	$(MIGRATE) -path $(ORDER_DIR)/migrations -database "$(DATABASE_URL)" up

migrate-down: ## Rollback last migration
	$(MIGRATE) -path $(USER_DIR)/migrations -database "$(DATABASE_URL)" down 1
	$(MIGRATE) -path $(PRODUCT_DIR)/migrations -database "$(DATABASE_URL)" down 1
	$(MIGRATE) -path $(ORDER_DIR)/migrations -database "$(DATABASE_URL)" down 1

migrate-create: ## Create new migration (usage: make migrate-create name=create_users service=user)
	$(MIGRATE) create -ext sql -dir $(service)_DIR/migrations -seq $(name)

# ------------------------------------------------------------------------------
# Hydra OAuth2
# ------------------------------------------------------------------------------
.PHONY: hydra-clients hydra-token

hydra-clients: ## Create OAuth2 clients in Hydra
	hydra create oauth2-client \
		--endpoint http://localhost:4445 \
		--grant-type authorization_code,refresh_token \
		--response-type code \
		--scope openid,profile,email \
		--redirect-uri http://localhost:3000/callback \
		--name "EC Platform SPA"

hydra-token: ## Get test token (development only)
	@echo "Use: hydra token user --endpoint http://localhost:4444"

# ------------------------------------------------------------------------------
# Development Tools
# ------------------------------------------------------------------------------
.PHONY: tools install-tools

tools: ## Check required tools
	@echo "Checking required tools..."
	@which buf || echo "buf not found: go install github.com/bufbuild/buf/cmd/buf@latest"
	@which golangci-lint || echo "golangci-lint not found: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
	@which gofumpt || echo "gofumpt not found: go install mvdan.cc/gofumpt@latest"
	@which migrate || echo "migrate not found: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"

install-tools: ## Install development tools
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ------------------------------------------------------------------------------
# Clean
# ------------------------------------------------------------------------------
.PHONY: clean clean-all

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

clean-all: clean proto-clean ## Clean all generated files

# ------------------------------------------------------------------------------
# Help
# ------------------------------------------------------------------------------
help: ## Show this help
	@echo "EC-Platform Development Commands"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
