# gowinproc - Windows用gRPCプロセスマネージャー

## プロジェクト概要

WindowsでgRPC実行ファイルを起動・監視し、自動更新とスケーリング機能を提供するプロセスマネージャー

## 動作モード

### スタンドアロンモード（デフォルト）

**最小限の依存関係で動作する基本モード**

- ローカル設定ファイルから環境変数・Secretを読み込み
- `.env`ファイルは手動作成または既存のSecret管理ツールで管理
- 自動更新機能は無効（手動でバイナリ更新）
- 単一サーバー運用に最適
- **外部サービスへの依存なし**

**利点**:
- セットアップが簡単
- 外部API・サービスへの依存なし
- オフライン環境でも動作可能
- 既存のSecret管理フローに統合しやすい
- GitHub Token不要

### Cloudflare統合モード（オプション）

**外部Secret管理とWebhook連携を使用する拡張モード**

- cloudflare-auth-workerからSecretを動的取得（RSA認証）
- github-webhook-workerでバージョン情報を一元管理
- `.env`ファイルは自動生成される
- GitHub Tokenは不要（Webhook経由）
- 複数サーバー間でSecret・バージョン情報を共有可能

**利点**:
- 集中管理されたSecret
- GitHub Webhookによるリアルタイム更新通知
- 複数サーバー環境でのSecret同期
- Secret更新時の自動反映

## 主要機能

### 1. プロセス管理
- gRPC実行ファイルの起動・停止・再起動
- プロセスのヘルスチェックと自動復旧
- 複数プロセスインスタンスの管理
- プロセスのグレースフルシャットダウン

### 2. GitHub統合とHot Deploy（オプション - Cloudflare統合モードのみ）
- github-webhook-workerからバージョン情報を取得
- 新しいリリースの自動検出
- **GitHub Releaseからのバイナリダウンロード**:
  - Public repositoryの場合: トークン不要
  - Private repositoryの場合: GitHub Personal Access Token必要
  - アセットファイルの自動ダウンロード
- Hot Deploy: 無停止でのバイナリ更新
  - 新バージョンのダウンロード
  - ローリングアップデート（順次再起動）
  - ロールバック機能

### 3. リソース監視
- CPU使用率
- メモリ使用量
- ディスクI/O
- ネットワークトラフィック
- プロセス稼働時間

### 4. 自動スケーリング
- リソース使用率に基づく自動スケールアウト/イン
- 設定可能な閾値
- プロセス数の上限・下限設定

### 5. APIエンドポイント
- RESTful API / gRPC API
- メトリクス取得
- プロセス制御（start/stop/restart）
- 設定変更

## アーキテクチャ

### システム全体構成

このプロセスマネージャーは、以下の外部サービスと連携します：

```
┌──────────────────────────────────────────────────────────────────────┐
│                    External Cloudflare Services                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌────────────────────┐  ┌─────────────────────┐  ┌──────────────┐ │
│  │ cloudflare-auth-   │  │ github-webhook-     │  │   GitHub     │ │
│  │     worker         │  │     worker          │◄─┤   Webhook    │ │
│  │  (Secret管理)      │  │  (バージョン管理)    │  │              │ │
│  └──────┬─────────────┘  └──────────┬──────────┘  └──────────────┘ │
│         │                           │                                │
│         │ RSA公開鍵認証              │ Service Binding                │
│         │ Secret取得                │ リポジトリメタデータ取得        │
└─────────┼───────────────────────────┼────────────────────────────────┘
          │                           │
          ▼                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                  gowinproc (Windows Process Manager)                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                   go_auth ライブラリ                          │  │
│  │  - RSA認証クライアント (チャレンジ-レスポンス)                │  │
│  │  - cloudflare-auth-workerからSecret取得                       │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                           │                                          │
│  ┌────────────────────────┼──────────────────────────────────────┐ │
│  │         Management API Layer                                  │ │
│  │  (REST/gRPC endpoints for control/metrics)                    │ │
│  └────────────────────────┬──────────────────────────────────────┘ │
│                           │                                          │
│  ┌────────────────────────┴──────────────────────────────────────┐ │
│  │         Process Manager Core                                  │ │
│  │  - Process Lifecycle Management                               │ │
│  │  - Configuration Management                                   │ │
│  │  - Event Bus                                                  │ │
│  │  - Secret Injection (go_auth経由)                            │ │
│  └───┬─────────┬──────────┬──────────────────┬───────────────────┘ │
│      │         │          │                  │                      │
│  ┌───┴───┐ ┌──┴──────┐ ┌─┴────────────┐ ┌──┴─────────────────┐   │
│  │Monitor│ │ Updater │ │ Auto Scaler  │ │  Secret Manager    │   │
│  │Service│ │ Service │ │   Service    │ │  (go_auth統合)     │   │
│  └───────┘ └────┬────┘ └──────────────┘ └────────────────────┘   │
│                 │                                                   │
│                 │ github-webhook-worker                             │
│                 │ からバージョン情報取得                             │
│                 ▼                                                   │
│        ┌────────────────┐                                          │
│        │  Process Pool  │                                          │
│        │ (gRPC Services)│                                          │
│        │  + Secrets     │                                          │
│        └────────────────┘                                          │
└─────────────────────────────────────────────────────────────────────┘
```

### 統合コンポーネントの役割

