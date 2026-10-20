import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getTrainings, type Training } from '../api.ts'

export function TrainingsPage() {
  const [items, setItems] = useState<Training[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getTrainings()
      .then(setItems)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <section className="section">
      <div className="section__header">
        <div>
          <p className="eyebrow">TRAINING</p>
          <h1 className="section__title">训练</h1>
        </div>
      </div>
      {loading ? <div className="empty-state">加载中...</div> : null}
      {error ? <div className="empty-state error-state">{error}</div> : null}
      {!loading && !error ? (
        <div className="section__table-container">
          <table className="data-table">
            <thead><tr><th>训练计划</th><th>简介</th><th>章节 / 题目</th><th>报名</th><th>状态</th></tr></thead>
            <tbody>
              {items.map((training) => (
                <tr key={training.id}>
                  <td><Link className="problem-link" to={`/training/${training.docId}`}>{training.title}</Link></td>
                  <td>{training.description || '-'}</td>
                  <td className="mono">{training.sectionCount} / {training.problemCount}</td>
                  <td className="mono">{training.attend}</td>
                  <td>{training.pin ? <span className="tag">置顶</span> : '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {items.length === 0 ? <div className="empty-state">暂无训练计划</div> : null}
        </div>
      ) : null}
      <div className="section__footer">共 {items.length} 个训练计划</div>
    </section>
  )
}
