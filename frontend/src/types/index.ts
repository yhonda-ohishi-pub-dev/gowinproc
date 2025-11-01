export interface ProcessInstance {
  id: string
  process_name: string
  pid: number
  status: string
  start_time: number
  port: number
  env_file_path: string
  metrics?: ProcessMetrics
}

export interface ProcessMetrics {
  instance_id: string
  cpu_usage: number
  memory_usage: number
  disk_read: number
  disk_write: number
  network_recv: number
  network_sent: number
  uptime: number
}

export interface AggregatedMetrics {
  total_cpu_usage: number
  total_memory_usage: number
  total_disk_read: number
  total_disk_write: number
  total_network_recv: number
  total_network_sent: number
  instance_count: number
}

export interface ProcessConfig {
  name: string
  binary_path: string
  args: string[]
  work_dir: string
  port: number
  min_instances: number
  max_instances: number
  auto_restart: boolean
  github?: GitHubConfig
  secrets?: SecretsConfig
  certificates?: CertificatesConfig
}

export interface GitHubConfig {
  repo: string
  auto_update: boolean
  update_check_interval: number
}

export interface SecretsConfig {
  enabled: boolean
  source: string
  env_file: string
}

export interface CertificatesConfig {
  enabled: boolean
  cert_path: string
  key_path: string
}

export interface ProcessInfo {
  process?: string  // API returns "process" field
  name?: string
  instances: ProcessInstance[]
  instance_count?: number
  count?: number  // API returns "count" field
  config?: ProcessConfig  // config may not be present in all responses
}

export interface Metrics {
  process_name: string
  instances: ProcessMetrics[]
  aggregated: AggregatedMetrics
}

export interface VersionInfo {
  process_name: string
  current_version: string
  latest_version: string
  update_available: boolean
  instances: InstanceVersion[]
}

export interface InstanceVersion {
  id: string
  version: string
  uptime: number
}

export interface UpdateAvailable {
  process_name: string
  current_version: string
  latest_version: string
  release_date: string
  release_notes: string
  up_to_date: boolean
}

export interface UpdateResponse {
  success: boolean
  message: string
  update_id: string
  processes: ProcessUpdateStatus[]
}

export interface ProcessUpdateStatus {
  name: string
  current_version: string
  target_version: string
  status: string
  estimated_duration: number
}

export interface UpdateStatus {
  update_id: string
  process_name: string
  status: string
  progress: number
  message: string
  timestamp: number
}
