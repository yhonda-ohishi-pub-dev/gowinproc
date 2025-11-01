import { useState, useEffect } from 'react'
import ProcessList from './components/ProcessList'
import ProcessDetail from './components/ProcessDetail'
import UpdateManager from './components/UpdateManager'
import RepositoryList from './components/RepositoryList'
import { ProcessInfo } from './types'
import { grpcProcessApi } from './api/grpc-client'
import './styles/App.css'

interface ServerStatus {
  status: string
  processes: number
  total_instances: number
  running_instances: number
  total_cpu?: number
  total_memory?: number
}

function App() {
  const [processes, setProcesses] = useState<string[]>([])
  const [selectedProcess, setSelectedProcess] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<'processes' | 'updates' | 'repositories'>('processes')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null)

  useEffect(() => {
    fetchProcesses()
    fetchStatus()
    const processInterval = setInterval(fetchProcesses, 5000)
    const statusInterval = setInterval(fetchStatus, 3000)
    return () => {
      clearInterval(processInterval)
      clearInterval(statusInterval)
    }
  }, [])

  const fetchProcesses = async () => {
    try {
      const data = await grpcProcessApi.listProcesses()
      const sortedProcesses = (data.processNames || []).sort((a: string, b: string) =>
        a.localeCompare(b)
      )
      setProcesses(sortedProcesses)
      setLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
      setLoading(false)
    }
  }

  const fetchStatus = async () => {
    try {
      // gRPCにはサーバー統計エンドポイントがないので、プロセス一覧から計算
      const data = await grpcProcessApi.listProcesses()
      let totalInstances = 0
      let runningInstances = 0
      let totalCpu = 0
      let totalMemory = 0

      for (const processName of data.processNames || []) {
        try {
          const processInfo = await grpcProcessApi.getProcess(processName)
          totalInstances += processInfo.instances?.length || 0
          runningInstances += processInfo.instances?.filter(i => i.status === 'running').length || 0

          // Aggregate CPU and memory from all instances
          for (const instance of processInfo.instances || []) {
            if (instance.metrics) {
              totalCpu += instance.metrics.cpuUsage || 0
              totalMemory += Number(instance.metrics.memoryUsage || 0)
            }
          }
        } catch (err) {
          console.error(`Failed to get process ${processName}:`, err)
        }
      }

      setServerStatus({
        status: 'running',
        processes: data.count || data.processNames?.length || 0,
        total_instances: totalInstances,
        running_instances: runningInstances,
        total_cpu: totalCpu,
        total_memory: totalMemory,
      })
    } catch (err) {
      console.error('Failed to fetch server status:', err)
    }
  }

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-top">
          <h1>GoWinProc - Process Manager</h1>
          {serverStatus && (
            <div className="server-status">
              <span className="status-indicator" title={`Server: ${serverStatus.status}`}>
                ●
              </span>
              <span className="status-text">
                {serverStatus.processes} Processes | {serverStatus.running_instances}/{serverStatus.total_instances} Running
                {serverStatus.total_cpu !== undefined && (
                  <> | CPU: {serverStatus.total_cpu.toFixed(1)}%</>
                )}
                {serverStatus.total_memory !== undefined && (
                  <> | Memory: {(serverStatus.total_memory / 1024 / 1024).toFixed(0)} MB</>
                )}
              </span>
            </div>
          )}
        </div>
        <nav className="tabs">
          <button
            className={activeTab === 'processes' ? 'active' : ''}
            onClick={() => setActiveTab('processes')}
          >
            Processes
          </button>
          <button
            className={activeTab === 'updates' ? 'active' : ''}
            onClick={() => setActiveTab('updates')}
          >
            Updates
          </button>
          <button
            className={activeTab === 'repositories' ? 'active' : ''}
            onClick={() => setActiveTab('repositories')}
          >
            Repositories
          </button>
        </nav>
      </header>

      <main className="app-main">
        {loading ? (
          <div className="loading">Loading...</div>
        ) : error ? (
          <div className="error">Error: {error}</div>
        ) : (
          <div className="content">
            {activeTab === 'processes' ? (
              <div className="process-view">
                <aside className="sidebar">
                  <ProcessList
                    processes={processes}
                    selectedProcess={selectedProcess}
                    onSelectProcess={setSelectedProcess}
                  />
                </aside>
                <section className="main-content">
                  {selectedProcess ? (
                    <ProcessDetail processName={selectedProcess} />
                  ) : (
                    <div className="placeholder">
                      Select a process to view details
                    </div>
                  )}
                </section>
              </div>
            ) : activeTab === 'updates' ? (
              <UpdateManager processes={processes} />
            ) : (
              <RepositoryList />
            )}
          </div>
        )}
      </main>
    </div>
  )
}

export default App
