# gRPC-Web Implementation Plan for TunnelService

## 概要

Cloudflare TunnelがHTTP/2プロトコルを強制するため、既存のHTTP JSONエンドポイント（`/api/registry`と`/api/invoke`）が405エラーで動作しない問題を解決するため、gRPC-Webとして実装し直す。

## 現状

### 動作している部分
- ✅ ローカル開発環境：`LOCAL_TUNNEL_URLS`環境変数でlocalhost:8080に直接アクセス
- ✅ gowinprocにgRPCサーバーとgRPC-Webラッパーが既に実装済み（ProcessManager用）
- ✅ 既存のHTTP JSONエンドポイント：`/api/registry`, `/api/invoke`
- ✅ front-jsのフロントエンド実装（HTTP JSONベース）

### 問題点
- ❌ Cloudflare Tunnel経由でのアクセス：HTTP/2プロトコルのため全てのリクエストがgRPC-Webとして扱われる
- ❌ GETメソッド拒否：`invalid gRPC request method "GET"` (405エラー)
- ❌ Content-Type拒否：`invalid gRPC request content-type "application/json"`

## 実装ステップ

### Phase 1: protoファイルとコード生成

#### 1.1 protoファイル作成 ✅
- ファイル: `src/internal/proto/tunnel_service.proto`
- サービス定義:
  - `GetRegistry()` - レジストリ情報取得
  - `InvokeMethod()` - gRPCメソッド呼び出し
- 既に作成済み

#### 1.2 Goコード生成
```bash
cd C:/go/gowinproc
protoc \
  --go_out=. \
  --go-grpc_out=. \
  --proto_path=src/internal/proto \
  src/internal/proto/tunnel_service.proto
```

生成されるファイル:
- `src/internal/proto/tunnel_service.pb.go` - メッセージ定義
- `src/internal/proto/tunnel_service_grpc.pb.go` - サービススタブ

### Phase 2: gowinproc側の実装

#### 2.1 TunnelService実装
新規ファイル: `src/internal/grpc/tunnel_service.go`

```go
package grpcserver

import (
    "context"
    "encoding/json"

    pb "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/proto"
    "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/handlers"
)

type TunnelServiceServer struct {
    pb.UnimplementedTunnelServiceServer
    registryHandler *handlers.RegistryHandler
    invokeHandler   *handlers.GrpcInvokeHandler
}

func NewTunnelServiceServer(
    regHandler *handlers.RegistryHandler,
    invHandler *handlers.GrpcInvokeHandler,
) *TunnelServiceServer {
    return &TunnelServiceServer{
        registryHandler: regHandler,
        invokeHandler:   invHandler,
    }
}

func (s *TunnelServiceServer) GetRegistry(
    ctx context.Context,
    req *pb.RegistryRequest,
) (*pb.RegistryResponse, error) {
    // 既存のregistryHandler.GetRegistryのロジックを使用
    // HTTPレスポンスの代わりにpb.RegistryResponseを返す
    // TODO: 実装
}

func (s *TunnelServiceServer) InvokeMethod(
    ctx context.Context,
    req *pb.InvokeRequest,
) (*pb.InvokeResponse, error) {
    // 既存のinvokeHandler.InvokeMethodのロジックを使用
    // HTTPレスポンスの代わりにpb.InvokeResponseを返す
    // TODO: 実装
}
```

#### 2.2 main.goでサービス登録
ファイル: `src/cmd/gowinproc/main.go`

現在のコード (line 257-259):
```go
grpcSrv := grpc.NewServer()
grpcServiceServer := grpcserver.NewServer(processManager, updateManager, repositoryList)
pb.RegisterProcessManagerServer(grpcSrv, grpcServiceServer)
```

追加するコード:
```go
// TunnelService用のハンドラーを取得（既存のものを再利用）
registryHandler := handlers.NewRegistryHandler(processManager, "localhost", 8080)
invokeHandler := handlers.NewGrpcInvokeHandler(processManager)

// TunnelServiceを登録
tunnelServiceServer := grpcserver.NewTunnelServiceServer(registryHandler, invokeHandler)
pb.RegisterTunnelServiceServer(grpcSrv, tunnelServiceServer)
```

### Phase 3: front-js側の実装

#### 3.1 TypeScriptコード生成 ✅
既に完了:
- `ui/proto/tunnel_service.proto`
- `ui/src/proto/tunnel_service.ts` (生成済み)

