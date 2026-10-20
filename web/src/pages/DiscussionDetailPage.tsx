import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useParams } from 'react-router-dom'
import { getDiscussion, type Discussion } from '../api.ts'

export function DiscussionDetailPage() {
  const { did = '' } = useParams()
  const [discussion, setDiscussion] = useState<Discussion | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getDiscussion(did).then(setDiscussion).catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
  }, [did])

  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>
  if (!discussion) return <section className="panel"><div className="empty-state">加载中...</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">DISCUSSION</p>
          <h1 className="panel-title">{discussion.title}</h1>
          <p className="panel-subtitle">UID {discussion.owner} · {discussion.nReply} 回复 · {discussion.views} 浏览</p>
        </div>
      </div>
      <div className="panel-body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{discussion.content || '暂无内容'}</ReactMarkdown>
      </div>
    </section>
  )
}
