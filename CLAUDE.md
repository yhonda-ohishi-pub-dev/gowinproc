# Claude Code - GoWinProc 開発ガイド

## フロントエンド開発

### 起動方法

```bash
cd frontend
npm run dev
```

**重要**: フロントエンドは**ポート3000固定**で起動します。多重起動を防止するため、`vite.config.ts`で`strictPort: true`を設定しています。

- ポート3000が既に使用中の場合、エラーが表示されます
- 複数のフロントエンドインスタンスを同時に起動しないでください
- 起動前に既存のプロセスが停止していることを確認してください

### gRPC-Web 実装

フロントエンドは**gRPC-Web**を使用してバックエンドと通信します。

- **gRPCエンドポイント**: `http://127.0.0.1:8080` (HTTP server with gRPC-Web wrapper)
- **ネイティブgRPCサーバー**: `http://127.0.0.1:9090` (フロントエンドからは使用しません)
- **Protocol Buffers**: `src/internal/proto/process_manager.proto`に定義
- **生成されたTypeScript型**: `frontend/src/proto/process_manager.ts`（自動生成）
- **gRPCクライアント**: `frontend/src/api/grpc-client.ts`

#### gRPC-Web ライブラリ

```typescript
import { grpc } from '@improbable-eng/grpc-web'
import * as pb from '../proto/process_manager'
```

主要な実装ファイル:
- [frontend/src/api/grpc-client.ts](frontend/src/api/grpc-client.ts) - gRPC-Web クライアント実装
- [frontend/src/proto/process_manager.ts](frontend/src/proto/process_manager.ts) - 自動生成されたTypeScript型定義
- [frontend/src/App.tsx](frontend/src/App.tsx) - gRPC APIの使用例

### Protocol Buffers型生成

`.proto`ファイルを変更した場合、TypeScript型を再生成する必要があります:

```bash
cd frontend
npm run proto
```

このコマンドは以下を実行します:
1. `src/internal/proto/process_manager.proto`を`frontend/proto/`にコピー
2. `protoc`と`ts-proto`を使用してTypeScriptコードを生成
3. 生成されたファイルは`frontend/src/proto/process_manager.ts`に保存されます

**重要**:
- Windows環境でのprotocプラグインパスの問題を解決するため、専用スクリプト([scripts/generate-proto.js](frontend/scripts/generate-proto.js))を使用しています
- 手動でprotocを実行する必要はありません

### バックエンド要件

フロントエンドが正常に動作するには、バックエンドが以下の条件を満たす必要があります:

1. **HTTPサーバー**: ポート8080でgRPC-Webラッパー付きHTTPサーバーが起動している
2. **gRPC-Web互換レイヤー**: `improbable-eng/grpc-web` Goライブラリを使用
3. **CORS設定**: 開発環境ではすべてのオリジンを許可（[main.go:148-157](src/cmd/gowinproc/main.go#L148-L157)）

### 開発時の注意事項

1. **ポート管理**
   - フロントエンド: 3000番ポート（固定、多重起動禁止）
   - バックエンド gRPC-Web: 8080番ポート（フロントエンドからアクセス）
   - バックエンド ネイティブgRPC: 9090番ポート（サーバー間通信用）

2. **プロセス管理**
   - Claude Codeでフロントエンドを起動する場合、既存のインスタンスを必ず停止してください
   - 多重起動すると異なるポート（3001, 3002...）で起動しますが、これは避けてください

   **ポート3000のプロセスを停止する方法**:
   ```bash
   # 1. ポート3000を使用しているプロセスIDを確認
   netstat -ano | findstr ":3000.*LISTENING"

   # 2. プロセスIDを使ってプロセスを停止（例: PID が 12345 の場合）
   powershell -Command "Stop-Process -Id 12345 -Force"
   ```

3. **型定義の更新**
   - `.proto`ファイルを変更したら、必ず`npm run proto`を実行してTypeScript型を再生成してください
   - 生成されたファイル(`frontend/src/proto/process_manager.ts`)は直接編集しないでください

## バックエンド開発

（TODO: バックエンドの開発ガイドを追加）

## ビルドとデプロイ

（TODO: ビルドとデプロイのガイドを追加）