#### 3.2 gRPC-Webクライアント実装 ✅
既に完了:
- `ui/src/api/grpc-tunnel-client.ts`

#### 3.3 既存APIクライアントの置き換え
ファイル: `ui/src/api/client.ts`

現在の実装:
```typescript
// HTTP JSON
export async function fetchGrpcRegistry(clientId: string): Promise<RegistryResponse>
export async function executeGrpcWebRequest(...)
```

新しい実装:
```typescript
import { createTunnelClient } from './grpc-tunnel-client'

export async function fetchGrpcRegistry(clientId: string): Promise<RegistryResponse> {
  const tunnelUrl = await getTunnelUrl(clientId) // Worker経由でtunnel URLを取得
  const client = createTunnelClient(tunnelUrl)
  const response = await client.getRegistry()

  // pb.RegistryResponse を RegistryResponse に変換
  return convertToRegistryResponse(response)
}
```

## テスト計画

### 1. ローカルテスト
```bash
# gowinprocを起動
cd C:/go/gowinproc
go run src/cmd/gowinproc/main.go

# front-jsのUIを起動
cd c:/js/front-js/ui
npm run dev

# ブラウザで http://localhost:5173 にアクセス
# LOCAL_TUNNEL_URLS=gowinproc=http://127.0.0.1:8080 で動作確認
```

### 2. Cloudflare Tunnel経由テスト
```bash
# front-jsをデプロイ
cd c:/js/front-js
npm run deploy

# ブラウザで https://front-js.m-tama-ramu.workers.dev にアクセス
# Cloudflare Tunnel経由で動作確認
```

### 3. 確認ポイント
- [ ] `/api/registry` がgRPC-Webプロトコルで正常に応答
- [ ] `/api/invoke` がgRPC-Webプロトコルで正常に応答
- [ ] サービスリストが表示される
- [ ] gRPCメソッドが実行できる
- [ ] レスポンスが表示される（白文字問題も解決済み）

## 依存関係

### gowinproc側
- `google.golang.org/grpc` (既にインストール済み)
- `google.golang.org/protobuf` (既にインストール済み)
- `github.com/improbable-eng/grpc-web` (既にインストール済み)

### front-js側
- `@improbable-eng/grpc-web` ✅ (インストール済み)
- `google-protobuf` ✅ (インストール済み)
- `ts-proto` ✅ (インストール済み)

## 参考資料

### 既存実装
- `C:/go/gowinproc/src/internal/proto/process_manager.proto` - ProcessManager protoファイル
- `C:/go/gowinproc/src/internal/grpc/server.go` - ProcessManager実装
- `C:/go/gowinproc/frontend/src/api/grpc-client.ts` - フロントエンドgRPC-Webクライアント

### ドキュメント
- gRPC-Web公式: https://github.com/grpc/grpc-web
- @improbable-eng/grpc-web: https://github.com/improbable-eng/grpc-web
- ts-proto: https://github.com/stephenh/ts-proto

## 注意事項

1. **既存のHTTP JSONエンドポイントを削除しない**
   - ローカル開発環境では引き続き使用可能にする
   - 後方互換性のため残しておく

2. **段階的な移行**
   - まずローカル環境で動作確認
   - その後Cloudflare Tunnel経由でテスト
   - 問題があれば切り戻しできるようにする

3. **エラーハンドリング**
   - gRPC-Webのエラーコードを適切にハンドル
   - ユーザーにわかりやすいエラーメッセージを表示

## タイムライン

- [ ] **Phase 1**: protoファイルとコード生成 (30分)
- [ ] **Phase 2**: gowinproc側実装 (2-3時間)
- [ ] **Phase 3**: front-js側実装 (1-2時間)
- [ ] **テスト**: 動作確認とデバッグ (1-2時間)

**合計見積もり**: 5-8時間

## 現在の進捗

### 完了済み ✅

- [x] **Phase 1.1**: protoファイル作成
  - `C:/go/gowinproc/src/internal/proto/tunnel_service.proto` 作成完了
  - 型名の衝突を避けるため、すべてのメッセージに"Tunnel"プレフィックスを追加
  - `TunnelRegistryRequest`, `TunnelRegistryResponse`, `TunnelInvokeRequest`, `TunnelInvokeResponse`

