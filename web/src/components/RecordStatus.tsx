import { Link } from 'react-router-dom'
import { statusClass, statusLabel } from '../status.ts'

type RecordStatusProps = {
  status: number
  rid: string
  score?: number
}

export function RecordStatusCell({ status, rid, score }: RecordStatusProps) {
  const className = statusClass(status)
  return (
    <td className={`col--status record-status--border ${className}`}>
      <div className="col--status__text">
        <span className={`icon record-status--icon ${className}`} />
        <Link className={`record-status--text ${className}`} to={`/record/${rid}`}>
          {typeof score === 'number' ? <span>{score} </span> : null}
          {statusLabel(status)}
        </Link>
      </div>
    </td>
  )
}
