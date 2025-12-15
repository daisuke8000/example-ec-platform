# Technology Stack

## Architecture Overview

```
Frontend (SPA)
     │ gRPC-Web + Bearer Token
     ▼
┌─────────────┐      ┌─────────────┐
│ Connect-go  │◀────▶│ Ory Hydra   │
│    BFF      │      │ (OAuth2)    │
└──────┬──────┘      └─────────────┘
       │ gRPC
       ▼
┌────────────────────────────────┐
│  User │ Product │ Order Service│
└───┬───┴────┬────┴───────┬──────┘
    └────────┴────────────┘
           PostgreSQL
```

### 通信パターン
- **Frontend → BFF**: gRPC-Web (HTTP/2) + Bearer Token
- **BFF → Services**: gRPC (HTTP/2)
- **BFF ↔ Hydra**: HTTP (OAuth2 フロー)

## Backend Stack

### Language & Runtime
- **Go 1.25**: 最新の Go バージョン
- **Go Workspace**: マルチモジュール管理 (`go.work`)

### API Layer
- **Connect-go**: BFF 用 gRPC-Web 互換フレームワーク
- **gRPC-go**: サービス間通信用

### Protocol Buffers
- **Buf v2**: Proto 管理・生成ツール
- **protoc-gen-go**: Go コード生成
- **protoc-gen-connect-go**: Connect ハンドラー生成
- **protoc-gen-go-grpc**: gRPC ハンドラー生成

### 認証
- **Ory Hydra v2.2**: OAuth2/OIDC サーバー
- **JWT**: アクセストークン形式
- **JWKS**: 公開鍵検証

## Infrastructure

### Database
- **PostgreSQL 16**: メインデータストア
- **Schema Isolation**: サービスごとに独立スキーマ
  - `user_schema`
  - `product_schema`
  - `order_schema`

### Cache
- **Redis 7**: キャッシュ・セッションストア
  - JWKS キャッシュ
  - 冪等性キー
  - セッションキャッシュ

### Container
- **Docker Compose**: 開発環境オーケストレーション

## Development Environment

### Required Tools
```bash
# 必須
go >= 1.25
buf >= 1.x (v2 config)
docker >= 24.x
docker-compose >= 2.x

# 推奨
golangci-lint >= 1.x
gofumpt >= 0.5.x
migrate >= 4.x
```

### Tool Installation
```bash
make install-tools
```

## Common Commands

### Infrastructure
```bash
make docker-up      # インフラ起動 (PostgreSQL, Redis, Hydra)
make docker-down    # インフラ停止
make docker-clean   # ボリューム削除
```

### Proto Generation
```bash
make proto          # Go コード生成
make proto-lint     # Proto リント
make proto-breaking # 破壊的変更チェック
```

### Build & Run
```bash
make build          # 全サービスビルド
make run-bff        # BFF 起動 (port 8080)
make run-user       # User Service 起動 (port 50051)
make run-product    # Product Service 起動 (port 50052)
make run-order      # Order Service 起動 (port 50053)
```

### Test & Quality
```bash
make test           # 全テスト実行 (-race 有効)
make lint           # golangci-lint
make fmt            # gofumpt フォーマット
```

### Dependencies
```bash
make deps           # 依存関係整理
```

## Environment Variables

### Database
```bash
DATABASE_URL=postgres://ecplatform:ecplatform_secret@localhost:5432/ecplatform?sslmode=disable
```

### Redis
```bash
REDIS_URL=localhost:6379
```

### Hydra
```bash
HYDRA_PUBLIC_URL=http://localhost:4444
HYDRA_ADMIN_URL=http://localhost:4445
```

### Service Addresses (for BFF)
```bash
USER_SERVICE_ADDR=localhost:50051
PRODUCT_SERVICE_ADDR=localhost:50052
ORDER_SERVICE_ADDR=localhost:50053
```

## Port Configuration

| Service | Port | Protocol |
|---------|------|----------|
| BFF | 8080 | HTTP/gRPC-Web |
| User Service | 50051 | gRPC |
| Product Service | 50052 | gRPC |
| Order Service | 50053 | gRPC |
| PostgreSQL | 5432 | PostgreSQL |
| Redis | 6379 | Redis |
| Hydra Public | 4444 | HTTP |
| Hydra Admin | 4445 | HTTP |

## Key Technical Decisions

### BFF 設計
- プロトコル変換と JWT 検証のみ
- ビジネスロジックなし
- 各サービスへの透過的プロキシ

### 認証設計
- Hydra による外部 IdP パターン
- JWKS ローカルキャッシュ (Redis)
- Bearer Token による API アクセス

### DB 設計
- サービス間 FK 制約なし（疎結合）
- 論理削除 (`deleted_at`)
- UUID 主キー

### セキュリティ
- BOLA 対策: 全クエリで `user_id` 絞り込み
- 冪等性キー: Order Service の CreateOrder
