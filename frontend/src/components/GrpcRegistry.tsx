import { useEffect, useState } from 'react'
import { registryApi } from '../api/registry-client'
import type { RegistryResponse, ProcessInfo } from '../types/registry'

export function GrpcRegistry() {
  const [registry, setRegistry] = useState<RegistryResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null)

  useEffect(() => {
    // レジストリ情報をポーリング（5秒ごと）
    const cleanup = registryApi.startPolling(5000, (data) => {
      setRegistry(data)
      setLoading(false)
      setError(null)
      setLastUpdate(new Date())
    })

    return cleanup
  }, [])

  if (loading) {
    return (
      <div style={{ padding: '20px' }}>
        <h2>gRPC Process Registry</h2>
        <p>Loading...</p>
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: '20px' }}>
        <h2>gRPC Process Registry</h2>
        <p style={{ color: 'red' }}>Error: {error}</p>
      </div>
    )
  }

  if (!registry) {
    return (
      <div style={{ padding: '20px' }}>
        <h2>gRPC Process Registry</h2>
        <p>No data available</p>
      </div>
    )
  }

  return (
    <div style={{
      padding: '20px',
      fontFamily: 'system-ui, sans-serif'
    }}>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: '20px'
      }}>
        <h2 style={{ margin: 0 }}>gRPC Process Registry</h2>
        <div style={{ fontSize: '14px', color: '#666' }}>
          Last update: {lastUpdate?.toLocaleTimeString()}
        </div>
      </div>

      <div style={{
        padding: '12px',
        backgroundColor: '#f5f5f5',
        borderRadius: '4px',
        marginBottom: '20px'
      }}>
        <div style={{ fontSize: '14px' }}>
          <strong>Proxy Base URL:</strong> {registry.proxy_base_url}
        </div>
        <div style={{ fontSize: '14px', marginTop: '8px' }}>
          <strong>Total Processes:</strong> {registry.available_processes.length}
        </div>
      </div>

      <div style={{
        display: 'grid',
        gap: '16px',
        gridTemplateColumns: 'repeat(auto-fill, minmax(450px, 1fr))',
        alignItems: 'start',
        gridAutoRows: 'min-content'
      }}>
        {registry.available_processes
          .sort((a, b) => a.name.localeCompare(b.name))
          .map((process) => (
            <ProcessCard key={process.name} process={process} />
          ))}
      </div>
    </div>
  )
}