#### 1. cloudflare-auth-worker
- **リポジトリ**: https://github.com/yhonda-ohishi-pub-dev/cloudflare-auth-worker
- **役割**: 各プロセスに必要なSecret（機密情報）を管理
- **認証方式**: RSA公開鍵認証（RSASSA-PKCS1-v1_5 + SHA-256）
- **プロトコル**: チャレンジ-レスポンス方式
- **技術**: Cloudflare Workers + Durable Objects

**APIエンドポイント**:
- `POST /challenge` - チャレンジ取得
- `POST /verify` - 署名検証とSecret取得
- `GET /health` - ヘルスチェック

#### 2. github-webhook-worker
- **リポジトリ**: https://github.com/yhonda-ohishi-pub-dev/github-webhook-worker
- **役割**: GitHubリポジトリのバージョン情報を管理
- **機能**:
  - GitHub Webhookの受信・検証
  - リポジトリメタデータ（version, grpcEndpoint）の保存・管理
  - Webhook履歴の記録
- **技術**: Cloudflare Workers + Durable Objects + KV Storage

**公開エンドポイント**:
- `POST /webhook` - GitHub Webhookの受信
- `GET /history` - Webhook履歴取得
- `GET /health` - ヘルスチェック

**内部エンドポイント（Service Binding限定）**:
- `POST /repo` - リポジトリメタデータ登録
- `GET /repo/{owner/repo}` - メタデータ取得
- `PATCH /repo/{owner/repo}` - メタデータ更新
- `GET /repos` - 全リポジトリ一覧
- `DELETE /repo/{owner/repo}` - メタデータ削除

#### 3. go_auth
- **リポジトリ**: https://github.com/yhonda-ohishi-pub-dev/go_auth
- **役割**: cloudflare-auth-workerからSecret取得するGoクライアントライブラリ
- **機能**:
  - RSA秘密鍵による署名生成
  - チャレンジ-レスポンス認証の実行
  - 自動リトライ機能
  - 鍵ペア生成ユーティリティ

**主要API**:
```go
client, _ := authclient.NewClientFromFile(
    "https://auth-worker.workers.dev",
    "client-id",
    "private.pem"
)
resp, _ := client.Authenticate()
// resp.SecretData にSecret変数が格納
```

### コンポーネント構成（gowinproc内部）

```
┌─────────────────────────────────────────────┐
│           Management API Layer              │
│  (REST/gRPC endpoints for control/metrics)  │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────┴───────────────────────────┐
│         Process Manager Core                │
│  - Process Lifecycle Management             │
│  - Configuration Management                 │
│  - Event Bus                                │
└─────┬─────────┬──────────┬──────────────────┘
      │         │          │
      │         │          │
┌─────┴───┐ ┌──┴──────┐ ┌─┴────────────┐
│ Monitor │ │ Updater │ │ Auto Scaler  │
│ Service │ │ Service │ │   Service    │
└─────────┘ └─────────┘ └──────────────┘
      │         │              │
      └─────────┴──────────────┘
                 │
        ┌────────┴────────┐
        │  Process Pool   │
        │ (gRPC Services) │
        └─────────────────┘
```

### 主要モジュール

#### 1. Process Manager (`pkg/manager`)
- プロセスのライフサイクル管理
- プロセスプールの維持
- Windows Process API (`os/exec`) の活用

#### 2. Monitor Service (`pkg/monitor`)
- Windowsパフォーマンスカウンターの利用
- `github.com/shirou/gopsutil` でクロスプラットフォーム対応
- メトリクスの収集と保存
- Prometheus形式のメトリクスエクスポート（オプション）

#### 3. Updater Service (`pkg/updater`)
- GitHub API統合
- リリース検出ロジック
- バイナリダウンロードと検証
- ローリングアップデート実装
- ロールバック機能

#### 4. Auto Scaler (`pkg/scaler`)
- メトリクスベースのスケーリング判定
- スケールアウト/イン実行
- クールダウン期間の管理

#### 5. API Server (`pkg/api`)
- REST API (Gin/Echo framework)
- gRPC API
- 認証・認可
- メトリクスエンドポイント

#### 6. Configuration (`pkg/config`)
- YAML/JSON設定ファイル
- 環境変数サポート
- 実行時設定変更

#### 7. Secret Manager (`pkg/secret`)

**スタンドアロンモード**:
- ローカル.envファイルから環境変数読み込み
- godotenvライブラリで.envファイルをパース
- 手動作成された.envファイルをそのまま使用

**Cloudflare統合モード**:
- go_authライブラリの統合
- cloudflare-auth-workerからの認証・Secret取得
- Secret情報のキャッシュ管理
- **.env ファイル自動生成**
  - 取得したSecretを.envファイルに出力
  - プロセスごとに専用の.envファイル作成
  - 環境変数形式で保存（KEY=VALUE）
- プロセス起動時のSecret注入

#### 8. Version Manager (`pkg/version`)

**スタンドアロンモード**:
- バージョン管理機能は無効
- 手動でのバイナリ更新
- プロセスの再起動機能のみ提供

**Cloudflare統合モード（オプション）**:
- github-webhook-workerとの連携
- リポジトリメタデータ情報の取得
- Webhook経由のリアルタイム更新
- バージョン比較とアップデート判定
- 自動ダウンロードとHot Deploy
- gRPCエンドポイント情報の管理

