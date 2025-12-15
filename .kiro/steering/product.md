# Product Overview

## Product Description

Example EC Platform は、Connect-go BFF + gRPC マイクロサービス構成を学習・検証するためのサンプル EC プラットフォームです。実際の EC サイト開発で使用されるパターンとベストプラクティスを実装しています。

## Core Features

### 認証・認可システム
- Ory Hydra v2.2 による OAuth2/OIDC 認証
- JWT トークンによるステートレス認証
- JWKS キャッシュによる高速トークン検証
- Login/Consent Provider パターン実装

### BFF (Backend for Frontend)
- Connect-go による gRPC-Web 互換 API
- フロントエンド向けプロトコル変換
- JWT ミドルウェアによるアクセス制御
- 各マイクロサービスへのルーティング

### ユーザー管理
- ユーザー登録・認証
- プロフィール管理
- セッション管理

### 商品管理
- 商品 CRUD 操作
- 在庫管理
- ページネーション対応リスト表示

### 注文・カート機能
- 永続化カート
- 注文作成（冪等性キー対応）
- 注文履歴管理

## Target Use Case

### 学習用途
- gRPC/Connect-go マイクロサービス構成の学習
- OAuth2/OIDC 認証フローの理解
- クリーンアーキテクチャの実践

### 検証用途
- BFF パターンの検証
- サービス間通信パターンの評価
- データベース設計パターンの検証

### テンプレート用途
- 新規 Go マイクロサービスプロジェクトの雛形
- CI/CD パイプラインの参考実装

## Key Value Proposition

### セキュリティ
- BOLA (Broken Object Level Authorization) 対策を標準実装
- 全クエリで user_id による絞り込み
- JWT ローカル検証による高速・安全な認証

### スケーラビリティ
- サービス間 FK 制約なしの疎結合設計
- スキーマ分離による独立デプロイ
- 冪等性キーによる重複リクエスト防止

### 開発体験
- Go Workspace による統合開発
- Buf v2 による型安全な API 開発
- Makefile による一貫した開発コマンド

## Business Context

このプロジェクトは商用製品ではなく、教育・検証目的のサンプル実装です。
実際の本番環境への適用時には、以下の考慮が必要です：

- シークレット管理の強化
- 本番用 Hydra 設定
- 監視・ロギング基盤の追加
- パフォーマンステスト・負荷テスト