- [x] **Phase 1.2**: gowinproc Goコード生成
  - `src/internal/proto/tunnel_service.pb.go` - メッセージ定義
  - `src/internal/proto/tunnel_service_grpc.pb.go` - サービススタブ
  - 生成完了

- [x] **Phase 2.1**: TunnelService実装
  - `src/internal/grpc/tunnel_service.go` 実装完了
  - `GetRegistry()` - レジストリ情報を返すgRPCメソッド
  - `InvokeMethod()` - gRPCメソッド呼び出しを中継
  - 既存のHTTPハンドラーロジックを再利用

- [x] **Phase 2.2**: main.goでサービス登録
  - TunnelServiceをgRPCサーバーに登録 (main.go:261-267)
  - gRPC-Web経由でアクセス可能
  - エンドポイント: `/tunnel.TunnelService/GetRegistry`, `/tunnel.TunnelService/InvokeMethod`

- [x] **Phase 2.3**: 既存ハンドラーの改善
  - `registry_handler.go`: `GetRegistryData()` メソッド追加（HTTPとgRPCの両方で使用）
  - `grpc_invoke_handler.go`: `InvokeMethodDirect()` メソッド追加

- [x] **Phase 3.2**: front-js gRPC-Webクライアント実装（初回）
  - `ui/src/api/grpc-tunnel-client.ts` 作成完了
  - @improbable-eng/grpc-web を使用

- [x] **Phase 3.1**: front-js TypeScriptコード再生成 ✅
  - protoファイルを更新（gowinprocから最新版をコピー）
  - `node scripts/generate-proto.cjs` で再生成完了
  - 新しい型名: `TunnelRegistryRequest`, `TunnelRegistryResponse`, `TunnelInvokeRequest`, `TunnelInvokeResponse`

- [x] **Phase 3.2b**: gRPC-Webクライアント修正 ✅
  - `grpc-tunnel-client.ts` を新しい型名に更新
  - TypeScript構文エラーを修正（`private` パラメーター → プロパティ宣言）
  - サービス名を `'tunnel.TunnelService'` に統一

- [x] **Phase 3.3**: 既存APIクライアントの置き換え ✅
  - `ui/src/api/client.ts` の `fetchGrpcRegistry()` を更新
  - `executeGrpcWebRequest()` を更新
  - ビルド成功、デプロイ完了

- [x] **Phase 3.4**: Worker側のgRPC-Webルーティング追加 ✅
  - `src/index.ts` に `/tunnel/:id/tunnel.TunnelService/*` ルートを追加
  - gRPC-Webリクエストを正しくgowinprocにプロキシ
  - CORSヘッダーとgRPC-Webヘッダーを適切に処理
  - 再デプロイ完了

### テスト 🔄

- [ ] **ローカルテスト**: localhost:8080での動作確認
- [ ] **Cloudflare Tunnel経由テスト**: 本番環境での動作確認
  - URL: https://front-js.m-tama-ramu.workers.dev
  - gRPC-Webプロトコルでの通信確認

### 現在の問題 ❌

#### 問題1: gowinprocがgRPC-Webリクエストを正しく処理していない

**症状**:
- Cloudflare Tunnel経由で `/tunnel.TunnelService/GetRegistry` にPOSTリクエストを送信
- リクエストヘッダーは正しい: `Content-Type: application/grpc-web+proto`, `X-Grpc-Web: 1`
- レスポンス: HTTP 200 OK だが、`Content-Type: text/html` でHTMLが返る
- HTMLの内容: gowinprocのデフォルトページ（"Desktop Server is Running"）
- ブラウザエラー: `Error: Response closed without grpc-status (Headers only)`

**検証結果**:
```bash
# 最新のTunnel URL: https://booking-aging-galleries-words.trycloudflare.com
curl -X POST "https://booking-aging-galleries-words.trycloudflare.com/tunnel.TunnelService/GetRegistry" \
  -H "Content-Type: application/grpc-web+proto" \
  -H "X-Grpc-Web: 1"
# => HTTP/1.1 200 OK
# => Content-Type: text/html (期待: application/grpc-web+proto)
# => grpc-statusヘッダーなし
# => <!DOCTYPE html>...<h1>Desktop Server is Running</h1>...
```