#### 9. Certificate Manager (`pkg/cert`)
- **SSL/TLS証明書の自動生成**
  - 起動時に自己署名証明書を自動作成
  - RSA/ECDSA鍵ペア生成
  - 証明書・秘密鍵のPEM形式保存
- 証明書の有効期限管理
- 証明書の自動更新（オプション）
- プロセスへの証明書パス提供

#### 10. Tunnel Manager (`pkg/tunnel`)（オプション）
- **Cloudflare Tunnelの統合**
  - cloudflaredプロセスの起動・管理
  - 公開URL（https://xxx.trycloudflare.com）の自動取得
  - プロセスごとに個別のトンネル作成
  - トンネルURLの環境変数注入
- **用途**:
  - github-webhook-workerからのWebhook Push受信
  - 外部からのAPI管理アクセス
  - ローカル開発環境での公開テスト
- **実装**:
  - woff_svのCloudflaredTunnel実装を活用
  - `cloudflared tunnel --url http://localhost:PORT --protocol http2`
  - 標準出力から公開URLを抽出

## 統合シナリオ

### 1A. プロセス起動フロー（スタンドアロンモード）

```
1. gowinproc起動
   ↓
2. 初期化処理
   - 設定ファイル読み込み
   - プロセス定義
   - ディレクトリ構造作成
   ↓
3. Certificate Manager起動
   - 証明書ディレクトリ確認
   - 証明書が存在しない場合は自動生成
     * 自己署名証明書作成
     * RSA 2048/4096ビット鍵生成
     * server.crt, server.key 保存
   - 証明書の有効期限チェック
   ↓
4. Secret Manager初期化
   - ローカル.envファイルの存在確認
   - .envファイルが存在しない場合は警告
     （手動作成を促す）
   ↓
5. 環境変数読み込み
   - .envファイルから環境変数読み込み（godotenv）
   - 証明書パスを環境変数に追加
     * CERT_FILE=/path/to/server.crt
     * KEY_FILE=/path/to/server.key
   - プロセス情報を環境変数に追加
     * PROCESS_NAME=service-name
     * PROCESS_PORT=50051
   ↓
6. プロセス起動
   - 環境変数を設定してgRPC実行ファイルを起動
   - プロセスIDを記録
   ↓
7. 監視開始
   - ヘルスチェック
   - メトリクス収集
```

### 1B. プロセス起動フロー（Cloudflare統合モード）

```
1. gowinproc起動
   ↓
2. 初期化処理
   - 設定ファイル読み込み
   - Cloudflare統合設定確認
   - RSA秘密鍵パス確認
   ↓
3. Certificate Manager起動
   - 証明書自動生成（同上）
   ↓
4. Secret Manager初期化
   - go_authクライアント作成
   - cloudflare-auth-workerに接続
   ↓
5. Secret取得と.env自動生成
   - チャレンジリクエスト
   - RSA秘密鍵で署名
   - 認証実行
   - Secret変数を取得
   - .envファイルに自動出力
     * ./processes/{process-name}/.env
     * KEY=VALUE形式
     * ファイルパーミッション: 0600
   ↓
6. プロセス起動
   - .envファイルから環境変数読み込み
   - 証明書パス・プロセス情報を追加
   - gRPC実行ファイルを起動
   ↓
7. 監視・自動更新開始
   - ヘルスチェック
   - メトリクス収集
   - github-webhook-workerポーリング
```

### 2. 自動更新フロー（GitHub Webhook連携）

```
1. GitHub Release作成
   ↓
2. GitHub Webhook送信
   ↓
3. github-webhook-worker受信
   - 署名検証
   - Webhook処理
   - KVにバージョン更新
   ↓
4. gowinproc定期ポーリング
   - github-webhook-workerに問い合わせ
   - GET /repos でリポジトリ一覧取得
   ↓
5. バージョン比較
   - 新バージョン検出
   - 更新判定
   ↓
6. Hot Deploy実行
   - 新バイナリダウンロード
   - チェックサム検証
   - ローリングアップデート
   - 旧プロセス停止 → 新プロセス起動
   ↓
7. Secret再注入
   - 新プロセスにSecret取得
   - 環境変数設定
   - 起動確認
```

### 3. スケーリング時のSecret管理

```
1. Monitor Serviceがリソース監視
   ↓
2. 閾値超過検出
   ↓
3. Auto Scalerが新インスタンス起動判定
   ↓
4. Secret Manager経由でSecret取得
   - 既存のSecretキャッシュがあれば再利用
   - 期限切れの場合は再認証
   ↓
5. 新プロセスインスタンス起動
   - Secretを環境変数として注入
   - ポート番号を動的に割り当て
   ↓
6. プロセスプールに追加
   - ロードバランシング設定更新
```

## データフロー

### Secret取得フロー

