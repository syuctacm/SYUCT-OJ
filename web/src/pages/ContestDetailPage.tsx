import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import { getContest, type Contest } from '../api.ts'

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false })
}

export function ContestDetailPage() {
  const { tid = '' } = useParams()
  const [contest, setContest] = useState<Contest | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getContest(tid).then(setContest).catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
  }, [tid])

  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>
  if (!contest) return <section className="panel"><div className="empty-state">加载中...</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">CONTEST</p>
          <h1 className="panel-title">{contest.title}</h1>
        </div>
        <Link className="button" to="/contest">返回比赛</Link>
      </div>
      <div className="contest-meta">
        <span>赛制 {contest.rule || '-'}</span>
        <span>开始 {formatTime(contest.beginAt)}</span>
        <span>结束 {formatTime(contest.endAt)}</span>
        <span>参赛 {contest.attend}</span>
        <span>题目 {contest.pids?.length ?? 0}</span>
      </div>
      <div className="panel-body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{contest.content || '暂无比赛说明'}</ReactMarkdown>
      </div>
    </section>
  )
}
