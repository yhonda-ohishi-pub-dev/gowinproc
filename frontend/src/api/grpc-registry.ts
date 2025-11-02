import { grpc } from '@improbable-eng/grpc-web'
import type { ProcessInfo } from '../types/registry'
import { registryApi } from './registry-client'

// メッセージラッパーヘルパー
function createMessageWrapper(data: Uint8Array) {
  return {
    serializeBinary: () => data,
  }
}

/**
 * プロキシパス対応のgRPC-Web RPCクライアント
 */
class ProxyGrpcWebRpc {
  constructor(private proxyPath: string) {}

  request(service: string, method: string, data: Uint8Array): Promise<Uint8Array> {
    const methodDesc = this.getMethodDescriptor(service, method)
    const wrappedRequest = createMessageWrapper(data)

    // プロキシパスからプロセス名を抽出 (/proxy/process_name)
    const processName = this.proxyPath.replace('/proxy/', '')

    // gRPC-Webのホストは http://host:port/proxy/process_name
    // grpc.unaryが自動的に /{service}/{method} を追加します
    const host = `http://127.0.0.1:8080/proxy/${processName}`

    console.log(`[gRPC-Web] Request to: ${host}/${service}/${method}`)

    return new Promise((resolve, reject) => {
      grpc.unary(methodDesc, {
        request: wrappedRequest as any,
        host,
        metadata: new grpc.Metadata(),
        onEnd: (response) => {
          const { status, statusMessage, message } = response
          if (status === grpc.Code.OK && message) {
            const responseData = (message as any).serializeBinary?.() || message
            resolve(responseData as Uint8Array)
          } else {
            reject(new Error(statusMessage || `gRPC error: ${status}`))
          }
        },
      })
    })
  }

  clientStreamingRequest(): Promise<Uint8Array> {
    throw new Error('Client streaming not implemented')
  }

  serverStreamingRequest(): any {
    throw new Error('Server streaming not implemented')
  }

  bidirectionalStreamingRequest(): any {
    throw new Error('Bidirectional streaming not implemented')
  }

  private getMethodDescriptor(service: string, method: string): any {
    return {
      methodName: method,
      service: { serviceName: service },
      requestStream: false,
      responseStream: false,
      requestType: {
        serializeBinary: (msg: any) => msg.serializeBinary(),
        deserializeBinary: (bytes: Uint8Array) => createMessageWrapper(bytes),
      },
      responseType: {
        serializeBinary: (msg: any) => msg.serializeBinary(),
        deserializeBinary: (bytes: Uint8Array) => createMessageWrapper(bytes),
      },
    }
  }
}

/**
 * gRPCクライアントレジストリ
 * プロセスごとの動的なgRPCクライアントを管理
 */
export class GrpcClientRegistry {
  private clients: Map<string, ProxyGrpcWebRpc> = new Map()

  /**
   * 指定されたプロセスのgRPCクライアントを取得（なければ作成）
   * @param processInfo プロセス情報
   * @returns gRPC RPCクライアント
   */
  getClient(processInfo: ProcessInfo): ProxyGrpcWebRpc {
    const { name, proxy_path } = processInfo

    if (!this.clients.has(name)) {
      this.clients.set(name, new ProxyGrpcWebRpc(proxy_path))
    }

    return this.clients.get(name)!
  }

  /**
   * 指定されたプロセス名のgRPCクライアントを取得
   * @param processName プロセス名
   * @param proxyPath プロキシパス
   * @returns gRPC RPCクライアント
   */
  getClientByName(processName: string, proxyPath: string): ProxyGrpcWebRpc {
    if (!this.clients.has(processName)) {
      this.clients.set(processName, new ProxyGrpcWebRpc(proxyPath))
    }

    return this.clients.get(processName)!
  }

  /**
   * すべてのクライアントをクリア
   */
  clear(): void {
    this.clients.clear()
  }

  /**
   * 登録されているクライアント数を取得
   */
  size(): number {
    return this.clients.size
  }
}

// シングルトンインスタンス
export const grpcClientRegistry = new GrpcClientRegistry()