```
gowinproc
   │
   │ 1. NewClientFromFile()
   ├──────────────────────────────────────┐
   │                                       │
   │ go_auth ライブラリ                    │
   │   │                                   │
   │   │ 2. POST /challenge               │
   │   ├─────────────────────────────►    │
   │   │                                   │
   │   │ ◄─────────────────────────────┤  │
   │   │ 3. Challenge Response            │
   │   │    (random bytes)                │
   │   │                            cloudflare-auth-worker
   │   │ 4. RSA署名生成                    │
   │   │    (秘密鍵で署名)                 │
   │   │                                   │
   │   │ 5. POST /verify                  │
   │   │    (署名送信)                     │
   │   ├─────────────────────────────►    │
   │   │                                   │
   │   │                             6. 署名検証
   │   │                                (公開鍵)
   │   │                                   │
   │   │ ◄─────────────────────────────┤  │
   │   │ 7. Verify Response               │
   │   │    {                             │
   │   │      token: "jwt",               │
   │   │      secretData: {               │
   │   │        "DB_PASSWORD": "***",     │
   │   │        "API_KEY": "***"          │
   │   │      }                            │
   │   │    }                              │
   │   │                                   │
   ├──┤                                    │
   │  │ 8. Secret取得完了                  │
   │◄─┘                                    │
   │                                       │
   │ 9. プロセス起動（Secretを環境変数に注入） │
   │                                       │
```

### バージョン情報取得フロー

```
GitHub
   │
   │ 1. Release作成/Push
   └─────────────────────────────────►
                                      │
                             github-webhook-worker
                                      │
                                      │ 2. Webhook処理
                                      │    - 署名検証
                                      │    - KV更新
                                      │      (version情報)
                                      │
   ┌──────────────────────────────────┤
   │                                  │
   │ 3. ポーリング                     │
   │    GET /repos                    │
   │    or                            │
   │    GET /repo/owner/repo          │
   │                                  │
gowinproc                             │
   │                                  │
   │ ◄────────────────────────────────┤
   │ 4. バージョン情報取得              │
   │    {                             │
   │      repo: "owner/repo",         │
   │      version: "v1.2.0",          │
   │      grpcEndpoint: "...",        │
   │      updatedAt: 1234567890       │
   │    }                             │
   │                                  │
   │ 5. 現在のバージョンと比較          │
   │                                  │
   │ 6. 新バージョン検出               │
   │    → Hot Deploy開始              │
```

## データ構造

### Process Configuration

```go
type ProcessConfig struct {
    Name            string
    ExecutablePath  string
    Args            []string
    WorkingDir      string
    Env             map[string]string
    Port            int
    HealthCheckURL  string

    // GitHub Integration (Cloudflare統合モードのみ)
    GitHubRepo      string // owner/repo
    GitHubToken     string // Private repoの場合のみ必要
    AutoUpdate      bool   // Cloudflare統合モードのみ有効
    UpdateInterval  time.Duration

    // Secret Management
    SecretsEnabled  bool
    SecretSource    string // "local" または "cloudflare"
    EnvFilePath     string // .envファイルのパス

    // Certificate
    CertEnabled     bool
    CertPath        string // 証明書ファイルパス
    KeyPath         string // 秘密鍵ファイルパス

    // Scaling
    MinInstances    int
    MaxInstances    int
    CPUThreshold    float64
    MemoryThreshold uint64
}
```

### Process Instance

```go
type ProcessInstance struct {
    ID              string
    Config          *ProcessConfig
    PID             int
    Status          ProcessStatus
    StartTime       time.Time
    Port            int

    // Environment
    EnvFilePath     string // 実際に使用している.envファイル
    CertPath        string // 実際に使用している証明書
    KeyPath         string // 実際に使用している秘密鍵

    // Metrics
    CPUUsage        float64
    MemoryUsage     uint64
    RequestCount    int64
}
```

### Secret Data Structure

```go
// Secret Manager用
type SecretData struct {
    ProcessName     string
    Secrets         map[string]string // key-value pairs
    EnvFilePath     string
    UpdatedAt       time.Time
    ExpiresAt       time.Time // JWT有効期限
}
```

### Certificate Data Structure

```go
// Certificate Manager用
type Certificate struct {
    ProcessName     string
    CertPath        string
    KeyPath         string
    NotBefore       time.Time
    NotAfter        time.Time
    Subject         string
    Issuer          string
}
```

## 技術スタック

### 必須ライブラリ

```
# Cloudflare統合
github.com/yhonda-ohishi-pub-dev/go_auth  # cloudflare-auth-worker認証クライアント

# システム監視
github.com/shirou/gopsutil/v3    # システムメトリクス取得

# GitHub統合
github.com/google/go-github/v50  # GitHub API（バイナリダウンロード用）

# gRPC/HTTP
google.golang.org/grpc           # gRPC framework
github.com/gin-gonic/gin         # REST API framework

# 設定・ファイル
gopkg.in/yaml.v3                 # 設定ファイル
github.com/fsnotify/fsnotify     # ファイル監視
github.com/joho/godotenv         # .envファイル読み込み

# 標準ライブラリ（証明書生成）
crypto/x509                      # X.509証明書生成
crypto/rsa                       # RSA鍵生成
crypto/tls                       # TLS機能
encoding/pem                     # PEMエンコード
```

### オプションライブラリ

```
github.com/prometheus/client_golang  # Prometheusメトリクス
go.uber.org/zap                      # 構造化ログ
github.com/spf13/cobra               # CLI
github.com/spf13/viper               # 設定管理
```

### 外部サービス依存

```
cloudflare-auth-worker          # Secret管理（RSA認証）
github-webhook-worker           # バージョン管理（GitHub Webhook）
GitHub API                      # リリースバイナリダウンロード
```

## 実装計画

### Phase 1: 基本プロセス管理とCloudflare統合（Week 1-2）
- [ ] プロジェクト構造のセットアップ
  - [ ] ディレクトリ構造作成（processes/, certs/, keys/, logs/, data/）
