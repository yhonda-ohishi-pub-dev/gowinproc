import { FC, useState, useEffect } from 'react'
import { processApi } from '../api/client'
import { ProcessInfo, Metrics } from '../types'
import MetricsChart from './MetricsChart'
import InstanceList from './InstanceList'
import '../styles/ProcessDetail.css'

interface ProcessDetailProps {
  processName: string
}

const ProcessDetail: FC<ProcessDetailProps> = ({ processName }) => {
  const [processInfo, setProcessInfo] = useState<ProcessInfo | null>(null)
  const [metrics, setMetrics] = useState<Metrics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [targetInstances, setTargetInstances] = useState<number>(1)

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 3000)
    return () => clearInterval(interval)
  }, [processName])

  const fetchData = async () => {
    try {
      const info = await processApi.getProcess(processName)
      setProcessInfo(info)

      // Try to get metrics, but don't fail if not available
      try {
        const metricsData = await processApi.getMetrics(processName)
        setMetrics(metricsData)
      } catch (metricsErr) {
        // Metrics endpoint not implemented yet, skip
        console.log('Metrics not available:', metricsErr)
      }

      setTargetInstances(info.count || (info.instances?.length ?? 1))
      setLoading(false)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
      setLoading(false)
    }
  }

  const handleStart = async () => {
    try {
      await processApi.startProcess(processName)
      await fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to start process')
    }
  }

  const handleStop = async () => {
    try {
      await processApi.stopProcess(processName)
      await fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to stop process')
    }
  }

  const handleRestart = async () => {
    try {
      await processApi.restartProcess(processName)
      await fetchData()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to restart process')
    }
  }

  const handleScale = async () => {
    try {
      await processApi.scaleProcess(processName, targetInstances)
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
          <button onClick={handleRestart} className="btn btn-warning">
            Restart
          </button>
        </div>
      </header>

      {processInfo.config && (
        <section className="config-section">
          <h3>Configuration</h3>
          <div className="config-grid">
            <div className="config-item">
              <label>Binary Path:</label>
              <span>{processInfo.config.binary_path}</span>
            </div>
            <div className="config-item">
              <label>Work Directory:</label>
              <span>{processInfo.config.work_dir}</span>
            </div>
            <div className="config-item">
              <label>Port:</label>
              <span>{processInfo.config.port}</span>
            </div>
            <div className="config-item">
              <label>Auto Restart:</label>
              <span>{processInfo.config.auto_restart ? 'Yes' : 'No'}</span>
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
                min={processInfo.config.min_instances || 1}
                max={processInfo.config.max_instances || 10}
                value={targetInstances}
                onChange={(e) => setTargetInstances(parseInt(e.target.value))}
              />
            </label>
            <button onClick={handleScale} className="btn btn-primary">
              Scale
            </button>
            <span className="instance-count">
              Current: {processInfo.count || processInfo.instances?.length || 0} instances
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
