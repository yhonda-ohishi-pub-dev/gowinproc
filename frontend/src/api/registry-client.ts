import type { RegistryResponse } from '../types/registry'

// レジストリAPIのベースURL
const REGISTRY_API_BASE = 'http://127.0.0.1:8080'

/**
 * gRPCレジストリAPIクライアント
 */
export const registryApi = {
  /**
   * 利用可能なgRPCプロセス一覧を取得
   */
  async fetchRegistry(): Promise<RegistryResponse> {
    const response = await fetch(`${REGISTRY_API_BASE}/api/grpc/registry`)

    if (!response.ok) {
      throw new Error(`Failed to fetch registry: ${response.statusText}`)
    }

    return response.json()
  },

  /**
   * 指定されたプロセスのプロキシURLを生成
   * @param processName プロセス名
   * @param serviceName gRPCサービス名
   * @param methodName gRPCメソッド名
   * @returns プロキシURL
   */
  buildProxyUrl(processName: string, serviceName: string, methodName: string): string {
    return `${REGISTRY_API_BASE}/proxy/${processName}/${serviceName}/${methodName}`
  },

  /**
   * レジストリ情報をポーリングで定期取得
   * @param intervalMs ポーリング間隔（ミリ秒）
   * @param callback コールバック関数
   * @returns クリーンアップ関数
   */
  startPolling(
    intervalMs: number,
    callback: (registry: RegistryResponse) => void
  ): () => void {
    let intervalId: number | null = null
    let isActive = true

    const poll = async () => {
      if (!isActive) return

      try {
        const registry = await this.fetchRegistry()
        callback(registry)
      } catch (error) {
        console.error('Failed to poll registry:', error)
      }
    }

    // 初回実行
    poll()

    // 定期実行
    intervalId = window.setInterval(poll, intervalMs)

    // クリーンアップ関数を返す
    return () => {
      isActive = false
      if (intervalId !== null) {
        clearInterval(intervalId)
      }
    }
  }
}
