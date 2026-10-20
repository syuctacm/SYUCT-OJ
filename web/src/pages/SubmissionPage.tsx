import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getSubmission, type Submission } from '../api.ts'
import { statusLabel } from '../status.ts'

export function SubmissionPage() {
  const { rid = '' } = useParams()
  const [submission, setSubmission] = useState<Submission | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    let timer: number | undefined
    const load = async () => {
      try {
        const next = await getSubmission(rid)
        if (cancelled) return
        setSubmission(next)
        setError('')
        if ([0, 20, 21, 22].includes(next.status)) {
          timer = window.setTimeout(load, 1000)
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : '提交加载失败')
      }
    }
    load()
    return () => {
      cancelled = true
      if (timer) window.clearTimeout(timer)
    }
  }, [rid])

  if (error) return <section className="section"><div className="empty-state error-state">{error}</div></section>
  if (!submission) return <section className="section"><div className="empty-state">加载中...</div></section>

  return (
    <section className="section">
      <div className="section__header">
        <div>
          <p className="eyebrow">SUBMISSION</p>
          <h1 className="section__title">提交结果</h1>
        </div>
        <Link className="rounded button" to="/p">返回题库</Link>
      </div>
      <div className="submission-summary">
        <div><span className="summary-label">状态</span><strong className={`status status-${submission.status}`}>{statusLabel(submission.status)}</strong></div>
        <div><span className="summary-label">分数</span><strong>{submission.score}</strong></div>
        <div><span className="summary-label">时间</span><strong>{submission.time} ms</strong></div>
        <div><span className="summary-label">内存</span><strong>{submission.memory} KiB</strong></div>
      </div>
      {submission.compilerTexts?.length ? <div className="section__body compiler-output"><pre>{submission.compilerTexts.join('\n')}</pre></div> : null}
      {submission.code ? <div className="section__body code-view"><pre>{submission.code}</pre></div> : null}
    </section>
  )
}
