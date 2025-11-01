import { FC } from 'react'
import type * as pb from '../proto/process_manager'
import '../styles/MetricsChart.css'

interface MetricsChartProps {
  metrics: pb.Metrics
}

const MetricsChart: FC<MetricsChartProps> = ({ metrics }) => {
  const formatBytes = (bytes: number | bigint): string => {
    const numBytes = typeof bytes === 'bigint' ? Number(bytes) : bytes
    if (numBytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(numBytes) / Math.log(k))
    return Math.round(numBytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
  }

  if (!metrics.aggregated) {
    return <div className="no-metrics">No metrics available</div>
  }

  return (
    <div className="metrics-chart">
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-label">Total CPU Usage</div>
          <div className="metric-value">
            {metrics.aggregated.totalCpuUsage.toFixed(1)}%
          </div>
          <div className="metric-bar">
            <div
              className="metric-bar-fill cpu"
              style={{ width: `${Math.min(metrics.aggregated.totalCpuUsage, 100)}%` }}
            />
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-label">Total Memory Usage</div>
          <div className="metric-value">
            {formatBytes(metrics.aggregated.totalMemoryUsage)}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-label">Active Instances</div>
          <div className="metric-value">{metrics.aggregated.instanceCount}</div>
        </div>

        <div className="metric-card">
          <div className="metric-label">Disk Read</div>
          <div className="metric-value">
            {formatBytes(metrics.aggregated.totalDiskRead)}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-label">Disk Write</div>
          <div className="metric-value">
            {formatBytes(metrics.aggregated.totalDiskWrite)}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-label">Network Received</div>
          <div className="metric-value">
            {formatBytes(metrics.aggregated.totalNetworkRecv)}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-label">Network Sent</div>
          <div className="metric-value">
            {formatBytes(metrics.aggregated.totalNetworkSent)}
          </div>
        </div>
      </div>

      <div className="instance-metrics">
        <h4>Per-Instance Metrics</h4>
        <table>
          <thead>
            <tr>
              <th>Instance</th>
              <th>CPU</th>
              <th>Memory</th>
              <th>Uptime</th>
            </tr>
          </thead>
          <tbody>
            {metrics.instances.map((instance) => (
              <tr key={instance.instanceId}>
                <td>{instance.instanceId.substring(0, 8)}</td>
                <td>{instance.cpuUsage.toFixed(1)}%</td>
                <td>{formatBytes(instance.memoryUsage)}</td>
                <td>{Math.floor(Number(instance.uptime) / 60)}m</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default MetricsChart