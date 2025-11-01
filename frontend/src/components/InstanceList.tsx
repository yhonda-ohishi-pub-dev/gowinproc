import { FC } from 'react'
import type * as pb from '../proto/process_manager'
import { grpcProcessApi } from '../api/grpc-client'
import '../styles/InstanceList.css'

interface InstanceListProps {
  instances: pb.ProcessInstance[]
  processName: string
  onRefresh: () => void
}

const InstanceList: FC<InstanceListProps> = ({
  instances,
  processName,
  onRefresh,
}) => {
  const handleStopInstance = async (instanceId: string) => {
    try {
      await grpcProcessApi.stopProcess(processName, instanceId)
      onRefresh()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to stop instance')
    }
  }

  const handleRestartInstance = async (instanceId: string) => {
    try {
      await grpcProcessApi.restartProcess(processName, instanceId)
      onRefresh()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to restart instance')
    }
  }

  const formatUptime = (startTime: number): string => {
    const uptimeSeconds = Math.floor((Date.now() - startTime * 1000) / 1000)
    const hours = Math.floor(uptimeSeconds / 3600)
    const minutes = Math.floor((uptimeSeconds % 3600) / 60)
    const seconds = uptimeSeconds % 60
    return `${hours}h ${minutes}m ${seconds}s`
  }

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
  }

  return (
    <div className="instance-list">
      {instances.length === 0 ? (
        <p className="empty-message">No instances running</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>PID</th>
              <th>Status</th>
              <th>Port</th>
              <th>Uptime</th>
              <th>CPU</th>
              <th>Memory</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {instances.map((instance) => (
              <tr key={instance.id}>
                <td className="id-cell">{instance.id.substring(0, 8)}</td>
                <td>{instance.pid}</td>
                <td>
                  <span className={`status status-${instance.status.toLowerCase()}`}>
                    {instance.status}
                  </span>
                </td>
                <td>{instance.port}</td>
                <td>{formatUptime(instance.startTime)}</td>
                <td>
                  {instance.metrics
                    ? `${instance.metrics.cpuUsage.toFixed(1)}%`
                    : 'N/A'}
                </td>
                <td>
                  {instance.metrics
                    ? formatBytes(Number(instance.metrics.memoryUsage))
                    : 'N/A'}
                </td>
                <td className="actions-cell">
                  <button
                    onClick={() => handleRestartInstance(instance.id)}
                    className="btn-small btn-warning"
                    title="Restart"
                  >
                    ↻
                  </button>
                  <button
                    onClick={() => handleStopInstance(instance.id)}
                    className="btn-small btn-danger"
                    title="Stop"
                  >
                    ■
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

export default InstanceList