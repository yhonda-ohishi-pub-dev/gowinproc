import { useState, useEffect } from 'react'
import { grpcProcessApi } from '../api/grpc-client'
import '../styles/RepositoryList.css'

function RepositoryList() {
  const [repositories, setRepositories] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchRepositories()
    const interval = setInterval(fetchRepositories, 30000) // Refresh every 30 seconds
    return () => clearInterval(interval)
  }, [])

  const fetchRepositories = async () => {
    try {
      setLoading(true)
      const data = await grpcProcessApi.listRepositories()
      setRepositories(data.repositories || [])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch repositories')
    } finally {
      setLoading(false)
    }
  }

  if (loading && repositories.length === 0) {
    return <div className="loading">Loading repositories...</div>
  }

  if (error) {
    return (
      <div className="error-container">
        <div className="error">Error: {error}</div>
        <button onClick={fetchRepositories} className="retry-button">
          Retry
        </button>
      </div>
    )
  }

  return (
    <div className="repository-list">
      <div className="list-header">
        <h2>Monitored Repositories</h2>
        <button onClick={fetchRepositories} className="refresh-button" disabled={loading}>
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {repositories.length === 0 ? (
        <div className="empty-state">
          <p>No repositories configured</p>
          <p className="help-text">
            Configure repositories in your config file to enable automatic updates
          </p>
        </div>
      ) : (
        <ul className="repositories">
          {repositories.map((repo, index) => (
            <li key={index} className="repository-item">
              <div className="repo-icon">📦</div>
              <div className="repo-info">
                <div className="repo-name">{repo}</div>
                <div className="repo-meta">
                  GitHub Repository
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      <div className="repository-count">
        Total: {repositories.length} {repositories.length === 1 ? 'repository' : 'repositories'}
      </div>
    </div>
  )
}

export default RepositoryList
