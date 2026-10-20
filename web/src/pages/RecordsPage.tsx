import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getSubmissions, type SubmissionPage } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { statusLabel } from '../status.ts'

export function RecordsPage() {
  const { user, loading: authLoading } = useAuth()
  const [page, setPage] = useState<SubmissionPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (authLoading) return
    if (!user) {
      setLoading(false)
      return
    }
    setLoading(true)
    getSubmissions()
      .then(setPage)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '提交记录加载失败'))
      .finally(() => setLoading(false))
  }, [authLoading, user])

  if (authLoading) return <section className="panel"><div className="empty-state">加载中...</div></section>
  if (!user) return <section className="panel"><div className="empty-state">请先 <Link to="/login">登录</Link> 查看提交记录。</div></section>
  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">MY SUBMISSIONS</p>
          <h1 className="panel-title">提交记录</h1>
        </div>
        <Link className="button" to="/p">去题库</Link>
      </div>
      {loading ? <div className="empty-state">加载中...</div> : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th className="col-id">提交</th>
                <th>题目</th>
                <th>状态</th>
                <th>分数</th>
                <th>语言</th>
                <th className="col-stat">时间 / 内存</th>
              </tr>
            </thead>
            <tbody>
              {page?.items.map((submission) => (
                <tr key={submission.id}>
                  <td><Link className="mono" to={`/record/${submission.id}`}>{submission.id.slice(-8)}</Link></td>
                  <td className="mono">#{submission.pid}</td>
                  <td><span className={`record-status record-status-${submission.status}`}>{statusLabel(submission.status)}</span></td>
                  <td className="mono">{submission.score}</td>
                  <td>{submission.lang}</td>
                  <td className="mono">{submission.time} ms / {submission.memory} KiB</td>
                </tr>
              ))}
            </tbody>
          </table>
          {page?.items.length === 0 ? <div className="empty-state">还没有提交记录</div> : null}
        </div>
      )}
      <div className="panel-footer">共 {page?.total ?? 0} 条提交</div>
    </section>
  )
}
