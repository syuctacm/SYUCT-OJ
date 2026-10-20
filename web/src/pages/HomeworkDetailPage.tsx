import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useParams } from 'react-router-dom'
import { getHomework, type Contest } from '../api.ts'

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false })
}

export function HomeworkDetailPage() {
  const { tid = '' } = useParams()
  const [homework, setHomework] = useState<Contest | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getHomework(tid).then(setHomework).catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
  }, [tid])

  if (error) return <section className="section"><div className="empty-state error-state">{error}</div></section>
  if (!homework) return <section className="section"><div className="empty-state">加载中...</div></section>

  return (
    <section className="section">
      <div className="section__header">
        <div>
          <p className="eyebrow">HOMEWORK</p>
          <h1 className="section__title">{homework.title}</h1>
        </div>
      </div>
      <div className="contest-meta">
        <span>开始 {formatTime(homework.beginAt)}</span>
        <span>截止 {formatTime(homework.endAt)}</span>
        <span>题目 {homework.pids?.length ?? 0}</span>
      </div>
      <div className="section__body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{homework.content || '暂无作业说明'}</ReactMarkdown>
      </div>
    </section>
  )
}
