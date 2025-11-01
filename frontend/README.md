# GoWinProc Frontend

React + TypeScript + Vite で構築された GoWinProc のウェブフロントエンドです。

## 機能

- **サーバー統計ダッシュボード**
  - リアルタイムサーバーステータス表示
  - 総プロセス数と稼働インスタンス数
  - 3秒ごとの自動更新

- **プロセス管理ダッシュボード**
  - プロセス一覧表示（アルファベット順ソート）
  - プロセスの起動・停止・再起動
  - インスタンスの個別制御
  - リアルタイムステータス監視（5秒ごと）

- **メトリクス可視化**
  - CPU使用率
  - メモリ使用量
  - ディスクI/O
  - ネットワーク使用量
  - インスタンスごとの詳細メトリクス

- **自動更新管理**
  - 利用可能な更新の確認
  - プロセスの個別更新
  - 全プロセス一括更新
  - バージョン履歴とロールバック
  - リリースノート表示

- **スケーリング**
  - インスタンス数の動的変更
  - min/max制約の確認

## 必要要件

- Node.js 18+ または npm
- GoWinProc バックエンドが起動している必要があります（デフォルト: http://localhost:8080）

## セットアップ

### 1. 依存関係のインストール

```bash
cd frontend
npm install
```

### 2. 開発サーバーの起動

```bash
npm run dev
```

ブラウザで http://localhost:3000 を開きます。

### 3. プロダクションビルド

```bash
npm run build
```

ビルドされたファイルは `dist/` ディレクトリに出力されます。

## 設定

### バックエンドAPIのURL

デフォルトでは、開発環境で Vite のプロキシ機能を使用して `http://localhost:8080` に接続します。

[vite.config.ts](vite.config.ts:8-13) で変更できます：

```typescript
server: {
  port: 3000,
  proxy: {
    '/api': {
      target: 'http://localhost:8080',  // バックエンドのURL
      changeOrigin: true,
    },
  },
}
```

プロダクション環境では、同じホストから配信するか、CORS設定が必要です。

## プロジェクト構造

```
frontend/
├── src/
│   ├── api/              # API クライアント
│   │   └── client.ts     # REST API 呼び出し
│   ├── components/       # React コンポーネント
│   │   ├── ProcessList.tsx       # プロセス一覧
│   │   ├── ProcessDetail.tsx     # プロセス詳細
│   │   ├── InstanceList.tsx      # インスタンス一覧
│   │   ├── MetricsChart.tsx      # メトリクス表示
│   │   └── UpdateManager.tsx     # 更新管理
│   ├── styles/           # CSS スタイル
│   ├── types/            # TypeScript 型定義
│   │   └── index.ts
│   ├── App.tsx           # メインアプリ
│   └── main.tsx          # エントリポイント
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
└── README.md
```

## 使用方法

### プロセスの管理

1. 左サイドバーからプロセスを選択
2. プロセス詳細画面で以下の操作が可能：
   - **Start**: プロセスを起動
   - **Stop**: すべてのインスタンスを停止
   - **Restart**: すべてのインスタンスを再起動
   - **Scale**: インスタンス数を変更

### インスタンスの個別制御

プロセス詳細の「Instances」セクションで：
- **↻**: インスタンスを再起動
- **■**: インスタンスを停止

### 更新の管理

1. 上部の「Updates」タブをクリック
2. 各プロセスの現在のバージョンと最新バージョンを確認
3. 以下の操作が可能：
   - **Update to Latest**: 最新バージョンに更新
   - **Rollback**: 前のバージョンにロールバック
   - **Update All**: 全プロセスを一括更新

## API 仕様

このフロントエンドは以下の REST API エンドポイントを使用します：

### サーバー情報
- `GET /api/v1/status` - サーバー統計（プロセス数、インスタンス数）
- `GET /health` - サーバーヘルスチェック

### プロセス管理
- `GET /api/v1/processes` - プロセス一覧
- `GET /api/v1/processes/:name/status` - プロセス詳細
- `POST /api/v1/processes/:name/start` - プロセス起動
- `POST /api/v1/processes/:name/stop` - プロセス停止
  - Body: `{ "instance_id": "...", "all": true }`
- `POST /api/v1/processes/:name/restart` - プロセス再起動
- `GET /api/v1/processes/:name/metrics` - メトリクス取得
- `POST /api/v1/processes/:name/scale` - スケーリング
  - Body: `{ "target_instances": 3 }`

### 更新管理
- `GET /api/v1/updates` - 利用可能な更新一覧
- `GET /api/v1/processes/:name/version` - バージョン情報
- `POST /api/v1/processes/:name/update` - プロセス更新
  - Body: `{ "version": "v1.2.3", "force": false }`
- `POST /api/v1/processes/:name/rollback` - ロールバック
  - Body: `{ "version": "v1.2.2" }`
- `POST /api/v1/updates/all` - 全プロセス更新

## 開発

### 自動リロード

開発サーバーはファイル変更を監視し、自動的にリロードします。

### TypeScript型チェック

```bash
npm run build
```

ビルド時に型チェックが実行されます。

## バックエンド統合

### REST API (デフォルト)

現在、REST API を使用してバックエンド（ポート 8080）と通信しています。

### gRPC (オプション)

バックエンドは gRPC サーバー（ポート 9090）も提供していますが、フロントエンドは現在 REST API のみを使用しています。

将来的に gRPC-Web サポートを追加する場合：

1. Protocol Buffers コンパイラをインストール
2. gRPC-Web 用の TypeScript コードを生成：

```bash
npm run proto
```

これにより、`src/proto/` ディレクトリに型定義が生成されます。

## トラブルシューティング

### バックエンドに接続できない

1. GoWinProc が起動していることを確認：
   ```bash
   curl http://localhost:8080/health
   ```

2. [vite.config.ts](vite.config.ts) のプロキシ設定を確認

### ビルドエラー

1. Node.js のバージョンを確認（18+ 必要）
2. 依存関係を再インストール：
   ```bash
   rm -rf node_modules package-lock.json
   npm install
   ```

## ライセンス

MIT

## 作成者

Generated with Claude Code
