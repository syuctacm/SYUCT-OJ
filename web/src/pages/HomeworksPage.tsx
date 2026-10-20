import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getHomeworks, type Contest } from '../api.ts'

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false })
}

export function HomeworksPage() {
  const [items, setItems] = useState<Contest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getHomeworks()
      .then(setItems)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <section className="section">
      <div className="section__header">
        <div>
          <p className="eyebrow">HOMEWORK</p>
          <h1 className="section__title">作业</h1>
        </div>
      </div>
      {loading ? <div className="empty-state">加载中...</div> : null}
      {error ? <div className="empty-state error-state">{error}</div> : null}
      {!loading && !error ? (
        <div className="section__table-container">
          <table className="data-table">
            <thead><tr><th>作业</th><th>开始时间</th><th>截止时间</th><th className="center">题目数</th></tr></thead>
            <tbody>
              {items.map((homework) => (
                <tr key={homework.id}>
                  <td><Link className="problem-link" to={`/homework/${homework.docId}`}>{homework.title}</Link></td>
                  <td>{formatTime(homework.beginAt)}</td>
                  <td>{formatTime(homework.endAt)}</td>
                  <td className="center mono">{homework.pids?.length ?? 0}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {items.length === 0 ? <div className="empty-state">暂无作业</div> : null}
        </div>
      ) : null}
      <div className="section__footer">共 {items.length} 次作业</div>
    </section>
  )
}
