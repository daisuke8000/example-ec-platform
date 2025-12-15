# Example EC Platform

Connect-go BFF + gRPC マイクロサービス構成のサンプルプロジェクト

## アーキテクチャ

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

## サービス構成

| Service | Port | 役割 |
|---------|------|------|
| BFF | 8080 | API集約、JWT検証 |
| User | 50051 | 認証、Hydra Login/Consent Provider |
| Product | 50052 | 商品CRUD、在庫管理 |
| Order | 50053 | 注文、永続化カート |

## 技術スタック

- **Language**: Go 1.25
- **BFF**: Connect-go (gRPC-Web互換)
- **Backend**: gRPC-go
- **認証**: Ory Hydra v2.2 (OAuth2/OIDC)
- **DB**: PostgreSQL 16 (スキーマ分離)
- **Cache**: Redis 7 (JWKS, 冪等性キー)
- **Proto**: Buf v2
- **Container**: Docker Compose

## 開発コマンド

```bash
# インフラ起動
make docker-up

# Proto生成
make proto

# 全サービスビルド
make build

# テスト
make test

# 依存関係整理
make deps
```

## 設計指針

- **BFF責務**: プロトコル変換・JWT検証のみ（ビジネスロジックなし）
- **認証**: Hydra OAuth2 + JWKSキャッシュによるローカルJWT検証
- **DB設計**: サービス間FK制約なし + 論理削除
- **セキュリティ**: BOLA対策（全クエリでuser_id絞り込み）
- **冪等性**: Order ServiceのCreateOrderに冪等性キー実装

## ディレクトリ構造

```
example-ec-platform/
├── bff/                    # Connect-go BFF
├── services/
│   ├── user/               # User + Auth Service
│   ├── product/            # Product + Inventory Service
│   └── order/              # Order + Cart Service
├── proto/                  # Protocol Buffer定義
├── gen/                    # 生成コード
├── deployments/            # Docker, Hydra設定
├── buf.yaml                # Buf設定
├── go.work                 # Go Workspace
└── Makefile
```

## 実装ロードマップ

### Phase 0: 開発環境構築 ✅
- [x] Go Workspace 構成 (`go.work`)
- [x] Buf v2 セットアップ (`buf.yaml`, `buf.gen.yaml`)
- [x] Proto 定義 (user, product, order)
- [x] Docker Compose 構成 (PostgreSQL, Redis, Hydra)
- [x] Makefile 整備
- [x] 各サービスのスケルトン作成

### Phase 1: User Service + 認証基盤
- [ ] Hydra Login/Consent Provider 実装
- [ ] User CRUD (gRPC handlers)
- [ ] JWT 発行・検証フロー
- [ ] PostgreSQL migrations (users スキーマ)
- [ ] 単体テスト

### Phase 2: BFF + JWT検証
- [ ] Connect-go サーバー構築
- [ ] JWKS キャッシュ (Redis)
- [ ] JWT ミドルウェア実装
- [ ] User Service との gRPC 連携
- [ ] E2E テスト (認証フロー)

### Phase 3: Product Service
- [ ] 商品 CRUD (gRPC handlers)
- [ ] 在庫管理ロジック
- [ ] PostgreSQL migrations (products スキーマ)
- [ ] BFF との連携
- [ ] 単体テスト

### Phase 4: Order Service
- [ ] 注文作成・取得 (gRPC handlers)
- [ ] 永続化カート実装
- [ ] 冪等性キー (Redis)
- [ ] PostgreSQL migrations (orders スキーマ)
- [ ] Saga パターン検討 (在庫引き当て)

### Phase 5: 統合・最適化
- [ ] 全サービス統合テスト
- [ ] パフォーマンス計測・最適化
- [ ] ドキュメント整備
- [ ] CI/CD パイプライン

## API 設計概要

### User Service (port 50051)
| RPC | 説明 |
|-----|------|
| `CreateUser` | ユーザー登録 |
| `GetUser` | ユーザー情報取得 |
| `Login` | Hydra Login Provider |
| `Consent` | Hydra Consent Provider |

### Product Service (port 50052)
| RPC | 説明 |
|-----|------|
| `CreateProduct` | 商品登録 (管理者) |
| `GetProduct` | 商品詳細取得 |
| `ListProducts` | 商品一覧 (ページネーション) |
| `UpdateStock` | 在庫更新 |

### Order Service (port 50053)
| RPC | 説明 |
|-----|------|
| `CreateOrder` | 注文作成 (冪等性キー必須) |
| `GetOrder` | 注文詳細取得 |
| `ListOrders` | 注文履歴 |
| `AddToCart` | カート追加 |
| `GetCart` | カート取得 |

## DB スキーマ設計

### 共通方針
- 各サービスは独立スキーマ (`user_schema`, `product_schema`, `order_schema`)
- サービス間の FK 制約なし（疎結合）
- 全テーブルに `created_at`, `updated_at`, `deleted_at` (論理削除)
- UUID を主キーに使用

### users テーブル (user_schema)
```sql
id UUID PRIMARY KEY
email VARCHAR(255) UNIQUE NOT NULL
password_hash VARCHAR(255) NOT NULL
name VARCHAR(100)
created_at, updated_at, deleted_at
```

### products テーブル (product_schema)
```sql
id UUID PRIMARY KEY
name VARCHAR(255) NOT NULL
description TEXT
price DECIMAL(10,2) NOT NULL
stock_quantity INTEGER NOT NULL DEFAULT 0
created_at, updated_at, deleted_at
```

### orders テーブル (order_schema)
```sql
id UUID PRIMARY KEY
user_id UUID NOT NULL  -- FK制約なし、参照のみ
idempotency_key VARCHAR(64) UNIQUE
status VARCHAR(20) NOT NULL
total_amount DECIMAL(10,2) NOT NULL
created_at, updated_at, deleted_at
```
