# gowinproc - 実装タスク管理

## 📊 プロジェクト進捗概要

**全体進捗**: Phase 1-2 完了 + Phase 3 主要機能完了 + UI実装完了 (約75%)

- ✅ Phase 1: 基本プロセス管理とCloudflare統合 (100%)
- ✅ Phase 2: 監視機能とREST/gRPC API (100%)
- ⚠️ Phase 3: 更新機能とTunnel統合 (85%)
- ❌ Phase 4: 自動スケーリング (0%)
- ❌ Phase 5: API完成とテスト (10%)
- ✅ **追加機能**: Webダッシュボード + システムトレイ (100%)

---

## ✅ Phase 1: 基本プロセス管理とCloudflare統合 (完了)

### プロジェクト構造
- [x] ディレクトリ構造作成 (processes/, certs/, keys/, logs/, data/)
- [x] go.mod セットアップ
- [x] 基本パッケージ構成

### Certificate Manager
- [x] 自己署名証明書生成機能
  - [x] crypto/x509, crypto/rsa使用
  - [x] PEM形式保存 ([src/internal/certs/manager.go](src/internal/certs/manager.go))
  - [x] 有効期限チェック機能
  - [x] プロセスごとの証明書生成

### go_auth統合
- [x] cloudflare-auth-worker接続 ([src/internal/cloudflare/auth.go](src/internal/cloudflare/auth.go))
- [x] RSA鍵ペア自動生成 ([src/internal/cloudflare/keygen.go](src/internal/cloudflare/keygen.go))
- [x] Secret取得機能
- [x] チャレンジ-レスポンス認証

### Secret Manager
- [x] .envファイル読み込み (godotenv) ([src/internal/secrets/manager.go](src/internal/secrets/manager.go))
- [x] Cloudflare統合時の.env自動生成
- [x] KEY=VALUE形式出力
- [x] ファイルパーミッション設定 (0600)
- [x] 証明書パス自動注入
- [x] プロセス情報自動注入

### プロセス管理
- [x] 基本的なプロセス起動・停止機能 ([src/internal/process/manager.go](src/internal/process/manager.go))
- [x] .envファイルから環境変数読み込み
- [x] 証明書パス環境変数注入
- [x] プロセス情報環境変数注入
- [x] プロセス監視・自動再起動
- [x] 複数インスタンス管理

### 設定管理
- [x] YAML設定ファイル読み込み ([src/internal/config/loader.go](src/internal/config/loader.go))
- [x] スタンドアロンモード設定 ([config.standalone.yaml](config.standalone.yaml))
- [x] Cloudflare統合モード設定 ([config.cloudflare.yaml](config.cloudflare.yaml))
- [x] 環境変数サポート

