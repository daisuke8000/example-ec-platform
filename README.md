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

| Service | Port | Protocol | 役割 |
|---------|------|----------|------|
| BFF | 8080 | Connect-go (HTTP/2) | API集約、JWT検証、BOLA保護 |
| User | 50051 (Connect) / 8051 (HTTP) | Connect-go | ユーザー管理、Hydra Login/Consent Provider |
| Product | 50052 | gRPC | 商品CRUD、在庫管理 |
| Order | 50053 | gRPC | 注文、永続化カート |

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

## E2Eテスト結果

```bash
# ヘルスチェック
GET /health → 200 OK
GET /ready  → 200 Ready

# ユーザー作成（公開エンドポイント）
POST /user.v1.UserService/CreateUser → 200 (user created)

# 認証テスト
POST /GetUser (認証なし)           → 401 Unauthenticated
POST /GetUser (不正トークン)        → 401 Unauthenticated
POST /GetUser (有効JWT, 自分のID)   → 404 Not Found (BOLA pass)
POST /GetUser (有効JWT, 他人のID)   → 403 Permission Denied (BOLA block)
```

### BOLA保護の動作確認
| 条件 | 結果 |
|------|------|
| JWT subject = リクエストID | ✅ 認可成功 |
| JWT subject ≠ リクエストID | ❌ 403拒否 |

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

### Phase 1: User Service + 認証基盤 ✅
- [x] Hydra Login/Consent Provider 実装
- [x] User CRUD (Connect-go handlers)
- [x] Hydra OAuth2 連携 (JWT発行はHydra担当)
- [x] PostgreSQL migrations (`deployments/init-db.sql`)
- [x] gRPC → Connect-go 移行完了
- [x] 単体テスト

### Phase 2: BFF + JWT検証 ✅
- [x] Connect-go サーバー構築
- [x] JWKS 取得・キャッシュ (Hydra → BFF)
- [x] JWT ミドルウェア実装 (RS256検証)
- [x] BOLA保護 (ユーザーIDチェック)
- [x] Rate Limiting (Token Bucket)
- [x] User Service との Connect-go 連携
- [x] E2E テスト (認証フロー)

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
- 各サービスは独立スキーマ (`user_service`, `product_service`, `order_service`)
- サービス間の FK 制約なし（疎結合）
- 全テーブルに `created_at`, `updated_at`, `deleted_at` (論理削除)
- UUID を主キーに使用
- 初期化: `deployments/init-db.sql`

### users テーブル (user_service)
```sql
id UUID PRIMARY KEY
email VARCHAR(255) UNIQUE NOT NULL
password_hash VARCHAR(255) NOT NULL
name VARCHAR(255)
is_deleted BOOLEAN DEFAULT FALSE
deleted_at TIMESTAMPTZ
created_at TIMESTAMPTZ DEFAULT NOW()
updated_at TIMESTAMPTZ DEFAULT NOW()
```

### products テーブル (product_service)
```sql
id UUID PRIMARY KEY
name VARCHAR(255) NOT NULL
description TEXT
price DECIMAL(10,2) NOT NULL
image_url VARCHAR(500)
is_deleted BOOLEAN DEFAULT FALSE
deleted_at TIMESTAMPTZ
created_at TIMESTAMPTZ DEFAULT NOW()
updated_at TIMESTAMPTZ DEFAULT NOW()
```

### inventory テーブル (product_service)
```sql
product_id UUID PRIMARY KEY REFERENCES products(id)
quantity INT NOT NULL DEFAULT 0
reserved INT NOT NULL DEFAULT 0
version INT NOT NULL DEFAULT 0
updated_at TIMESTAMPTZ DEFAULT NOW()
```

### orders テーブル (order_service)
```sql
id UUID PRIMARY KEY
user_id UUID NOT NULL  -- FK制約なし、参照のみ
status VARCHAR(50) NOT NULL DEFAULT 'pending'
total_amount DECIMAL(10,2) NOT NULL
shipping_address JSONB
idempotency_key VARCHAR(255)
created_at TIMESTAMPTZ DEFAULT NOW()
updated_at TIMESTAMPTZ DEFAULT NOW()
```