**根本原因**:
`wrappedGrpc.IsGrpcWebRequest(r)` が `/tunnel.TunnelService/*` リクエストを正しくgRPC-Webリクエストとして認識していない。そのため、リクエストが通常のHTTP muxに渡され、HTMLページが返される。

**試した解決策**:
- ❌ [main.go:381-389](C:/go/gowinproc/src/cmd/gowinproc/main.go#L381-L389) に明示的なルーティングを追加
  - `/tunnel.TunnelService/*` パスを検出して `wrappedGrpc.ServeHTTP()` に渡す
  - ユーザーの要望により元に戻した

**次の対応方針**:
1. **原因調査**: `wrappedGrpc.IsGrpcWebRequest()` がなぜ失敗するのか
   - gRPC-Webライブラリのソースを確認
   - Content-Typeやヘッダーのチェック条件を確認
2. **代替案1**: パスベースのルーティングを再度実装
   - `/tunnel.TunnelService/*` を明示的にgRPC-Webハンドラーにルーティング
3. **代替案2**: 既存のHTTPエンドポイント(`/api/registry`, `/api/invoke`)を使い続ける
   - gRPC-Web移行を諦め、HTTP JSONで運用
4. **代替案3**: 別のgRPC-Webライブラリを検討
   - `github.com/improbable-eng/grpc-web` の代わりに公式の `grpc-web` を使用

## 次のステップ - 選択肢

### 選択肢A: gRPC-Web実装を完成させる（推奨）

**手順**:
1. `wrappedGrpc.IsGrpcWebRequest()` の動作を調査
2. `/tunnel.TunnelService/*` を明示的にgRPC-Webハンドラーにルーティング
3. テストして動作確認

**メリット**:
- Cloudflare TunnelのHTTP/2と互換性が高い
- 将来的にネイティブgRPCに近い実装

**デメリット**:
- デバッグに時間がかかる可能性

### 選択肢B: HTTP JSONエンドポイントを継続使用

**手順**:
1. 既存の `/api/registry` と `/api/invoke` を使い続ける
2. front-js UIを元のHTTP JSON実装に戻す
3. Cloudflare TunnelでのHTTP/2問題は未解決のまま

**メリット**:
- すぐに動作する（ローカル環境では既に動作済み）
- 実装がシンプル

**デメリット**:
- Cloudflare Tunnel経由で405エラーが発生し続ける
- 本番環境で使えない

## テスト手順（選択肢Aを選んだ場合）

### 1. ローカルテスト

gowinprocを起動してローカルでテスト：

```bash
# gowinprocを起動（localhost:8080）
cd C:/go/gowinproc
go run src/cmd/gowinproc/main.go

# curlでテスト
curl -v -X POST "http://localhost:8080/tunnel.TunnelService/GetRegistry" \
  -H "Content-Type: application/grpc-web+proto" \
  -H "X-Grpc-Web: 1" \
  --data-binary ""

# 別ターミナルでfront-js UIを起動
cd c:/js/front-js/ui
# .envでVITE_LOCAL_TUNNEL_URLS=gowinproc=http://127.0.0.1:8080 を設定
npm run dev

# ブラウザで http://localhost:5173 にアクセス
# gRPC-Web経由でレジストリとメソッド呼び出しをテスト
```

### 2. Cloudflare Tunnel経由テスト

本番環境でテスト：

```bash
# gowinprocが起動していることを確認
# Cloudflare Tunnelが有効になっていることを確認

# 最新のTunnel URL
# https://booking-aging-galleries-words.trycloudflare.com

# curlでテスト
curl -v -X POST "https://booking-aging-galleries-words.trycloudflare.com/tunnel.TunnelService/GetRegistry" \
  -H "Content-Type: application/grpc-web+proto" \
  -H "X-Grpc-Web: 1" \
  --data-binary ""

# ブラウザで https://front-js.m-tama-ramu.workers.dev にアクセス
# gRPC-Webプロトコルでの通信を確認
```

### 確認ポイント

- [ ] `grpc-status` ヘッダーがレスポンスに含まれる
- [ ] `Content-Type: application/grpc-web+proto` が返る
- [ ] HTMLではなくバイナリデータが返る
- [ ] ブラウザで "Response closed without grpc-status" エラーが出ない
- [ ] レジストリ情報が正常に表示される
- [ ] サービスとメソッドのリストが表示される
- [ ] gRPCメソッドが実行できる