### ヘルスチェック
- [x] 基本的なヘルスチェック実装 ([src/cmd/gowinproc/main.go:117](src/cmd/gowinproc/main.go#L117))

---

## ✅ Phase 2: 監視機能とREST/gRPC API (完了)

### REST API (完了)
- [x] 基本的なプロセス管理API ([src/internal/api/handlers.go](src/internal/api/handlers.go))
  - [x] GET /api/v1/processes - プロセス一覧
  - [x] GET /api/v1/processes/:name/status - ステータス取得
  - [x] POST /api/v1/processes/:name/start - プロセス起動
  - [x] POST /api/v1/processes/:name/stop - プロセス停止
- [x] 手動更新トリガーAPI
  - [x] POST /api/v1/processes/:name/update - 更新開始
  - [x] GET /api/v1/processes/:name/version - バージョン情報取得
  - [x] POST /api/v1/processes/:name/rollback - ロールバック

### gRPC API (完了)
- [x] protobuf定義 ([src/internal/proto/process_manager.proto](src/internal/proto/process_manager.proto))
- [x] gRPCサーバー実装 ([src/internal/grpc/server.go](src/internal/grpc/server.go))
- [x] プロセス管理RPC
  - [x] ListProcesses
  - [x] GetProcess
  - [x] StartProcess
  - [x] StopProcess
  - [x] RestartProcess
  - [x] ScaleProcess
- [x] 更新管理RPC
  - [x] UpdateProcess
  - [x] UpdateAllProcesses
  - [x] GetProcessVersion
  - [x] ListAvailableUpdates
  - [x] RollbackProcess
- [x] WatchUpdate ストリーミング (更新進捗監視)
- [x] メインサーバーへの統合 ([src/cmd/gowinproc/main.go:157-174](src/cmd/gowinproc/main.go#L157))
- [x] Graceful Shutdown ([src/cmd/gowinproc/main.go:184-186](src/cmd/gowinproc/main.go#L184))

### メトリクス収集 (未実装)
- [ ] CPU使用率監視
- [ ] メモリ使用量監視
- [ ] ディスクI/O監視
- [ ] ネットワークトラフィック監視
- [ ] プロセス稼働時間トラッキング
- [ ] Windowsパフォーマンスカウンター統合
- [ ] gopsutil v3 統合
- [ ] メトリクスストレージ (SQLite/JSON)
- [ ] Prometheusメトリクスエクスポート (オプション)

---

## ⚠️ Phase 3: 更新機能とTunnel統合 (85% 完了)

### Tunnel Manager (完了)
- [x] cloudflaredプロセス起動・管理 ([src/internal/tunnel/manager.go](src/internal/tunnel/manager.go))
- [x] 公開URL取得ロジック
- [x] トンネル状態監視
- [x] 自動再起動機能
- [x] cloudflared検出 (PATH/共通パス)

### Webhook受信 (部分完了)
- [x] POST /webhook/github エンドポイント ([src/internal/webhook/handler.go:47](src/internal/webhook/handler.go#L47))
- [x] POST /webhook/cloudflare エンドポイント ([src/internal/webhook/handler.go:115](src/internal/webhook/handler.go#L115))
- [ ] **TODO**: GitHub Webhook署名検証 ([src/internal/webhook/handler.go:54](src/internal/webhook/handler.go#L54))
- [x] 更新トリガー連携
- [ ] **TODO**: リポジトリ→プロセス名マッピング ([src/internal/webhook/handler.go:94](src/internal/webhook/handler.go#L94))

### github-webhook-workerクライアント (完了)
- [x] ポーリング機能実装 ([src/internal/poller/github_poller.go](src/internal/poller/github_poller.go))
- [x] リポジトリメタデータ取得
- [x] 定期バージョンチェック
- [x] Service Binding対応 (HTTP経由)

### バージョン管理 (完了)
- [x] バージョン比較ロジック ([src/internal/version/manager.go](src/internal/version/manager.go))
- [x] バージョン履歴管理
- [x] 前バージョン取得機能
- [x] バージョン情報の永続化 (JSON)

### GitHub統合 (完了)
- [x] GitHub API クライアント ([src/internal/github/client.go](src/internal/github/client.go))
- [x] リリース情報取得
- [x] バイナリダウンロード
- [x] 進捗コールバック
- [x] Public/Private リポジトリ対応

### Hot Deploy (部分完了)
- [x] ローリングアップデート実装 ([src/internal/update/manager.go](src/internal/update/manager.go))
- [x] Secret再注入
- [x] グレースフルシャットダウン
- [x] 更新進捗トラッキング
- [ ] **TODO**: プロセス設定の動的バイナリパス更新 ([src/internal/update/manager.go:145](src/internal/update/manager.go#L145))
- [x] エラーハンドリング

### ロールバック (完了)
- [x] 前バージョンへのロールバック ([src/internal/update/manager.go:194](src/internal/update/manager.go#L194))
- [x] 特定バージョン指定ロールバック
- [x] バージョン履歴からの復元

---

## ❌ Phase 4: 自動スケーリング (未実装)

### スケーリングロジック
- [ ] リソース監視統合
- [ ] 閾値判定ロジック
- [ ] スケールアウト判定
- [ ] スケールイン判定
- [ ] クールダウン期間管理

### プロセス動的管理
- [ ] プロセスインスタンス動的追加
- [ ] プロセスインスタンス動的削除
- [ ] Secret自動取得 (スケール時)
- [ ] ポート動的割り当て
- [ ] 最小/最大インスタンス数制御

### 負荷分散
- [ ] ロードバランシング戦略
- [ ] ヘルスチェックベースルーティング
- [ ] インスタンス間トラフィック分散

---

## ✅ 追加機能: Webダッシュボード + システムトレイ (完了)

### Webダッシュボード (React + TypeScript + Vite)
- [x] プロジェクト構成 ([frontend/](frontend/))
- [x] プロセス管理UI
  - [x] ProcessList コンポーネント ([frontend/src/components/ProcessList.tsx](frontend/src/components/ProcessList.tsx))
  - [x] ProcessDetail コンポーネント ([frontend/src/components/ProcessDetail.tsx](frontend/src/components/ProcessDetail.tsx))
  - [x] InstanceList コンポーネント ([frontend/src/components/InstanceList.tsx](frontend/src/components/InstanceList.tsx))
- [x] メトリクス可視化
  - [x] MetricsChart コンポーネント ([frontend/src/components/MetricsChart.tsx](frontend/src/components/MetricsChart.tsx))
  - [x] Recharts統合 (CPU/メモリグラフ)
- [x] 更新管理UI
  - [x] UpdateManager コンポーネント ([frontend/src/components/UpdateManager.tsx](frontend/src/components/UpdateManager.tsx))
  - [x] バージョン比較表示
  - [x] 更新・ロールバック操作
- [x] REST API クライアント ([frontend/src/api/client.ts](frontend/src/api/client.ts))
  - [x] 全エンドポイント対応 (プロセス管理・更新・メトリクス)
- [x] レスポンシブデザイン
  - [x] CSSスタイル (App.css, ProcessList.css, etc.)
- [x] リアルタイム更新 (5秒ごとポーリング)
- [x] タブナビゲーション (Processes/Updates)
- [x] ドキュメント ([frontend/README.md](frontend/README.md))

### システムトレイ統合
- [x] システムトレイマネージャー ([src/internal/systray/systray.go](src/internal/systray/systray.go))
- [x] Windows通知領域アイコン表示
- [x] トレイメニュー実装
  - [x] ステータス表示 (Running)
  - [x] REST APIアドレス表示
  - [x] gRPC APIアドレス表示
  - [x] "Open Dashboard" - ブラウザ起動
  - [x] "View Logs" - エクスプローラー起動
  - [x] "Quit" - 安全なシャットダウン
- [x] アイコンデータ (16x16 ICO形式)
- [x] メインプログラム統合 ([src/cmd/gowinproc/main.go](src/cmd/gowinproc/main.go))
- [x] Graceful Shutdown対応

### 依存関係
- [x] frontend: react, react-dom, recharts, vite
- [x] backend: github.com/getlantern/systray

---

## ❌ Phase 5: API完成とテスト (10% 完了)

### 認証・認可
- [ ] API key ベース認証
- [ ] JWT トークン認証 (オプション)
- [ ] ロールベースアクセス制御 (RBAC)
- [ ] APIエンドポイント保護

### テスト
- [ ] ユニットテスト
  - [ ] Secret Manager
  - [ ] Version Manager
  - [ ] Certificate Manager
  - [ ] Process Manager
  - [ ] Update Manager
- [ ] 統合テスト
  - [ ] Cloudflare Workers Mock
  - [ ] GitHub API Mock
  - [ ] エンドツーエンドテスト
  - [ ] Webhook テスト

### ドキュメント
- [x] README.md ([README.md](README.md))
- [x] DESIGN.md ([DESIGN.md](DESIGN.md))
- [ ] API仕様ドキュメント (OpenAPI/Swagger)
- [ ] gRPC API ドキュメント
- [ ] 運用ガイド
- [ ] トラブルシューティングガイド
- [ ] セキュリティベストプラクティス

---

## 🔥 優先度付きタスクリスト

### 🚨 高優先度 (Phase 3 完了のため)

#### 1. Webhook署名検証実装
**ファイル**: [src/internal/webhook/handler.go:54](src/internal/webhook/handler.go#L54)
**説明**: GitHub Webhookの署名検証を実装してセキュリティを強化
```go
// TODO: Implement GitHub webhook signature validation
// - X-Hub-Signature-256 ヘッダー検証
// - HMAC-SHA256 署名生成・比較
// - タイミング攻撃対策
```

#### 2. 動的バイナリパス更新
**ファイル**: [src/internal/update/manager.go:145](src/internal/update/manager.go#L145)
**説明**: 更新時にプロセス設定のバイナリパスを動的に変更
```go
// TODO: Update process config with new binary path
// - ProcessConfig.BinaryPath を新バージョンに変更
// - プロセスマネージャーの設定更新
// - 永続化処理
```

#### 3. リポジトリ→プロセス名マッピング
**ファイル**: [src/internal/webhook/handler.go:94](src/internal/webhook/handler.go#L94)
**説明**: GitHubリポジトリ名から管理対象プロセスを特定
```go
// TODO: Need to match repository to process name
// - 設定からリポジトリマップ構築
// - Webhook受信時にプロセス特定
// - 複数プロセス対応
```

### 📊 中優先度 (Phase 2 メトリクス機能)

#### 4. メトリクス収集実装
**新規パッケージ**: `src/internal/monitor/`
**説明**: gopsutil統合でシステムメトリクス収集
- CPU使用率
- メモリ使用量
- ディスクI/O
- ネットワークトラフィック
- プロセス稼働時間

**依存関係**:
```bash
go get github.com/shirou/gopsutil/v3/cpu
go get github.com/shirou/gopsutil/v3/mem
go get github.com/shirou/gopsutil/v3/disk
go get github.com/shirou/gopsutil/v3/net
go get github.com/shirou/gopsutil/v3/process
```

#### 5. メトリクスストレージ
**新規パッケージ**: `src/internal/storage/`
**説明**: メトリクスの永続化と履歴管理
- SQLite または JSON ファイル
- 時系列データ保存
- クエリAPI

#### 6. Prometheusエクスポート (オプション)
**新規パッケージ**: `src/internal/metrics/`
**説明**: Prometheus形式メトリクス公開
```bash
go get github.com/prometheus/client_golang/prometheus
```

### 🔄 低優先度 (Phase 4-5)

#### 7. 自動スケーリング実装
**新規パッケージ**: `src/internal/scaler/`
**説明**: メトリクスベースの自動スケーリング
- 閾値判定
- スケールアウト/イン実行
- クールダウン管理

#### 8. 認証・認可機能
**新規パッケージ**: `src/internal/auth/`
**説明**: API セキュリティ強化
- API key 認証
- JWT トークン (オプション)
- RBAC

#### 9. テストスイート
**新規ディレクトリ**: `tests/`
**説明**: 包括的なテストカバレッジ
- ユニットテスト (各パッケージ)
- 統合テスト
- E2Eテスト

---

## 📝 既知の問題・制限事項

### 実装済みの制限
1. **プロセスバイナリパス固定**: 更新後も設定ファイルのパスを使用 (動的変更未実装)
2. **Webhook署名検証なし**: セキュリティリスク (実装必須)
3. **リポジトリマッピング手動**: Webhook受信時のプロセス特定が不完全
4. **メトリクス未収集**: リソース監視機能なし
5. **自動スケーリング未実装**: 手動でのインスタンス管理のみ

### 設計上の考慮事項
1. **Windows専用**: Linux/macOS対応は将来課題
2. **gRPCサービス専用**: 他のタイプのプロセスは未サポート
3. **単一サーバー**: 分散環境でのクラスタリング未対応
4. **自己署名証明書**: 本番環境では信頼されたCA証明書推奨

---

## 🎯 次のマイルストーン

### Milestone 1: Phase 3 完了 (短期)
**期間**: 1-2週間
**目標**: Hot Deploy機能の完全動作
- [ ] Webhook署名検証
- [ ] 動的バイナリパス更新
- [ ] リポジトリマッピング
- [ ] 統合テスト (手動)

### Milestone 2: Phase 2 完了 (中期)
**期間**: 2-3週間
**目標**: 監視・API機能の完全実装
- [ ] メトリクス収集
- [ ] メトリクスストレージ
- [ ] Prometheusエクスポート (オプション)
- [ ] APIドキュメント

### Milestone 3: Phase 4-5 完了 (長期)
**期間**: 4-6週間
**目標**: 自動化・テスト・セキュリティ
- [ ] 自動スケーリング
- [ ] 認証・認可
- [ ] テストスイート
- [ ] 完全なドキュメント

---

## 📦 依存関係

### 現在の主要依存
```go
require (
    github.com/google/uuid v1.6.0
    github.com/joho/godotenv v1.5.1
    github.com/yhonda-ohishi-pub-dev/go_auth v0.1.0
    google.golang.org/grpc v1.60.0
    google.golang.org/protobuf v1.31.0
    gopkg.in/yaml.v3 v3.0.1
)
```

### 今後追加予定
```go
// メトリクス収集
github.com/shirou/gopsutil/v3 v3.23.12

// Prometheusエクスポート (オプション)
github.com/prometheus/client_golang v1.18.0

// ロギング
go.uber.org/zap v1.26.0

// CLI
github.com/spf13/cobra v1.8.0

// テスト
github.com/stretchr/testify v1.8.4
```

---

## 🔗 関連リポジトリ

1. **cloudflare-auth-worker**
   URL: https://github.com/yhonda-ohishi-pub-dev/cloudflare-auth-worker
   役割: Secret管理サーバー (RSA認証)

2. **github-webhook-worker**
   URL: https://github.com/yhonda-ohishi-pub-dev/github-webhook-worker
   役割: GitHubバージョン管理サーバー

3. **go_auth**
   URL: https://github.com/yhonda-ohishi-pub-dev/go_auth
   役割: 認証クライアントライブラリ

---

## 📊 コード統計 (参考)

### ファイル数
- **Go ソースファイル**: 20個 (gRPC server + systray追加)
- **protobuf定義**: 1個
- **設定ファイル**: 3個 (YAML)
- **ドキュメント**: 4個 (README, DESIGN, tasks, frontend/README)
- **フロントエンド**: React + TypeScript プロジェクト (frontend/)

### 主要パッケージ
| パッケージ | ファイル | 行数概算 | 状態 |
|-----------|---------|---------|------|
| cmd/gowinproc | 1 | ~202 | ✅ |
| internal/process | 1 | ~300 | ✅ |
| internal/certs | 1 | ~150 | ✅ |
| internal/secrets | 1 | ~160 | ✅ |
| internal/version | 1 | ~200 | ✅ |
| internal/update | 1 | ~260 | ⚠️ |
| internal/api | 1 | ~300 | ✅ |
| internal/grpc | 1 | ~200 | ✅ |
| internal/webhook | 1 | ~185 | ⚠️ |
| internal/tunnel | 1 | ~190 | ✅ |
| internal/poller | 1 | ~150 | ✅ |
| internal/cloudflare | 2 | ~250 | ✅ |
| internal/github | 1 | ~200 | ✅ |
| internal/config | 1 | ~100 | ✅ |
| internal/proto | 1 | ~230 | ✅ |
| internal/systray | 1 | ~125 | ✅ |
| pkg/models | 3 | ~200 | ✅ |
| **frontend/** | 11 | ~600 | ✅ |

**合計**: 約3,400行 (Go: 2,800行 + Frontend: 600行)

---

**最終更新**: 2025-11-01
**ステータス**: Phase 1-2完了、Phase 3部分完了、UI実装完了、次は高優先度タスク着手推奨