- [ ] Certificate Manager実装
  - [ ] 自己署名証明書生成機能
  - [ ] crypto/x509, crypto/rsaを使用
  - [ ] 証明書ファイル保存（PEM形式）
  - [ ] 有効期限チェック機能
- [ ] go_authライブラリの統合（オプション）
  - [ ] cloudflare-auth-workerとの接続
  - [ ] RSA鍵ペアの生成・管理
  - [ ] Secret取得テスト
- [ ] Secret Manager実装
  - [ ] .envファイル読み込み機能（godotenv）
  - [ ] Cloudflare統合時: .envファイル自動生成機能
  - [ ] KEY=VALUE形式での出力
  - [ ] ファイルパーミッション設定（0600）
- [ ] 基本的なプロセス起動・停止機能
  - [ ] .envファイルから環境変数読み込み
  - [ ] 証明書パスの環境変数注入
  - [ ] プロセス情報の環境変数注入
- [ ] 設定ファイルの読み込み
  - [ ] スタンドアロンモード設定
  - [ ] Cloudflare統合設定（オプション）
  - [ ] 証明書設定
- [ ] 簡単なヘルスチェック

### Phase 2: 監視機能とREST/gRPC API（Week 3）
- [ ] メトリクス収集の実装
- [ ] Windowsパフォーマンスカウンター統合
- [ ] メトリクスストレージ
- [ ] **REST API実装**
  - [ ] 基本的なプロセス管理API
  - [ ] **手動更新トリガーAPI** ★推奨実装
    * POST /api/v1/update
    * POST /api/v1/processes/:name/update
    * GET /api/v1/processes/:name/version
    * POST /api/v1/processes/:name/rollback
- [ ] **gRPC API実装**
  - [ ] プロセス管理RPC
  - [ ] **更新管理RPC** ★推奨実装
  - [ ] **WatchUpdate ストリーミング** (進捗監視)

### Phase 3: 更新機能とTunnel統合（Week 4-5）
- [ ] **Tunnel Manager実装** ★推奨実装
  - [ ] woff_svのCloudflaredTunnel移植
  - [ ] cloudflaredプロセス起動・管理
  - [ ] 公開URL取得ロジック
  - [ ] トンネル状態監視
- [ ] **Webhook受信エンドポイント** ★推奨実装
  - [ ] POST /api/v1/webhook
  - [ ] 署名検証（共有シークレット）
  - [ ] 更新トリガー連携
- [ ] github-webhook-workerクライアント実装（オプション）
  - [ ] Service Binding対応（HTTP経由）
  - [ ] リポジトリメタデータ取得
  - [ ] ポーリング機能（フォールバック）
- [ ] バージョン比較ロジック
- [ ] GitHub Releaseからバイナリダウンロード機能
- [ ] ローリングアップデート実装
  - [ ] Secret再注入
  - [ ] グレースフルシャットダウン
- [ ] ロールバック機能

### Phase 4: 自動スケーリング（Week 6）
- [ ] スケーリングロジックの実装
- [ ] メトリクスベースの判定
- [ ] プロセスの動的追加・削除
  - [ ] Secret自動取得
  - [ ] ポート動的割り当て
- [ ] 負荷分散の考慮

### Phase 5: API完成とテスト（Week 7-8）
- [ ] 完全なREST/gRPC API実装
- [ ] 認証・認可機能
- [ ] ユニットテスト
  - [ ] Secret Manager
  - [ ] Version Manager
- [ ] 統合テスト
  - [ ] Cloudflare Workers Mock
  - [ ] エンドツーエンド
- [ ] ドキュメント整備

## 設定ファイル例

```yaml
# config.yaml
manager:
  api_port: 8080
  grpc_port: 9090
  log_level: info

  # 証明書設定
  certificates:
    auto_generate: true  # 起動時に自動生成
    cert_dir: ./certs    # 証明書保存ディレクトリ
    key_size: 2048       # RSA鍵サイズ (2048/4096)
    validity_days: 365   # 証明書有効期間（日）
    organization: "gowinproc"
    common_name: "localhost"

# Cloudflare統合設定
cloudflare:
  # cloudflare-auth-worker (Secret管理)
  auth_worker:
    url: https://auth-worker.example.workers.dev
    client_id: gowinproc-client-001
    private_key_path: ./keys/private.pem
    retry_max: 3
    retry_backoff: 2s

  # github-webhook-worker (バージョン管理)
  webhook_worker:
    # Service Binding経由でアクセス（内部通信）
    url: https://github-webhook-worker.example.workers.dev
    poll_interval: 5m  # バージョンチェック間隔
    service_binding: true

  # Cloudflare Tunnel (オプション - Webhook Push受信用)
  tunnel:
    enabled: false  # デフォルトは無効（ポーリング方式）
    port: 8080  # gowinproc管理APIのポート
    protocol: http2  # http2 または quic
    # 有効にすると公開URLが自動取得される
    # 例: https://xxx.trycloudflare.com
    # この URLをgithub-webhook-workerに登録すると
    # Webhook Push方式で即座に更新通知を受け取れる

processes:
  - name: grpc-service-1
    executable: ./bin/service.exe
    working_dir: ./services/service1
    args:
      - --port=50051
    port: 50051
    health_check_url: http://localhost:50051/health

    # GitHub Repository情報
    github:
      repo: owner/repo-name
      auto_update: true
      update_check_interval: 5m
      # github-webhook-workerから取得するため
      # GitHub Tokenは不要

    # Secret管理
    secrets:
      enabled: true
      # .envファイル出力先
      env_file: ./processes/grpc-service-1/.env
      # Secretの内容はcloudflare-auth-workerで管理
      # 例: DB_PASSWORD, API_KEY など

    # 証明書設定
    certificates:
      enabled: true
      # 証明書パスは自動設定される
      # 環境変数として以下が注入される:
      # - CERT_FILE=./certs/grpc-service-1.crt
      # - KEY_FILE=./certs/grpc-service-1.key

    # Scaling
    scaling:
      min_instances: 2
      max_instances: 10
      cpu_threshold: 80.0      # percent
      memory_threshold: 1073741824  # 1GB in bytes
      cooldown: 60s

  - name: grpc-service-2
    executable: ./bin/another-service.exe
    working_dir: ./services/service2
    args:
      - --port=50052
    port: 50052
    health_check_url: http://localhost:50052/health

    github:
      repo: owner/another-repo
      auto_update: true
      update_check_interval: 10m

    secrets:
      enabled: true
      env_file: ./processes/grpc-service-2/.env

    certificates:
      enabled: true

    scaling:
      min_instances: 1
      max_instances: 5
      cpu_threshold: 75.0
      memory_threshold: 536870912  # 512MB
      cooldown: 30s
```

