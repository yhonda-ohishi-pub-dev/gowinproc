import { grpc } from '@improbable-eng/grpc-web'
import * as pb from '../proto/process_manager'

// gRPC-Web endpoint (port 8080)
const GRPC_HOST = 'http://127.0.0.1:8080'

// Helper to create a message wrapper for @improbable-eng/grpc-web
function createMessageWrapper(data: Uint8Array) {
  return {
    serializeBinary: () => data,
  }
}

// RPC implementation for @improbable-eng/grpc-web
class GrpcWebRpc implements pb.Rpc {
  request(service: string, method: string, data: Uint8Array): Promise<Uint8Array> {
    const methodDesc = this.getMethodDescriptor(service, method)
    const wrappedRequest = createMessageWrapper(data)

    return new Promise((resolve, reject) => {
      grpc.unary(methodDesc, {
        request: wrappedRequest as any,
        host: GRPC_HOST,
        metadata: new grpc.Metadata(),
        onEnd: (response) => {
          const { status, statusMessage, message } = response
          if (status === grpc.Code.OK && message) {
            // message is the wrapped response, extract the Uint8Array
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

// Create RPC instance
const rpc = new GrpcWebRpc()

// gRPC Client API using generated types
export const grpcProcessApi = {
  // プロセス一覧取得
  listProcesses: async (): Promise<pb.ListProcessesResponse> => {
    const request = pb.ListProcessesRequest.encode({}).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'ListProcesses', request)
    return pb.ListProcessesResponse.decode(response)
  },

  // プロセス詳細取得
  getProcess: async (processName: string): Promise<pb.ProcessInfo> => {
    const request = pb.GetProcessRequest.encode({ processName }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'GetProcess', request)
    return pb.ProcessInfo.decode(response)
  },

  // プロセス起動
  startProcess: async (processName: string): Promise<pb.ProcessInfo> => {
    const request = pb.StartProcessRequest.encode({ processName }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'StartProcess', request)
    return pb.ProcessInfo.decode(response)
  },

  // プロセス停止
  stopProcess: async (processName: string, instanceId?: string, all?: boolean): Promise<pb.Empty> => {
    const request = pb.StopProcessRequest.encode({ processName, instanceId: instanceId || '', all: all || false }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'StopProcess', request)
    return pb.Empty.decode(response)
  },

  // プロセス再起動
  restartProcess: async (processName: string, instanceId?: string): Promise<pb.ProcessInfo> => {
    const request = pb.RestartProcessRequest.encode({ processName, instanceId: instanceId || '' }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'RestartProcess', request)
    return pb.ProcessInfo.decode(response)
  },

  // メトリクス取得
  getMetrics: async (processName: string, instanceId?: string): Promise<pb.Metrics> => {
    const request = pb.GetMetricsRequest.encode({ processName, instanceId: instanceId || '' }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'GetMetrics', request)
    return pb.Metrics.decode(response)
  },

  // スケーリング
  scaleProcess: async (processName: string, targetInstances: number): Promise<pb.ProcessInfo> => {
    const request = pb.ScaleProcessRequest.encode({ processName, targetInstances }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'ScaleProcess', request)
    return pb.ProcessInfo.decode(response)
  },

  // バージョン情報取得
  getVersion: async (processName: string): Promise<pb.VersionInfo> => {
    const request = pb.GetVersionRequest.encode({ processName }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'GetProcessVersion', request)
    return pb.VersionInfo.decode(response)
  },

  // 利用可能な更新一覧
  listAvailableUpdates: async (): Promise<pb.ListUpdatesResponse> => {
    const request = pb.ListUpdatesRequest.encode({}).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'ListAvailableUpdates', request)
    return pb.ListUpdatesResponse.decode(response)
  },

  // プロセス更新
  updateProcess: async (
    processName: string,
    options?: { version?: string; force?: boolean; strategy?: string; instances?: number[] }
  ): Promise<pb.UpdateResponse> => {
    const request = pb.UpdateProcessRequest.encode({
      processName,
      version: options?.version || '',
      force: options?.force || false,
      strategy: options?.strategy || '',
      instances: options?.instances || [],
    }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'UpdateProcess', request)
    return pb.UpdateResponse.decode(response)
  },

  // 全プロセス更新
  updateAll: async (options?: {
    strategy?: string
    force?: boolean
    timeout?: number
    healthCheckDelay?: number
  }): Promise<pb.UpdateResponse> => {
    const request = pb.UpdateAllRequest.encode({
      strategy: options?.strategy || '',
      force: options?.force || false,
      timeout: options?.timeout || 0,
      healthCheckDelay: options?.healthCheckDelay || 0,
    }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'UpdateAllProcesses', request)
    return pb.UpdateResponse.decode(response)
  },

  // ロールバック
  rollback: async (processName: string, version?: string): Promise<pb.RollbackResponse> => {
    const request = pb.RollbackRequest.encode({ processName, version: version || '' }).finish()
    const response = await rpc.request(pb.ProcessManagerServiceName, 'RollbackProcess', request)
    return pb.RollbackResponse.decode(response)
  },
}
