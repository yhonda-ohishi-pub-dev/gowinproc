import { FC, useState, useEffect } from 'react'
import { grpcProcessApi } from '../api/grpc-client'
import type * as pb from '../proto/process_manager'
import MetricsChart from './MetricsChart'
import InstanceList from './InstanceList'
import '../styles/ProcessDetail.css'

interface ProcessDetailProps {
  processName: string
}

const ProcessDetail: FC<ProcessDetailProps> = ({ processName }) => {
  const [processInfo, setProcessInfo] = useState<pb.ProcessInfo | null>(null)
  const [metrics, setMetrics] = useState<pb.Metrics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [targetInstances, setTargetInstances] = useState<number>(1)
  const [isInitialized, setIsInitialized] = useState(false)
  const [isRestarting, setIsRestarting] = useState(false)

  useEffect(() => {
    // Reset initialization state when processName changes
    setIsInitialized(false)
    setLoading(true)
    fetchData()
    const interval = setInterval(() => {
      // Continue polling even during restart to show transition state (old + new instances)
      // Polling interval: 500ms to catch the transition state during hot restart
      fetchData()
    }, 500)
    return () => clearInterval(interval)
  }, [processName])

  const fetchData = async () => {
    try {
      const info = await grpcProcessApi.getProcess(processName)
      setProcessInfo(info)

      // Try to get metrics, but don't fail if not available
      try {
        const metricsData = await grpcProcessApi.getMetrics(processName)
        setMetrics(metricsData)
      } catch (metricsErr) {
        // Metrics endpoint not implemented yet, skip
        console.log('Metrics not available:', metricsErr)
      }

      // Only update targetInstances on initial load, not during polling
      if (!isInitialized) {
        setTargetInstances(info.instanceCount || (info.instances?.length ?? 1))
        setIsInitialized(true)
      }

      setLoading(false)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
      setLoading(false)
    }
  }

  const handleStart = async () => {
    try {
      await grpcProcessApi.startProcess(processName)
      await fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to start process')
    }
  }

  const handleStop = async () => {
    try {
      await grpcProcessApi.stopProcess(processName)
      await fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to stop process')
    }
  }

  const handleRestart = async () => {
    setIsRestarting(true)
    try {
      // Restart process (backend will perform health checks automatically)
      await grpcProcessApi.restartProcess(processName)
      await fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to restart process')
    } finally {
      setIsRestarting(false)
    }
  }

  const handleScale = async () => {
    try {
      await grpcProcessApi.scaleProcess(processName, targetInstances)
      // Allow targetInstances to be updated after scaling
      setIsInitialized(false)
      await fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to scale process')
    }
  }

  if (loading) return <div className="loading">Loading process details...</div>
  if (error) return <div className="error">Error: {error}</div>
  if (!processInfo) return <div className="error">Process not found</div>

  return (
    <div className="process-detail">
      <header className="detail-header">
        <h2>{processInfo.name}</h2>
        <div className="actions">
          <button onClick={handleStart} className="btn btn-success">
            Start
          </button>
          <button onClick={handleStop} className="btn btn-danger">
            Stop
          </button>
          <button onClick={handleRestart} className="btn btn-warning" disabled={isRestarting}>
            {isRestarting ? 'Restarting... (health check in progress)' : 'Restart'}
          </button>
        </div>
      </header>

      {processInfo.config && (
        <section className="config-section">
          <h3>Configuration</h3>
          <div className="config-grid">
            <div className="config-item">
              <label>Binary Path:</label>
              <span>{processInfo.config.binaryPath}</span>
            </div>
            <div className="config-item">
              <label>Work Directory:</label>
              <span>{processInfo.config.workDir}</span>
            </div>
            <div className="config-item">
              <label>Port:</label>
              <span>{processInfo.config.port}</span>
            </div>
            <div className="config-item">
              <label>Auto Restart:</label>
              <span>{processInfo.config.autoRestart ? 'Yes' : 'No'}</span>
            </div>
            {processInfo.config.github && (
              <div className="config-item">
                <label>GitHub Repo:</label>
                <span>{processInfo.config.github.repo}</span>
              </div>
            )}
          </div>
        </section>
      )}

      {processInfo.config && (
        <section className="scaling-section">
          <h3>Scaling</h3>
          <div className="scaling-controls">
            <label>
              Target Instances:
              <input
                type="number"
                min={processInfo.config.minInstances || 1}
                max={processInfo.config.maxInstances || 10}
                value={targetInstances}
                onChange={(e) => setTargetInstances(parseInt(e.target.value))}
              />
            </label>
            <button onClick={handleScale} className="btn btn-primary">
              Scale
            </button>
            <span className="instance-count">
              Current: {processInfo.instanceCount || processInfo.instances?.length || 0} instances
            </span>
          </div>
        </section>
      )}

      <section className="instances-section">
        <h3>Instances</h3>
        <InstanceList
          instances={processInfo.instances}
          processName={processName}
          onRefresh={fetchData}
        />
      </section>

      {metrics && (
        <section className="metrics-section">
          <h3>Metrics</h3>
          <MetricsChart metrics={metrics} />
        </section>
      )}
    </div>
  )
}

export default ProcessDetail
