import { FC, useState, useEffect } from 'react'
import { processApi } from '../api/client'
import { UpdateAvailable, VersionInfo } from '../types'
import '../styles/UpdateManager.css'

interface UpdateManagerProps {
  processes: string[]
}

const UpdateManager: FC<UpdateManagerProps> = ({ processes }) => {
  const [updates, setUpdates] = useState<UpdateAvailable[]>([])
  const [versions, setVersions] = useState<Map<string, VersionInfo>>(new Map())
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
        processApi.listAvailableUpdates(),
        ...processes.map((p) => processApi.getVersion(p)),
      ])

      setUpdates(updatesData.updates || [])

      const versionMap = new Map<string, VersionInfo>()
      versionData.forEach((v) => {
        versionMap.set(v.process_name, v)
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
      const response = await processApi.updateProcess(processName, {
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
      const response = await processApi.rollback(processName)
      alert(`Rollback completed: ${response.from_version} → ${response.to_version}`)
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
      const response = await processApi.updateAll({ strategy: 'rolling' })
      alert(`Update all started: ${response.message}`)
      await fetchUpdates()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Update all failed')
    }
  }

  if (loading) return <div className="loading">Loading update information...</div>

  const hasUpdates = updates.some((u) => !u.up_to_date)

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
            const version = versions.get(update.process_name)
            const isUpdating = updating.has(update.process_name)

            return (
              <div
                key={update.process_name}
                className={`update-card ${update.up_to_date ? 'up-to-date' : 'outdated'}`}
              >
                <div className="update-header">
                  <h3>{update.process_name}</h3>
                  {update.up_to_date ? (
                    <span className="badge badge-success">Up to date</span>
                  ) : (
                    <span className="badge badge-warning">Update available</span>
                  )}
                </div>

                <div className="version-info">
                  <div className="version-item">
                    <label>Current Version:</label>
                    <span className="version">{update.current_version}</span>
                  </div>
                  {!update.up_to_date && (
                    <div className="version-item">
                      <label>Latest Version:</label>
                      <span className="version latest">{update.latest_version}</span>
                    </div>
                  )}
                </div>

                {version && version.instances.length > 0 && (
                  <div className="instance-versions">
                    <label>Instance Versions:</label>
                    <div className="instances">
                      {version.instances.map((inst) => (
                        <span key={inst.id} className="instance-version">
                          {inst.id.substring(0, 8)}: v{inst.version}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {!update.up_to_date && update.release_notes && (
                  <div className="release-notes">
                    <label>Release Notes:</label>
                    <p>{update.release_notes}</p>
                    {update.release_date && (
                      <small>Released: {update.release_date}</small>
                    )}
                  </div>
                )}

                <div className="update-actions">
                  {!update.up_to_date && (
                    <button
                      onClick={() => handleUpdate(update.process_name)}
                      disabled={isUpdating}
                      className="btn btn-primary"
                    >
                      {isUpdating ? 'Updating...' : 'Update to Latest'}
                    </button>
                  )}
                  <button
                    onClick={() => handleRollback(update.process_name)}
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
