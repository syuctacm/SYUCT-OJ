import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getDiscussions, type Discussion } from '../api.ts'

export function DiscussionsPage() {
  const [items, setItems] = useState<Discussion[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getDiscussions()
      .then(setItems)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <section className="section">
      <div className="section__header">
        <div>
          <p className="eyebrow">DISCUSSIONS</p>
          <h1 className="section__title">讨论</h1>
        </div>
      </div>
      {loading ? <div className="empty-state">加载中...</div> : null}
      {error ? <div className="empty-state error-state">{error}</div> : null}
      {!loading && !error ? (
        <div className="section__table-container">
          <table className="data-table">
            <thead><tr><th>标题</th><th>UID</th><th>回复</th><th>浏览</th><th>更新时间</th></tr></thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td><Link className="problem-link" to={`/discuss/${item.docId}`}>{item.pin ? '📌 ' : ''}{item.title}</Link></td>
                  <td className="mono">{item.owner}</td>
                  <td className="mono">{item.nReply}</td>
                  <td className="mono">{item.views}</td>
                  <td>{new Date(item.updateAt).toLocaleString('zh-CN', { hour12: false })}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {items.length === 0 ? <div className="empty-state">暂无讨论</div> : null}
        </div>
      ) : null}
      <div className="section__footer">共 {items.length} 个讨论</div>
    </section>
  )
}
