import { FC, useState, useEffect } from 'react'
import { grpcProcessApi } from '../api/grpc-client'
import type * as pb from '../proto/process_manager'
import '../styles/UpdateManager.css'

interface UpdateManagerProps {
  processes: string[]
}

const UpdateManager: FC<UpdateManagerProps> = ({ processes }) => {
  const [updates, setUpdates] = useState<pb.UpdateAvailable[]>([])
  const [versions, setVersions] = useState<Map<string, pb.VersionInfo>>(new Map())
  const [loading, setLoading] = useState(true)
  const [updating, setUpdating] = useState<Set<string>>(new Set())

  useEffect(() => {
    fetchUpdates()
    const interval = setInterval(fetchUpdates, 30000) // 30秒ごと
    return () => clearInterval(interval)
  }, [processes])

  const fetchUpdates = async () => {
    try {
      const [updatesData, ...versionData] = await Promise.all([
        grpcProcessApi.listAvailableUpdates(),
        ...processes.map((p) => grpcProcessApi.getVersion(p)),
      ])

      setUpdates(updatesData.updates || [])

      const versionMap = new Map<string, pb.VersionInfo>()
      versionData.forEach((v: pb.VersionInfo) => {
        versionMap.set(v.processName, v)
      })
      setVersions(versionMap)

      setLoading(false)
    } catch (err) {
      console.error('Failed to fetch updates:', err)
      setLoading(false)
    }
  }

  const handleUpdate = async (processName: string, version?: string) => {
    setUpdating((prev) => new Set(prev).add(processName))
    try {
      const response = await grpcProcessApi.updateProcess(processName, {
        version,
        strategy: 'rolling',
      })
      alert(`Update started: ${response.message}`)
      await fetchUpdates()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Update failed')
    } finally {
      setUpdating((prev) => {
        const next = new Set(prev)
        next.delete(processName)
        return next
      })
    }
  }

  const handleRollback = async (processName: string) => {
    if (!confirm(`Rollback ${processName} to previous version?`)) return

    setUpdating((prev) => new Set(prev).add(processName))
    try {
      const response = await grpcProcessApi.rollback(processName)
      alert(`Rollback completed: ${response.fromVersion} → ${response.toVersion}`)
      await fetchUpdates()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Rollback failed')
    } finally {
      setUpdating((prev) => {
        const next = new Set(prev)
        next.delete(processName)
        return next
      })
    }
  }

  const handleUpdateAll = async () => {
    if (!confirm('Update all processes with available updates?')) return

    try {
      const response = await grpcProcessApi.updateAll({ strategy: 'rolling' })
      alert(`Update all started: ${response.message}`)
      await fetchUpdates()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Update all failed')
    }
  }

  if (loading) return <div className="loading">Loading update information...</div>

  const hasUpdates = updates.some((u) => !u.upToDate)

  return (
    <div className="update-manager">
      <header className="update-header">
        <h2>Update Management</h2>
        <button
          onClick={handleUpdateAll}
          disabled={!hasUpdates}
          className="btn btn-primary"
        >
          Update All
        </button>
      </header>

      <div className="updates-list">
        {updates.length === 0 ? (
          <p className="empty-message">No update information available</p>
        ) : (
          updates.map((update) => {
            const version = versions.get(update.processName)
            const isUpdating = updating.has(update.processName)

            return (
              <div
                key={update.processName}
                className={`update-card ${update.upToDate ? 'up-to-date' : 'outdated'}`}
              >
                <div className="update-header">
                  <h3>{update.processName}</h3>
                  {update.upToDate ? (
                    <span className="badge badge-success">Up to date</span>
                  ) : (
                    <span className="badge badge-warning">Update available</span>
                  )}
                </div>

                <div className="version-info">
                  <div className="version-item">
                    <label>Current Version:</label>
                    <span className="version">{update.currentVersion}</span>
                  </div>
                  {!update.upToDate && (
                    <div className="version-item">
                      <label>Latest Version:</label>
                      <span className="version latest">{update.latestVersion}</span>
                    </div>
                  )}
                </div>

                {version && version.instances.length > 0 && (
                  <div className="instance-versions">
                    <label>Instance Versions:</label>
                    <div className="instances">
                      {version.instances.map((inst: pb.InstanceVersion) => (
                        <span key={inst.id} className="instance-version">
                          {inst.id.substring(0, 8)}: v{inst.version}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {!update.upToDate && update.releaseNotes && (
                  <div className="release-notes">
                    <label>Release Notes:</label>
                    <p>{update.releaseNotes}</p>
                    {update.releaseDate && (
                      <small>Released: {update.releaseDate}</small>
                    )}
                  </div>
                )}

                <div className="update-actions">
                  {!update.upToDate && (
                    <button
                      onClick={() => handleUpdate(update.processName)}
                      disabled={isUpdating}
                      className="btn btn-primary"
                    >
                      {isUpdating ? 'Updating...' : 'Update to Latest'}
                    </button>
                  )}
                  <button
                    onClick={() => handleRollback(update.processName)}
                    disabled={isUpdating}
                    className="btn btn-secondary"
                  >
                    {isUpdating ? 'Processing...' : 'Rollback'}
                  </button>
                </div>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}

export default UpdateManager