function ProcessCard({ process }: { process: ProcessInfo }) {
  const [expanded, setExpanded] = useState(false)
  const [testResult, setTestResult] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)
  const [serviceName, setServiceName] = useState('')
  const [methodName, setMethodName] = useState('')
  const [requestJson, setRequestJson] = useState('{}')

  // Parse services into service->methods map from gRPC Reflection
  const serviceMethodsMap = new Map<string, Array<{
    name: string
    input_type: string
    output_type: string
    input_schema?: import('../types/registry').MessageField[]
    output_schema?: import('../types/registry').MessageField[]
  }>>()

  if (process.services) {
    process.services.forEach(service => {
      if (!service.name.startsWith('grpc.reflection')) {
        // Enrich methods with schema information from process.messages
        const enrichedMethods = service.methods.map(method => {
          const inputSchema = process.messages?.[method.input_type]?.fields
          const outputSchema = process.messages?.[method.output_type]?.fields
          return {
            ...method,
            input_schema: inputSchema,
            output_schema: outputSchema
          }
        })
        serviceMethodsMap.set(service.name, enrichedMethods)
      }
    })
  }

  const availableServices = Array.from(serviceMethodsMap.keys()).sort()
  const availableMethods = serviceName ? serviceMethodsMap.get(serviceName) || [] : []

  // 選択されたメソッドの型情報を取得
  const selectedMethodInfo = availableMethods.find(m => m.name === methodName)

  const statusColor = process.status === 'running' ? '#22c55e' : '#ef4444'
  const statusBg = process.status === 'running' ? '#dcfce7' : '#fee2e2'

  const handleExecuteMethod = async () => {
    if (!serviceName || !methodName) {
      setTestResult('❌ サービス名とメソッド名を入力してください')
      return
    }

    setTesting(true)
    setTestResult(null)

    try {
      const startTime = performance.now()

      // パラメータをパース
      let params = {}
      try {
        params = JSON.parse(requestJson)
      } catch (e) {
        throw new Error('Invalid JSON: ' + (e instanceof Error ? e.message : 'Unknown error'))
      }

      // grpcurlベースの /api/invoke エンドポイントを使用
      const response = await fetch('http://127.0.0.1:8080/api/invoke', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          process: process.name,
          service: serviceName,
          method: methodName,
          data: params,
        }),
      })

      const endTime = performance.now()
      const duration = (endTime - startTime).toFixed(2)

      if (!response.ok) {
        const errorText = await response.text()
        throw new Error(`HTTP ${response.status}: ${errorText}`)
      }

      const result = await response.json()

      if (!result.success) {
        throw new Error(result.error || 'Unknown error')
      }

      // レスポンスデータを整形
      const responseText = JSON.stringify(result.data, null, 2)

      setTestResult(`✅ Success (${duration}ms)\n\nResponse:\n${responseText}`)
    } catch (error) {
      setTestResult(`❌ Error: ${error instanceof Error ? error.message : 'Unknown error'}`)
    } finally {
      setTesting(false)
    }
  }

  return (
    <div style={{
      border: '1px solid #e5e7eb',
      borderRadius: '8px',
      padding: '16px',
      backgroundColor: '#fff',
      boxShadow: '0 1px 3px rgba(0,0,0,0.1)'
    }}>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'start',
        marginBottom: '12px',
        gap: '12px'
      }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <h3 style={{
            margin: '0 0 4px 0',
            fontSize: '16px',
            wordBreak: 'break-word'
          }}>
            {process.display_name}
          </h3>
          <div style={{
            fontSize: '11px',
            color: '#6b7280',
            wordBreak: 'break-word'
          }}>
            {process.repository}
          </div>
        </div>
        <div style={{
          padding: '4px 12px',
          borderRadius: '12px',
          fontSize: '12px',
          fontWeight: '600',
          backgroundColor: statusBg,
          color: statusColor,
          flexShrink: 0
        }}>
          {process.status}
        </div>
      </div>

      <div style={{
        display: 'grid',
        gap: '8px',
        fontSize: '14px'
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span style={{ color: '#6b7280' }}>Instances:</span>
          <span style={{ fontWeight: '500' }}>{process.instances}</span>
        </div>

        {process.current_ports.length > 0 && (
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <span style={{ color: '#6b7280' }}>Ports:</span>
            <span style={{ fontWeight: '500', fontFamily: 'monospace' }}>
              {process.current_ports.join(', ')}
            </span>
          </div>
        )}

        <div style={{
          marginTop: '8px',
          padding: '8px',
          backgroundColor: '#f9fafb',
          borderRadius: '4px',
          fontFamily: 'monospace',
          fontSize: '12px',
          wordBreak: 'break-all'
        }}>
          <div style={{ color: '#6b7280', marginBottom: '4px' }}>
            Proxy Path:
          </div>
          <div style={{ color: '#1f2937' }}>
            {process.proxy_path}
          </div>
        </div>
      </div>

      <div style={{ marginTop: '12px', display: 'flex', gap: '8px' }}>
        <button
          onClick={() => setExpanded(!expanded)}
          disabled={process.status !== 'running'}
          style={{
            flex: 1,
            padding: '8px',
            backgroundColor: process.status === 'running' ? '#10b981' : '#9ca3af',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: process.status === 'running' ? 'pointer' : 'not-allowed',
            fontSize: '14px',
            fontWeight: '500'
          }}
        >
          {expanded ? '▼ Hide Test Panel' : '▶ Test gRPC Call'}
        </button>
        <button
          onClick={() => {
            navigator.clipboard.writeText(process.proxy_path)
            alert('Proxy path copied!')
          }}
          style={{
            flex: 1,
            padding: '8px',
            backgroundColor: '#3b82f6',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '14px',
            fontWeight: '500'
          }}
        >
          Copy Path
        </button>
      </div>

      {expanded && (
        <div style={{
          marginTop: '12px',
          padding: '12px',
          backgroundColor: '#f9fafb',
          borderRadius: '4px',
          border: '1px solid #e5e7eb',
          maxHeight: '400px',
          overflow: 'auto'
        }}>
          <h4 style={{ margin: '0 0 12px 0', fontSize: '14px', fontWeight: '600' }}>
            gRPC Method Tester
          </h4>

          {availableServices.length > 0 && (
            <div style={{
              marginBottom: '12px',
              padding: '10px',
              backgroundColor: '#fff',
              borderRadius: '4px',
              fontSize: '12px'
            }}>
              <div style={{ fontWeight: '600', marginBottom: '8px', color: '#374151' }}>
                Available Services ({availableServices.length}):
              </div>
              <div style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '4px',
                fontFamily: 'monospace',
                fontSize: '11px',
                color: '#6b7280',
                maxHeight: '150px',
                overflow: 'auto'
              }}>
                {availableServices.map((svc, idx) => {
                  const methods = serviceMethodsMap.get(svc) || []
                  return (
                    <div key={idx}>
                      <div style={{ fontWeight: '600', color: '#1f2937' }}>• {svc}</div>
                      {methods.map((method, mIdx) => (
                        <div key={mIdx} style={{ marginLeft: '16px', color: '#6b7280' }}>
                          - {method.name}
                        </div>
                      ))}
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '12px', marginBottom: '4px', fontWeight: '500' }}>
                Service Name:
              </label>
              <select
                value={serviceName}
                onChange={(e) => {
                  setServiceName(e.target.value)
                  setMethodName('') // Reset method when service changes
                }}
                style={{
                  width: '100%',
                  padding: '8px',
                  fontSize: '13px',
                  fontFamily: 'monospace',
                  border: '1px solid #d1d5db',
                  borderRadius: '4px',
                  boxSizing: 'border-box',
                  backgroundColor: '#fff',
                  cursor: 'pointer'
                }}
              >
                <option value="">Select a service...</option>
                {availableServices.map((svc) => (
                  <option key={svc} value={svc}>
                    {svc}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '12px', marginBottom: '4px', fontWeight: '500' }}>
                Method Name:
              </label>
              <select
                value={methodName}
                onChange={(e) => setMethodName(e.target.value)}
                disabled={!serviceName || availableMethods.length === 0}
                style={{
                  width: '100%',
                  padding: '8px',
                  fontSize: '13px',
                  fontFamily: 'monospace',
                  border: '1px solid #d1d5db',
                  borderRadius: '4px',
                  boxSizing: 'border-box',
                  backgroundColor: serviceName ? '#fff' : '#f9fafb',
                  cursor: serviceName ? 'pointer' : 'not-allowed'
                }}
              >
                <option value="">Select a method...</option>
                {availableMethods.map((method) => (
                  <option key={method.name} value={method.name}>
                    {method.name}
                  </option>
                ))}
              </select>
            </div>

            {selectedMethodInfo && (
              <div style={{
                padding: '10px',
                backgroundColor: '#eff6ff',
                borderRadius: '4px',
                fontSize: '11px',
                fontFamily: 'monospace',
                border: '1px solid #bfdbfe'
              }}>
                <div style={{ marginBottom: '6px' }}>
                  <span style={{ fontWeight: '600', color: '#1e40af' }}>Input Type:</span>{' '}
                  <span style={{ color: '#3b82f6' }}>{selectedMethodInfo.input_type}</span>
                </div>

                {selectedMethodInfo.input_schema && selectedMethodInfo.input_schema.length > 0 && (
                  <div style={{
                    marginTop: '8px',
                    padding: '8px',
                    backgroundColor: '#fff',
                    borderRadius: '4px',
                    border: '1px solid #bfdbfe',
                    maxHeight: '200px',
                    overflow: 'auto'
                  }}>
                    <div style={{ fontWeight: '600', color: '#1e40af', marginBottom: '6px' }}>
                      Input Fields:
                    </div>
                    {selectedMethodInfo.input_schema.map((field, idx) => (
                      <div key={idx} style={{
                        padding: '4px 0',
                        borderBottom: idx < selectedMethodInfo.input_schema!.length - 1 ? '1px solid #e5e7eb' : 'none'
                      }}>
                        <span style={{ color: '#059669', fontWeight: '600' }}>{field.name}</span>
                        <span style={{ color: '#6b7280' }}> : </span>
                        <span style={{ color: '#3b82f6' }}>{field.type}</span>
                        {field.repeated && <span style={{ color: '#f59e0b' }}> []</span>}
                        {field.optional && <span style={{ color: '#9ca3af', fontSize: '10px' }}> (optional)</span>}
                      </div>
                    ))}
                  </div>
                )}

                <div style={{ marginTop: '8px', marginBottom: '6px' }}>
                  <span style={{ fontWeight: '600', color: '#1e40af' }}>Output Type:</span>{' '}
                  <span style={{ color: '#3b82f6' }}>{selectedMethodInfo.output_type}</span>
                </div>

                {selectedMethodInfo.output_schema && selectedMethodInfo.output_schema.length > 0 && (
                  <div style={{
                    marginTop: '8px',
                    padding: '8px',
                    backgroundColor: '#fff',
                    borderRadius: '4px',
                    border: '1px solid #bfdbfe',
                    maxHeight: '200px',
                    overflow: 'auto'
                  }}>
                    <div style={{ fontWeight: '600', color: '#1e40af', marginBottom: '6px' }}>
                      Output Fields:
                    </div>
                    {selectedMethodInfo.output_schema.map((field, idx) => (
                      <div key={idx} style={{
                        padding: '4px 0',
                        borderBottom: idx < selectedMethodInfo.output_schema!.length - 1 ? '1px solid #e5e7eb' : 'none'
                      }}>
                        <span style={{ color: '#059669', fontWeight: '600' }}>{field.name}</span>
                        <span style={{ color: '#6b7280' }}> : </span>
                        <span style={{ color: '#3b82f6' }}>{field.type}</span>
                        {field.repeated && <span style={{ color: '#f59e0b' }}> []</span>}
                        {field.optional && <span style={{ color: '#9ca3af', fontSize: '10px' }}> (optional)</span>}
                      </div>
                    ))}
                  </div>
                )}

                {!selectedMethodInfo.input_schema && (
                  <div style={{
                    marginTop: '8px',
                    paddingTop: '8px',
                    borderTop: '1px solid #bfdbfe',
                    fontSize: '10px',
                    color: '#6b7280'
                  }}>
                    💡 Tip: grpcurlを使用してスキーマを確認:<br/>
                    <code style={{
                      display: 'block',
                      marginTop: '4px',
                      padding: '4px',
                      backgroundColor: '#fff',
                      borderRadius: '2px',
                      wordBreak: 'break-all'
                    }}>
                      grpcurl -plaintext localhost:{'{port}'} describe {selectedMethodInfo.input_type.replace('.', '').replace(/^/, '')}
                    </code>
                  </div>
                )}
              </div>
            )}

            <div>
              <label style={{ display: 'block', fontSize: '12px', marginBottom: '4px', fontWeight: '500' }}>
                Request (JSON):
              </label>
              <textarea
                value={requestJson}
                onChange={(e) => setRequestJson(e.target.value)}
                placeholder="{}"
                rows={4}
                style={{
                  width: '100%',
                  padding: '8px',
                  fontSize: '12px',
                  fontFamily: 'monospace',
                  border: '1px solid #d1d5db',
                  borderRadius: '4px',
                  resize: 'vertical',
                  boxSizing: 'border-box'
                }}
              />
            </div>

            <button
              onClick={handleExecuteMethod}
              disabled={testing}
              style={{
                width: '100%',
                padding: '10px',
                backgroundColor: testing ? '#9ca3af' : '#8b5cf6',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: testing ? 'not-allowed' : 'pointer',
                fontSize: '14px',
                fontWeight: '500'
              }}
            >
              {testing ? 'Executing...' : '🚀 Execute Method'}
            </button>
          </div>

          {testResult && (
            <div style={{
              marginTop: '12px',
              padding: '10px',
              backgroundColor: testResult.startsWith('✅') ? '#dcfce7' : '#fee2e2',
              borderRadius: '4px',
              fontSize: '12px',
              fontFamily: 'monospace',
              color: testResult.startsWith('✅') ? '#166534' : '#991b1b',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              maxHeight: '300px',
              overflow: 'auto'
            }}>
              {testResult}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
