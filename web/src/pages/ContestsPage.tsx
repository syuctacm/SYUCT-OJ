import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getContests, type Contest } from '../api.ts'

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function ContestsPage() {
  const [items, setItems] = useState<Contest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getContests()
      .then(setItems)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">CONTESTS</p>
          <h1 className="panel-title">比赛</h1>
        </div>
      </div>
      {loading ? <div className="empty-state">加载中...</div> : null}
      {error ? <div className="empty-state error-state">{error}</div> : null}
      {!loading && !error ? (
        <div className="table-wrap">
          <table className="data-table">
            <thead><tr><th>比赛</th><th>赛制</th><th>开始时间</th><th>结束时间</th><th className="center">参赛人数</th></tr></thead>
            <tbody>
              {items.map((contest) => (
                <tr key={contest.id}>
                  <td><Link className="problem-link" to={`/contest/${contest.docId}`}>{contest.title}</Link></td>
                  <td className="mono">{contest.rule || '-'}</td>
                  <td>{formatTime(contest.beginAt)}</td>
                  <td>{formatTime(contest.endAt)}</td>
                  <td className="center mono">{contest.attend}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {items.length === 0 ? <div className="empty-state">暂无比赛</div> : null}
        </div>
      ) : null}
      <div className="panel-footer">共 {items.length} 场比赛</div>
    </section>
  )
}