### 環境変数（gowinproc用）

```bash
# RSA秘密鍵のパス（オプション）
GOWINPROC_PRIVATE_KEY=./keys/private.pem

# cloudflare-auth-worker URL（オプション）
GOWINPROC_AUTH_WORKER_URL=https://auth-worker.workers.dev

# github-webhook-worker URL（オプション）
GOWINPROC_WEBHOOK_WORKER_URL=https://github-webhook-worker.workers.dev

# Client ID（オプション）
GOWINPROC_CLIENT_ID=gowinproc-client-001

# ログレベル
GOWINPROC_LOG_LEVEL=info
```

### 自動生成される.envファイル例

```bash
# ./processes/grpc-service-1/.env
# このファイルはgowinprocが自動生成します
# cloudflare-auth-workerから取得したSecretを含みます

# データベース接続情報
DB_HOST=database.example.com
DB_PORT=5432
DB_NAME=myapp
DB_USER=appuser
DB_PASSWORD=secret_password_from_cloudflare

# API認証情報
API_KEY=api_key_from_cloudflare
API_SECRET=api_secret_from_cloudflare

# 外部サービス
REDIS_URL=redis://redis.example.com:6379
STRIPE_API_KEY=sk_test_from_cloudflare

# 証明書パス（gowinprocが自動で追加）
CERT_FILE=C:\go\gowinproc\certs\grpc-service-1.crt
KEY_FILE=C:\go\gowinproc\certs\grpc-service-1.key

# プロセス情報（gowinprocが自動で追加）
PROCESS_NAME=grpc-service-1
PROCESS_PORT=50051
```

### ディレクトリ構成

```
gowinproc/
├── config.yaml              # メイン設定ファイル
├── gowinproc.exe            # 実行ファイル
│
├── keys/                    # RSA認証鍵（go_auth用）
│   ├── private.pem          # cloudflare-auth-worker認証用秘密鍵
│   └── public.pem           # 公開鍵（Cloudflare側に登録）
│
├── certs/                   # SSL/TLS証明書（自動生成）
│   ├── grpc-service-1.crt   # プロセス1用証明書
│   ├── grpc-service-1.key   # プロセス1用秘密鍵
│   ├── grpc-service-2.crt   # プロセス2用証明書
│   ├── grpc-service-2.key   # プロセス2用秘密鍵
│   └── ca.crt               # CA証明書（オプション）
│
├── processes/               # プロセス専用ディレクトリ
│   ├── grpc-service-1/
│   │   ├── .env            # Secret環境変数（自動生成、0600）
│   │   └── service.exe     # gRPCサービス実行ファイル
│   │
│   └── grpc-service-2/
│       ├── .env            # Secret環境変数（自動生成、0600）
│       └── another.exe     # gRPCサービス実行ファイル
│
├── logs/                    # ログファイル
│   ├── gowinproc.log       # メインログ
│   ├── grpc-service-1.log  # プロセス1ログ
│   └── grpc-service-2.log  # プロセス2ログ
│
└── data/                    # データ保存
    ├── metrics.db           # メトリクスDB
    └── state.json           # 状態保存
```

## APIエンドポイント

### REST API

#### プロセス管理

```
GET    /api/v1/processes              # プロセス一覧取得
GET    /api/v1/processes/:name        # 特定プロセスの詳細
POST   /api/v1/processes/:name/start  # プロセス起動
POST   /api/v1/processes/:name/stop   # プロセス停止
POST   /api/v1/processes/:name/restart # プロセス再起動
GET    /api/v1/processes/:name/metrics # メトリクス取得
POST   /api/v1/processes/:name/scale  # スケール変更
GET    /api/v1/health                 # ヘルスチェック
```

#### 更新管理（手動トリガー）

```
POST   /api/v1/update                      # 全プロセス更新
POST   /api/v1/processes/:name/update      # 特定プロセス更新
GET    /api/v1/processes/:name/version     # 現在のバージョン情報
GET    /api/v1/updates/available           # 利用可能な更新一覧
POST   /api/v1/processes/:name/rollback    # ロールバック
```

