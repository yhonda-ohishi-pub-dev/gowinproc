# gowinproc

Windows用gRPCプロセスマネージャー - Cloudflare Workers統合版

## 概要

gowinprocは、WindowsでgRPC実行ファイルを管理するプロセスマネージャーです。自動更新、スケーリング、そしてCloudflare Workersとの統合によるセキュアなSecret管理機能を提供します。

## 主要機能

- **プロセス管理**: gRPC実行ファイルの起動・停止・再起動・監視
- **自動更新 (Hot Deploy)**: GitHub Releaseからの自動更新・無停止デプロイ
- **自動生成機能**:
  - `.env`ファイル自動生成（環境変数管理）
  - SSL/TLS証明書自動生成（起動時に自己署名証明書作成）
- **リソース監視**: CPU、メモリ、ディスクI/O、ネットワーク
- **自動スケーリング**: リソース使用率に基づくプロセスの動的増減
- **REST/gRPC API**: プロセス制御とメトリクス取得
- **Cloudflare統合（オプション）**:
  - [cloudflare-auth-worker](https://github.com/yhonda-ohishi-pub-dev/cloudflare-auth-worker) によるSecret管理
  - [github-webhook-worker](https://github.com/yhonda-ohishi-pub-dev/github-webhook-worker) によるバージョン管理

## 動作モード

### 1. スタンドアロンモード（デフォルト）
Cloudflare Workers統合なしで動作します。

- ローカル設定ファイルからSecret読み込み
- GitHub API直接アクセスでバージョンチェック
- 最小限の依存関係で高速起動

### 2. Cloudflare統合モード（オプション）
外部Secret管理とWebhook連携が必要な場合に使用します。

- cloudflare-auth-workerからSecretを動的取得
- github-webhook-workerでバージョン情報を一元管理
- 複数サーバー間でのSecret共有が可能

## アーキテクチャ

### スタンドアロンモード
```
    gowinproc (Windows)
         ↓
    gRPC Services (自動管理)
```

### Cloudflare統合モード（オプション）
```
Cloudflare Workers (Secret & Version管理)
         ↓
    gowinproc (Windows)
         ↓
    gRPC Services (自動管理)
```

詳細は [DESIGN.md](DESIGN.md) を参照してください。

## クイックスタート

### 必要要件

- Go 1.21+
- Windows 10/11 or Windows Server 2019+
- Git

### インストール

```bash
git clone https://github.com/yhonda-ohishi-pub-dev/gowinproc
cd gowinproc
go mod tidy
go build -o gowinproc.exe ./src/cmd/gowinproc
```

### 初期設定

#### スタンドアロンモード（最小設定）

```yaml
# config.yaml
manager:
  api_port: 8080
  grpc_port: 9090
  log_level: info

  certificates:
    auto_generate: true
    cert_dir: ./certs
    key_size: 2048
    validity_days: 365

processes:
  - name: my-grpc-service
    executable: ./bin/service.exe
    port: 50051

    # ローカルの.envファイルから環境変数読み込み
    secrets:
      enabled: true
      env_file: ./processes/my-grpc-service/.env
      # local modeではenv_fileを手動作成

    certificates:
      enabled: true

    # GitHub API直接アクセス（オプション）
    github:
      repo: owner/repo-name
      token: ${GITHUB_TOKEN}  # 環境変数から
      auto_update: true
```

`.env`ファイルを手動作成：

```bash
# ./processes/my-grpc-service/.env
DB_PASSWORD=your_password
API_KEY=your_api_key
```

#### Cloudflare統合モード（フル機能）

1. **RSA鍵ペアの生成**

```bash
go run github.com/yhonda-ohishi-pub-dev/go_auth/cmd/example -generate-keys \
  -private-key ./keys/private.pem \
  -public-key ./keys/public.pem
```

2. **公開鍵をCloudflare Workerに登録**

3. **設定ファイルの作成**

```yaml
# config.yaml
manager:
  api_port: 8080
  grpc_port: 9090
  log_level: info

  certificates:
    auto_generate: true

# Cloudflare統合を有効化
cloudflare:
  enabled: true  # これがfalseまたは未設定の場合はスタンドアロン

  auth_worker:
    url: https://your-auth-worker.workers.dev
    client_id: your-client-id
    private_key_path: ./keys/private.pem

  webhook_worker:
    url: https://your-webhook-worker.workers.dev
    poll_interval: 5m

processes:
  - name: my-grpc-service
    executable: ./bin/service.exe
    port: 50051

    github:
      repo: owner/repo-name
      auto_update: true
      # Cloudflare統合時はtokenは不要（webhook経由）

    secrets:
      enabled: true
      source: cloudflare  # "local" または "cloudflare"
      env_file: ./processes/my-grpc-service/.env

    certificates:
      enabled: true
```

### 起動

**基本起動:**
```bash
./gowinproc.exe
```

**オプション指定:**
```bash
./gowinproc.exe \
  -config config.yaml \
  -certs ./certs \
  -keys ./keys \
  -data ./data \
  -binaries ./binaries \
  -github-token YOUR_GITHUB_TOKEN
```

**利用可能なフラグ:**
- `-config` - 設定ファイルパス（デフォルト: config.yaml）
- `-certs` - 証明書ディレクトリ（デフォルト: certs）
- `-keys` - 秘密鍵ディレクトリ（デフォルト: keys）
- `-data` - データファイルディレクトリ（デフォルト: data）
- `-binaries` - バイナリバージョンディレクトリ（デフォルト: binaries）
- `-github-token` - GitHub Personal Access Token（または環境変数GITHUB_TOKEN）

起動時に自動的に以下が実行されます：

1. ディレクトリ構造の作成
2. SSL/TLS証明書の生成（存在しない場合）
3. Cloudflare-auth-workerからSecret取得（Cloudflare統合モード時）
4. `.env`ファイルの生成
5. プロセスの起動
6. Cloudflare Tunnelの起動（有効時）
7. GitHubポーリングの開始（有効時）

## 自動生成される.envファイル

起動時に`./processes/{process-name}/.env`に環境変数ファイルが自動生成されます：

```bash
# cloudflare-auth-workerから取得したSecret
DB_PASSWORD=***
API_KEY=***

# gowinprocが自動追加
CERT_FILE=C:\path\to\certs\my-grpc-service.crt
KEY_FILE=C:\path\to\certs\my-grpc-service.key
PROCESS_NAME=my-grpc-service
PROCESS_PORT=50051
```

gRPCサービスから環境変数として読み込めます：

```go
dbPassword := os.Getenv("DB_PASSWORD")
certFile := os.Getenv("CERT_FILE")
```

## ディレクトリ構成

```
gowinproc/
├── config.yaml           # 設定ファイル
├── gowinproc.exe         # 実行ファイル
├── go.mod                # Go依存関係
│
├── src/                  # ソースコード
│   ├── cmd/
│   │   └── gowinproc/    # メインエントリポイント
│   ├── internal/         # 内部パッケージ
│   │   ├── api/          # REST APIハンドラ
│   │   ├── certs/        # 証明書管理
│   │   ├── config/       # 設定読み込み
│   │   ├── process/      # プロセス管理
│   │   ├── proto/        # gRPC定義（Phase 2）
│   │   └── secrets/      # Secret管理
│   └── pkg/
│       └── models/       # データモデル
│
├── keys/                 # RSA認証鍵
│   ├── private.pem       # 秘密鍵
│   └── public.pem        # 公開鍵
│
├── certs/                # SSL証明書（自動生成）
│   ├── service-1.crt
│   └── service-1.key
│
├── data/                 # .envファイル（自動生成）
│   └── my-grpc-service.env
│
├── processes/            # プロセス専用ディレクトリ
│   └── my-grpc-service/
│       └── service.exe   # gRPCサービス
│
└── logs/                 # ログファイル
```

## API

### REST API

**プロセス管理:**
```
GET    /api/v1/processes                    # プロセス一覧
GET    /api/v1/processes/:name/status       # プロセスステータス
POST   /api/v1/processes/:name/start        # プロセス起動
POST   /api/v1/processes/:name/stop         # プロセス停止
```

**更新管理（Hot Deploy）:**
```
POST   /api/v1/processes/:name/update       # プロセス更新（最新版または指定バージョン）
GET    /api/v1/processes/:name/version      # バージョン情報・更新ステータス取得
POST   /api/v1/processes/:name/rollback     # 前バージョンへロールバック
```

**Webhook（Cloudflare統合時）:**
```
POST   /webhook/github                      # GitHub直接Webhook
POST   /webhook/cloudflare                  # Cloudflare Workers Webhook
```

**ヘルスチェック:**
```
GET    /health                              # サーバーヘルスチェック
```

### 更新API使用例

**最新バージョンへ更新:**
```bash
curl -X POST http://localhost:8080/api/v1/processes/my-service/update
```

**特定バージョンへ更新:**
```bash
curl -X POST http://localhost:8080/api/v1/processes/my-service/update \
  -H "Content-Type: application/json" \
  -d '{"version": "v1.2.3"}'
```

**更新ステータス確認:**
```bash
curl http://localhost:8080/api/v1/processes/my-service/version
```

**ロールバック:**
```bash
curl -X POST http://localhost:8080/api/v1/processes/my-service/rollback
```

## 関連リポジトリ

- [cloudflare-auth-worker](https://github.com/yhonda-ohishi-pub-dev/cloudflare-auth-worker) - Secret管理サーバー
- [github-webhook-worker](https://github.com/yhonda-ohishi-pub-dev/github-webhook-worker) - バージョン管理サーバー
- [go_auth](https://github.com/yhonda-ohishi-pub-dev/go_auth) - 認証クライアントライブラリ

## ドキュメント

- [DESIGN.md](DESIGN.md) - 詳細設計ドキュメント

## セキュリティ

- `.env`ファイルには機密情報が含まれます。必ず`.gitignore`に追加してください。
- RSA秘密鍵は安全に保管してください。
- 本番環境では信頼されたCA証明書の使用を推奨します。

## ライセンス

MIT

## 作成者

Generated with Claude Code
