import { FC } from 'react'
import '../styles/ProcessList.css'

interface ProcessListProps {
  processes: string[]
  selectedProcess: string | null
  onSelectProcess: (process: string) => void
}

const ProcessList: FC<ProcessListProps> = ({
  processes,
  selectedProcess,
  onSelectProcess,
}) => {
  return (
    <div className="process-list">
      <h2>Processes ({processes.length})</h2>
      <ul>
        {processes.map((process) => (
          <li
            key={process}
            className={selectedProcess === process ? 'active' : ''}
            onClick={() => onSelectProcess(process)}
          >
            <span className="process-name">{process}</span>
          </li>
        ))}
      </ul>
      {processes.length === 0 && (
        <p className="empty-message">No processes found</p>
      )}
    </div>
  )
}

export default ProcessList
