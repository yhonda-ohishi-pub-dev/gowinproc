// TypeScript types based on process_manager.proto

export interface ListProcessesRequest {}

export interface ListProcessesResponse {
  process_names: string[]
  count: number
}

export interface GetProcessRequest {
  process_name: string
}

export interface ProcessInfo {
  name: string
  instances: ProcessInstance[]
  instance_count: number
  config?: ProcessConfig
}

export interface ProcessInstance {
  id: string
  process_name: string
  pid: number
  status: string
  start_time: number  // Unix timestamp
  port: number
  env_file_path: string
  metrics?: ProcessMetrics
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
  update_check_interval: number  // seconds
}

export interface SecretsConfig {
  enabled: boolean
  source: string  // "local" or "cloudflare"
  env_file: string
}

export interface CertificatesConfig {
  enabled: boolean
  cert_path: string
  key_path: string
}

export interface StartProcessRequest {
  process_name: string
}

export interface StopProcessRequest {
  process_name: string
  instance_id?: string  // Optional - if empty, stops all instances
  all?: boolean
}

export interface RestartProcessRequest {
  process_name: string
  instance_id?: string  // Optional - if empty, restarts all instances
}

export interface GetMetricsRequest {
  process_name: string
  instance_id?: string  // Optional - if empty, returns aggregated metrics
}

export interface Metrics {
  process_name: string
  instances: ProcessMetrics[]
  aggregated?: AggregatedMetrics
}

export interface ProcessMetrics {
  instance_id: string
  cpu_usage: number      // percentage
  memory_usage: number   // bytes (use bigint for uint64)
  disk_read: number      // bytes
  disk_write: number     // bytes
  network_recv: number   // bytes
  network_sent: number   // bytes
  uptime: number         // seconds
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

export interface ScaleProcessRequest {
  process_name: string
  target_instances: number
}

export interface UpdateAllRequest {
  strategy?: string        // rolling, blue-green, immediate
  force?: boolean
  timeout?: number          // seconds
  health_check_delay?: number  // seconds
}

export interface UpdateProcessRequest {
  process_name: string
  version?: string         // empty string = latest version
  force?: boolean
  strategy?: string
  instances?: number[]  // specific instance indices (optional)
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
  status: string          // updating, completed, failed
  estimated_duration: number  // seconds
}

export interface GetVersionRequest {
  process_name: string
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
  uptime: number  // seconds
}

export interface ListUpdatesRequest {}

export interface ListUpdatesResponse {
  updates: UpdateAvailable[]
}

export interface UpdateAvailable {
  process_name: string
  current_version: string
  latest_version: string
  release_date: string
  release_notes: string
  up_to_date: boolean
}

export interface RollbackRequest {
  process_name: string
  version?: string  // empty string = previous version
}

export interface RollbackResponse {
  success: boolean
  process_name: string
  from_version: string
  to_version: string
  rollback_id: string
}

export interface WatchUpdateRequest {
  update_id: string
}

export interface UpdateStatus {
  update_id: string
  process_name: string
  status: string  // downloading, extracting, stopping, starting, completed, failed
  progress: number  // 0-100
  message: string
  timestamp: number  // Unix timestamp
}

export interface Empty {}