#### 更新API詳細

**全プロセス更新**
```http
POST /api/v1/update
Content-Type: application/json

{
  "strategy": "rolling",        // rolling, blue-green, immediate
  "force": false,                // 強制更新フラグ
  "timeout": 300,                // タイムアウト（秒）
  "healthCheckDelay": 30         // ヘルスチェック待機時間（秒）
}

# レスポンス
{
  "success": true,
  "message": "Update initiated for all processes",
  "processes": [
    {
      "name": "grpc-service-1",
      "currentVersion": "v1.0.0",
      "targetVersion": "v1.1.0",
      "status": "updating"
    },
    {
      "name": "grpc-service-2",
      "currentVersion": "v2.0.0",
      "targetVersion": "v2.1.0",
      "status": "updating"
    }
  ],
  "updateId": "upd_abc123"
}
```

**特定プロセス更新**
```http
POST /api/v1/processes/grpc-service-1/update
Content-Type: application/json

{
  "version": "v1.2.0",           // 指定バージョン（省略時は最新）
  "force": false,                // 強制更新
  "strategy": "rolling",         // 更新戦略
  "instances": [1, 2]            // 特定インスタンスのみ（省略時は全て）
}

# レスポンス
{
  "success": true,
  "processName": "grpc-service-1",
  "currentVersion": "v1.0.0",
  "targetVersion": "v1.2.0",
  "updateId": "upd_xyz789",
  "status": "updating",
  "estimatedDuration": 120       // 予想所要時間（秒）
}
```

**バージョン情報取得**
```http
GET /api/v1/processes/grpc-service-1/version

# レスポンス
{
  "processName": "grpc-service-1",
  "currentVersion": "v1.0.0",
  "latestVersion": "v1.2.0",
  "updateAvailable": true,
  "instances": [
    {
      "id": "inst_1",
      "version": "v1.0.0",
      "uptime": 86400
    },
    {
      "id": "inst_2",
      "version": "v1.0.0",
      "uptime": 86400
    }
  ]
}
```

**利用可能な更新一覧**
```http
GET /api/v1/updates/available

# レスポンス
{
  "updates": [
    {
      "processName": "grpc-service-1",
      "currentVersion": "v1.0.0",
      "latestVersion": "v1.2.0",
      "releaseDate": "2025-10-29T10:00:00Z",
      "releaseNotes": "Bug fixes and performance improvements"
    },
    {
      "processName": "grpc-service-2",
      "currentVersion": "v2.0.0",
      "latestVersion": "v2.0.0",
      "upToDate": true
    }
  ]
}
```

**ロールバック**
```http
POST /api/v1/processes/grpc-service-1/rollback
Content-Type: application/json

{
  "version": "v1.0.0"  // ロールバック先（省略時は直前バージョン）
}

# レスポンス
{
  "success": true,
  "processName": "grpc-service-1",
  "fromVersion": "v1.2.0",
  "toVersion": "v1.0.0",
  "rollbackId": "rbk_abc123"
}
```

### gRPC Service

```protobuf
service ProcessManager {
  // プロセス管理
  rpc ListProcesses(ListProcessesRequest) returns (ListProcessesResponse);
  rpc GetProcess(GetProcessRequest) returns (ProcessInfo);
  rpc StartProcess(StartProcessRequest) returns (ProcessInfo);
  rpc StopProcess(StopProcessRequest) returns (Empty);
  rpc RestartProcess(RestartProcessRequest) returns (ProcessInfo);
  rpc GetMetrics(GetMetricsRequest) returns (Metrics);
  rpc ScaleProcess(ScaleProcessRequest) returns (ProcessInfo);

  // 更新管理（手動トリガー）
  rpc UpdateAllProcesses(UpdateAllRequest) returns (UpdateResponse);
  rpc UpdateProcess(UpdateProcessRequest) returns (UpdateResponse);
  rpc GetProcessVersion(GetVersionRequest) returns (VersionInfo);
  rpc ListAvailableUpdates(ListUpdatesRequest) returns (ListUpdatesResponse);
  rpc RollbackProcess(RollbackRequest) returns (RollbackResponse);

  // ストリーミング - 更新進捗監視
  rpc WatchUpdate(WatchUpdateRequest) returns (stream UpdateStatus);
}

// 更新管理メッセージ定義
message UpdateAllRequest {
  string strategy = 1;        // rolling, blue-green, immediate
  bool force = 2;
  int32 timeout = 3;          // seconds
  int32 health_check_delay = 4;
}

message UpdateProcessRequest {
  string process_name = 1;
  string version = 2;         // 空文字列の場合は最新
  bool force = 3;
  string strategy = 4;
  repeated int32 instances = 5; // 特定インスタンスのみ
}

message UpdateResponse {
  bool success = 1;
  string message = 2;
  string update_id = 3;
  repeated ProcessUpdateStatus processes = 4;
}

message ProcessUpdateStatus {
  string name = 1;
  string current_version = 2;
  string target_version = 3;
  string status = 4;          // updating, completed, failed
  int32 estimated_duration = 5;
}

message GetVersionRequest {
  string process_name = 1;
}

message VersionInfo {
  string process_name = 1;
  string current_version = 2;
  string latest_version = 3;
  bool update_available = 4;
  repeated InstanceVersion instances = 5;
}

message InstanceVersion {
  string id = 1;
  string version = 2;
  int64 uptime = 3;           // seconds
}

message ListUpdatesRequest {
  // 空でも可
}

message ListUpdatesResponse {
  repeated UpdateAvailable updates = 1;
}

message UpdateAvailable {
  string process_name = 1;
  string current_version = 2;
  string latest_version = 3;
  string release_date = 4;
  string release_notes = 5;
  bool up_to_date = 6;
}

message RollbackRequest {
  string process_name = 1;
  string version = 2;         // 空文字列の場合は直前バージョン
}

message RollbackResponse {
  bool success = 1;
  string process_name = 2;
  string from_version = 3;
  string to_version = 4;
  string rollback_id = 5;
}

// ストリーミング - リアルタイム更新監視
message WatchUpdateRequest {
  string update_id = 1;
}

message UpdateStatus {
  string update_id = 1;
  string process_name = 2;
  string status = 3;          // downloading, extracting, stopping, starting, completed, failed
  int32 progress = 4;         // 0-100
  string message = 5;
  int64 timestamp = 6;
}
```

