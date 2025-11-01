import { ProcessInfo, Metrics, VersionInfo, UpdateAvailable, UpdateResponse } from '../types'

const API_BASE = '/api/v1'

export const processApi = {
  // プロセス一覧取得
  listProcesses: async (): Promise<{ processes: string[]; count: number }> => {
    const response = await fetch(`${API_BASE}/processes`)
    if (!response.ok) throw new Error('Failed to fetch processes')
    return response.json()
  },

  // プロセス詳細取得
  getProcess: async (name: string): Promise<ProcessInfo> => {
    const response = await fetch(`${API_BASE}/processes/${name}/status`)
    if (!response.ok) throw new Error(`Failed to fetch process: ${name}`)
    return response.json()
  },

  // プロセス起動
  startProcess: async (name: string): Promise<ProcessInfo> => {
    const response = await fetch(`${API_BASE}/processes/${name}/start`, {
      method: 'POST',
    })
    if (!response.ok) throw new Error(`Failed to start process: ${name}`)
    return response.json()
  },

  // プロセス停止
  stopProcess: async (name: string, instanceId?: string): Promise<void> => {
    const url = instanceId
      ? `${API_BASE}/processes/${name}/stop?instance_id=${instanceId}`
      : `${API_BASE}/processes/${name}/stop`

    const response = await fetch(url, {
      method: 'POST',
    })
    if (!response.ok) throw new Error(`Failed to stop process: ${name}`)
  },

  // プロセス再起動
  restartProcess: async (name: string, instanceId?: string): Promise<ProcessInfo> => {
    const url = instanceId
      ? `${API_BASE}/processes/${name}/restart?instance_id=${instanceId}`
      : `${API_BASE}/processes/${name}/restart`

    const response = await fetch(url, {
      method: 'POST',
    })
    if (!response.ok) throw new Error(`Failed to restart process: ${name}`)
    return response.json()
  },

  // メトリクス取得
  getMetrics: async (name: string, instanceId?: string): Promise<Metrics> => {
    const url = instanceId
      ? `${API_BASE}/processes/${name}/metrics?instance_id=${instanceId}`
      : `${API_BASE}/processes/${name}/metrics`

    const response = await fetch(url)
    if (!response.ok) throw new Error(`Failed to fetch metrics: ${name}`)
    return response.json()
  },

  // スケーリング
  scaleProcess: async (name: string, targetInstances: number): Promise<ProcessInfo> => {
    const response = await fetch(`${API_BASE}/processes/${name}/scale`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ target_instances: targetInstances }),
    })
    if (!response.ok) throw new Error(`Failed to scale process: ${name}`)
    return response.json()
  },

  // バージョン情報取得
  getVersion: async (name: string): Promise<VersionInfo> => {
    const response = await fetch(`${API_BASE}/processes/${name}/version`)
    if (!response.ok) throw new Error(`Failed to fetch version: ${name}`)
    return response.json()
  },

  // 利用可能な更新一覧
  listAvailableUpdates: async (): Promise<{ updates: UpdateAvailable[] }> => {
    const response = await fetch(`${API_BASE}/updates`)
    if (!response.ok) throw new Error('Failed to fetch available updates')
    return response.json()
  },

  // プロセス更新
  updateProcess: async (
    name: string,
    options?: {
      version?: string
      force?: boolean
      strategy?: string
    }
  ): Promise<UpdateResponse> => {
    const response = await fetch(`${API_BASE}/processes/${name}/update`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(options || {}),
    })
    if (!response.ok) throw new Error(`Failed to update process: ${name}`)
    return response.json()
  },

  // ロールバック
  rollback: async (name: string, version?: string): Promise<any> => {
    const response = await fetch(`${API_BASE}/processes/${name}/rollback`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ version }),
    })
    if (!response.ok) throw new Error(`Failed to rollback process: ${name}`)
    return response.json()
  },

  // 全プロセス更新
  updateAll: async (options?: {
    strategy?: string
    force?: boolean
    timeout?: number
    health_check_delay?: number
  }): Promise<UpdateResponse> => {
    const response = await fetch(`${API_BASE}/updates/all`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(options || {}),
    })
    if (!response.ok) throw new Error('Failed to update all processes')
    return response.json()
  },
}

// サーバー統計情報取得
export const getServerStatus = async (): Promise<{
  status: string
  processes: number
  total_instances: number
  running_instances: number
}> => {
  const response = await fetch(`${API_BASE}/status`)
  if (!response.ok) throw new Error('Failed to fetch server status')
  return response.json()
}

// ヘルスチェック
export const healthCheck = async (): Promise<{ status: string }> => {
  const response = await fetch('/health')
  if (!response.ok) throw new Error('Health check failed')
  return response.json()
}
