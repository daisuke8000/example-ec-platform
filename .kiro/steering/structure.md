# Project Structure

## Root Directory Organization

```
example-ec-platform/
├── .kiro/                  # Kiro steering & specs
│   ├── steering/           # Project context (product, tech, structure)
│   └── specs/              # Feature specifications
├── bff/                    # Connect-go BFF service
├── services/               # Backend microservices
│   ├── user/               # User + Auth service
│   ├── product/            # Product + Inventory service
│   └── order/              # Order + Cart service
├── proto/                  # Protocol Buffer definitions
│   ├── api/v1/             # Public API (BFF-exposed)
│   └── internal/v1/        # Internal service APIs
├── gen/                    # Generated code (protobuf/gRPC/Connect)
├── deployments/            # Infrastructure configuration
│   ├── docker-compose.yml  # Development infrastructure
│   └── hydra/              # Hydra OAuth2 configuration
├── buf.yaml                # Buf v2 configuration
├── buf.gen.yaml            # Buf code generation settings
├── go.work                 # Go Workspace definition
├── Makefile                # Development commands
└── README.md               # Project documentation
```

## Service Structure (Standard Pattern)

各マイクロサービスは以下の構造に従います：

```
services/{name}/
├── cmd/
│   └── server/
│       └── main.go         # Service entrypoint
├── internal/
│   ├── handler/            # gRPC handlers
│   │   └── {name}_handler.go
│   ├── service/            # Business logic
│   │   └── {name}_service.go
│   ├── repository/         # Database access
│   │   └── {name}_repository.go
│   └── config/             # Service configuration
│       └── config.go
├── migrations/             # SQL migration files
│   ├── 000001_create_{table}.up.sql
│   └── 000001_create_{table}.down.sql
└── go.mod                  # Service module definition
```

## BFF Structure

```
bff/
├── cmd/
│   └── server/
│       └── main.go         # BFF entrypoint
├── internal/
│   ├── handler/            # Connect handlers (public API)
│   │   ├── user_handler.go
│   │   ├── product_handler.go
│   │   └── order_handler.go
│   ├── middleware/         # HTTP/gRPC middleware
│   │   ├── auth.go         # JWT validation
│   │   └── logging.go      # Request logging
│   ├── client/             # gRPC clients to backend services
│   │   ├── user_client.go
│   │   ├── product_client.go
│   │   └── order_client.go
│   └── config/
│       └── config.go
└── go.mod
```

## Proto Structure

```
proto/
├── api/                    # Public API definitions
│   └── v1/
│       ├── user.proto      # User API (BFF-exposed)
│       ├── product.proto   # Product API (BFF-exposed)
│       └── order.proto     # Order API (BFF-exposed)
└── internal/               # Internal service definitions
    └── v1/
        ├── user_service.proto      # User service internal
        ├── product_service.proto   # Product service internal
        └── order_service.proto     # Order service internal
```

## Generated Code Structure

```
gen/
├── api/v1/                 # Generated from proto/api/v1/
│   ├── *.pb.go             # Protobuf messages
│   ├── *_grpc.pb.go        # gRPC server/client
│   └── *connect/*.go       # Connect handlers
├── internal/v1/            # Generated from proto/internal/v1/
│   ├── *.pb.go
│   └── *_grpc.pb.go
└── go.mod
```

## Code Organization Patterns

### Layered Architecture

```
Handler (Transport) → Service (Business) → Repository (Data)
```

- **Handler**: HTTP/gRPC リクエスト処理、入出力変換
- **Service**: ビジネスロジック、トランザクション管理
- **Repository**: データアクセス、SQL クエリ

### Dependency Injection

```go
// Constructor injection pattern
func NewUserHandler(svc *UserService) *UserHandler {
    return &UserHandler{svc: svc}
}
```

### Error Handling

```go
// Service layer errors
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
)

// Handler converts to gRPC/Connect status codes
```

## File Naming Conventions

### Go Files
- `{resource}_handler.go`: gRPC/Connect ハンドラー
- `{resource}_service.go`: ビジネスロジック
- `{resource}_repository.go`: データアクセス
- `config.go`: 設定読み込み
- `main.go`: エントリーポイント

### Proto Files
- `{resource}.proto`: API 定義（単数形）
- `{resource}_service.proto`: 内部サービス定義

### Migration Files
- `{number}_{description}.up.sql`: マイグレーション適用
- `{number}_{description}.down.sql`: ロールバック

## Import Organization

```go
import (
    // Standard library
    "context"
    "fmt"

    // Third-party
    "connectrpc.com/connect"
    "google.golang.org/grpc"

    // Project internal
    "github.com/sasakidaisuke/example-ec-platform/gen/api/v1"
    "github.com/sasakidaisuke/example-ec-platform/bff/internal/service"
)
```

## Key Architectural Principles

### 1. サービス境界
- 各サービスは独立したデータベーススキーマを持つ
- サービス間の直接 DB アクセス禁止
- gRPC による明示的なサービス間通信

### 2. BFF 責務の限定
- プロトコル変換（gRPC-Web → gRPC）
- JWT 検証
- リクエストルーティング
- **ビジネスロジックは持たない**

### 3. セキュリティ by Design
- BOLA 対策: 全クエリに `user_id` 条件
- JWT claims からユーザー識別
- 論理削除による監査証跡

### 4. API バージョニング
- `v1/` ディレクトリによるバージョン分離
- 破壊的変更は新バージョンで対応
- `buf breaking` による互換性チェック

## Module Dependencies

```
go.work
├── bff           → gen (protobuf/connect)
├── services/user → gen (protobuf/grpc)
├── services/product → gen
├── services/order → gen
└── gen           (standalone, no internal deps)
```

各サービスは `gen` モジュールのみを参照し、他サービスのコードは参照しません。
