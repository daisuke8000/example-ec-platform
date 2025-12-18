# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Connect-go BFF + gRPC microservices e-commerce platform with Ory Hydra OAuth2/OIDC authentication.

## Development Commands

```bash
# Infrastructure (PostgreSQL, Redis, Hydra)
make docker-up          # Start all infrastructure
make docker-down        # Stop infrastructure
make docker-clean       # Remove volumes and containers

# Proto generation
make proto              # Generate Go code from proto files
make proto-lint         # Lint proto files
make proto-breaking     # Check for breaking changes

# Build
make build              # Build all services
make build-bff          # Build BFF only
make build-user         # Build User service only

# Run services (development)
make run-bff            # Run BFF (port 8080)
make run-user           # Run User service (port 50051)
make run-product        # Run Product service (port 50052)
make run-order          # Run Order service (port 50053)

# Test
make test               # Run all tests with race detection
make test-bff           # Run BFF tests only
make test-user          # Run User service tests only
go test -race ./services/user/internal/handler/...  # Run specific package tests

# Lint and format
make lint               # Run golangci-lint
make fmt                # Format code (go fmt + gofumpt)
make vet                # Run go vet

# Dependencies
make deps               # Tidy and download all module dependencies
make install-tools      # Install required development tools
```

## Architecture

### Service Communication Flow
```
Frontend (SPA) --[gRPC-Web + Bearer Token]--> BFF (Connect-go :8080)
                                               |
                                               +--> Hydra (OAuth2 :4444/:4445)
                                               |
                    [gRPC]                     v
User Service (:50051) <--+--> BFF <--+--> Product Service (:50052)
                                     +--> Order Service (:50053)
                         |
                         v
                    PostgreSQL (schema-isolated)
```

### Module Structure (Go Workspace)

This is a multi-module Go workspace. All modules are linked via `go.work`:

| Module | Path | Description |
|--------|------|-------------|
| `gen` | `./gen` | Generated protobuf/gRPC/Connect code |
| `bff` | `./bff` | Connect-go BFF (JWT validation, protocol bridge) |
| `user` | `./services/user` | User service + Hydra Login/Consent Provider |
| `product` | `./services/product` | Product CRUD, inventory management |
| `order` | `./services/order` | Orders, persistent cart, idempotency |

Each service follows this internal structure:
```
services/{name}/
├── cmd/server/         # Main entrypoint
├── internal/
│   ├── handler/        # gRPC handlers
│   ├── repository/     # Database access
│   └── service/        # Business logic
└── migrations/         # SQL migrations
```

### Key Patterns

- **BFF Responsibility**: Protocol translation and JWT verification only. No business logic.
- **JWT Verification**: Local validation using cached JWKS from Hydra (via Redis).
- **Database Isolation**: Each service uses a separate PostgreSQL schema. No cross-service FK constraints.
- **BOLA Prevention**: All queries filter by `user_id` from JWT claims.
- **Idempotency**: Order Service implements idempotency keys for CreateOrder.
- **Soft Deletes**: All tables use logical deletion (`deleted_at` timestamp).

### Proto Management (Buf v2)

Proto files are in `proto/` directory. Generated code goes to `gen/`:
- `buf generate` creates Go, gRPC-Go, and Connect-go code
- Generated packages are imported as `github.com/daisuke8000/example-ec-platform/gen/...`
- Services reference `gen` module via `replace` directive in their `go.mod`

## Infrastructure Dependencies

- **PostgreSQL 16**: Main database (port 5432, db: `ecplatform`)
- **Redis 7**: JWKS cache, idempotency keys (port 6379)
- **Ory Hydra v2.2**: OAuth2/OIDC server (public: 4444, admin: 4445)

## Environment Setup

Copy `.env.example` to `.env` for local development. Key variables:
- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string
- `HYDRA_PUBLIC_URL` / `HYDRA_ADMIN_URL`: Hydra endpoints
- `*_SERVICE_ADDR`: Backend service addresses for BFF

## Active Specifications

| Feature | Phase | Status |
|---------|-------|--------|
| `user-service-hydra-auth` | initialized | Awaiting requirements generation |
| `bff-jwt-verification` | initialized | BFF JWT validation middleware with JWKS caching |
| `product-service` | initialized | Product CRUD, inventory management, catalog service |

Use `/kiro:spec-status <feature-name>` to check detailed progress.