### gRPC使用例

```go
// Go クライアント例

// プロセス更新
client := pb.NewProcessManagerClient(conn)
resp, err := client.UpdateProcess(ctx, &pb.UpdateProcessRequest{
    ProcessName: "grpc-service-1",
    Version:     "v1.2.0",
    Strategy:    "rolling",
})

// 更新進捗をストリーミングで監視
stream, err := client.WatchUpdate(ctx, &pb.WatchUpdateRequest{
    UpdateId: resp.UpdateId,
})

for {
    status, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Printf("Progress: %d%% - %s\n", status.Progress, status.Message)
}
```

## セキュリティ考慮事項

1. **認証・認可**
   - API keyベースの認証
   - JWTトークン（オプション）
   - ロールベースアクセス制御

2. **GitHub Token管理**
   - 環境変数での管理
   - 暗号化ストレージ（オプション）
   - 最小権限の原則

3. **プロセス分離**
   - 各プロセスを専用ユーザーで実行（オプション）
   - リソースリミット設定

4. **バイナリ検証**
   - ダウンロードファイルのチェックサム検証
   - 署名検証（オプション）

## モニタリング・ロギング

1. **構造化ログ**
   - JSON形式
   - ログレベル管理
   - ローテーション

2. **メトリクス**
   - Prometheusエクスポーター
   - 内部メトリクスAPI
   - ダッシュボード（Grafana統合可能）

3. **アラート**
   - プロセス異常終了
   - リソース閾値超過
   - アップデート失敗

## 開発環境

- Go 1.21+
- Windows 10/11 or Windows Server 2019+
- Git
- Protocol Buffers Compiler (protoc)

## 関連リポジトリ

### Cloudflare Workers統合

1. **cloudflare-auth-worker**
   - **URL**: https://github.com/yhonda-ohishi-pub-dev/cloudflare-auth-worker
   - **役割**: Secret管理サーバー（RSA公開鍵認証）
   - **技術**: TypeScript, Cloudflare Workers, Durable Objects

2. **github-webhook-worker**
   - **URL**: https://github.com/yhonda-ohishi-pub-dev/github-webhook-worker
   - **役割**: GitHubバージョン管理サーバー（Webhook受信・KV保存）
   - **技術**: TypeScript, Cloudflare Workers, Durable Objects, KV Storage

3. **go_auth**
   - **URL**: https://github.com/yhonda-ohishi-pub-dev/go_auth
   - **役割**: cloudflare-auth-worker認証クライアントライブラリ
   - **技術**: Go, 標準ライブラリのみ

## 参考実装

類似プロジェクト:
- PM2 (Node.js)
- Supervisor (Python)
- systemd (Linux)

## 次のステップ

1. プロジェクト構造の作成
2. go_authライブラリの統合
3. 基本的なCLIの実装
4. プロセスマネージャーコアの実装
5. Cloudflare Workers連携機能の実装
6. 段階的な機能追加

## セキュリティ考慮事項の補足

### Cloudflare統合のセキュリティ

1. **RSA鍵管理**
   - 秘密鍵はローカルファイルシステムに保存（0600パーミッション）
   - 公開鍵はcloudflare-auth-workerに登録
   - 鍵のローテーション機能（将来実装）

2. **通信の暗号化**
   - Cloudflare Workers へは全てHTTPS通信
   - TLS 1.3推奨

3. **Secret管理**
   - Secretは環境変数として注入（プロセス隔離）
   - .envファイルはプロセスごとに分離
   - ファイルパーミッション0600（所有者のみ読み書き可）
   - .envファイルは.gitignoreに追加（バージョン管理しない）
   - メモリ内での一時保存のみ
   - ログに出力しない

4. **認証フロー**
   - チャレンジは使い捨て（リプレイ攻撃対策）
   - 署名検証による認証
   - JWTトークンによるセッション管理（cloudflare-auth-worker側）

5. **証明書管理**
   - 自己署名証明書の自動生成
   - プロセスごとに個別の証明書
   - 証明書パーミッション: 0644（読み取り可）
   - 秘密鍵パーミッション: 0600（所有者のみ）
   - 有効期限の定期チェック
   - 本番環境では信頼されたCA証明書の使用を推奨
