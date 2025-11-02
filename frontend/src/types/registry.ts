// gRPCレジストリAPI型定義

export type ProcessStatus = 'running' | 'stopped'

export interface MessageField {
  name: string
  type: string
  repeated: boolean
  number: number
  optional: boolean
}

export interface GrpcMethod {
  name: string
  input_type: string
  output_type: string
  input_schema?: MessageField[]  // Optional: populated if backend provides schema info
  output_schema?: MessageField[] // Optional: populated if backend provides schema info
}

export interface GrpcService {
  name: string
  methods: GrpcMethod[]
}

export interface MessageDetail {
  name: string
  fields: MessageField[]
}

export interface ProcessInfo {
  name: string
  display_name: string
  status: ProcessStatus
  instances: number
  proxy_path: string
  repository: string
  current_ports: number[]
  services?: GrpcService[]  // gRPC Reflectionで取得したサービスとメソッド一覧
  messages?: Record<string, MessageDetail>  // メッセージ型名 -> メッセージスキーマ
}

export interface RegistryResponse {
  proxy_base_url: string
  available_processes: ProcessInfo[]
  timestamp: string
}
