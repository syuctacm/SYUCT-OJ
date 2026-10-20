import { useEffect, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import {
  createDiscussionReply,
  getDiscussion,
  getDiscussionReplies,
  type Discussion,
  type DiscussionReply,
} from '../api.ts'
import { useAuth } from '../auth.tsx'

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

export function DiscussionDetailPage() {
  const { did = '' } = useParams()
  const { user, loading: authLoading } = useAuth()
  const [discussion, setDiscussion] = useState<Discussion | null>(null)
  const [replies, setReplies] = useState<DiscussionReply[]>([])
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [replyError, setReplyError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    Promise.all([getDiscussion(did), getDiscussionReplies(did)])
      .then(([nextDiscussion, nextReplies]) => {
        if (cancelled) return
        setDiscussion(nextDiscussion)
        setReplies(nextReplies)
        setError('')
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : '讨论加载失败')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [did])

  async function handleReply(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!user || !discussion) return
    if (!content.trim()) {
      setReplyError('回复内容不能为空')
      return
    }
    setSubmitting(true)
    setReplyError('')
    try {
      const reply = await createDiscussionReply(did, content)
      setReplies((current) => [reply, ...current])
      setDiscussion((current) => current ? { ...current, nReply: current.nReply + 1 } : current)
      setContent('')
    } catch (err) {
      setReplyError(err instanceof Error ? err.message : '回复失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) return <section className="panel"><div className="empty-state">加载中...</div></section>
  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>
  if (!discussion) return <section className="panel"><div className="empty-state">讨论不存在</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">DISCUSSION</p>
          <h1 className="panel-title">{discussion.title}</h1>
          <p className="panel-subtitle">
            <Link to={`/user/${discussion.owner}`}>UID {discussion.owner}</Link>
            {' · '}{discussion.nReply} 回复 · {discussion.views} 浏览
            {discussion.lock ? ' · 已锁定' : ''}
          </p>
        </div>
        <Link className="button" to="/discuss">返回讨论区</Link>
      </div>
      <div className="panel-body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{discussion.content || '暂无内容'}</ReactMarkdown>
      </div>

      <div className="discussion-replies-header">
        <h2>回复 {discussion.nReply}</h2>
      </div>
      <div className="discussion-reply-list">
        {replies.map((reply) => (
          <article className="discussion-reply" key={reply.id}>
            <div className="discussion-reply-meta">
              <Link to={`/user/${reply.owner}`}>UID {reply.owner}</Link>
              <span>{formatTime(reply.createdAt)}</span>
              {reply.edited ? <span>已编辑</span> : null}
            </div>
            <div className="problem-content discussion-reply-content">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{reply.content}</ReactMarkdown>
            </div>
            {reply.replies?.length ? (
              <div className="discussion-tail-replies">
                {reply.replies.map((tail) => (
                  <div className="discussion-tail-reply" key={tail.id}>
                    <div className="discussion-reply-meta">
                      <Link to={`/user/${tail.owner}`}>UID {tail.owner}</Link>
                      <span>{formatTime(tail.createdAt)}</span>
                      {tail.edited ? <span>已编辑</span> : null}
                    </div>
                    <div className="problem-content discussion-reply-content">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{tail.content}</ReactMarkdown>
                    </div>
                  </div>
                ))}
              </div>
            ) : null}
          </article>
        ))}
        {replies.length === 0 ? <div className="empty-state">还没有回复</div> : null}
      </div>

      <div className="discussion-reply-compose">
        {discussion.lock ? (
          <p className="form-hint">该讨论已锁定，不能再回复。</p>
        ) : authLoading ? (
          <p className="form-hint">正在确认登录状态...</p>
        ) : user ? (
          <form className="form-stack" onSubmit={handleReply}>
            <label className="field">
              <span>写下回复</span>
              <textarea
                className="textarea discussion-reply-input"
                value={content}
                onChange={(event) => setContent(event.target.value)}
                onKeyDown={(event) => {
                  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                    event.preventDefault()
                    event.currentTarget.form?.requestSubmit()
                  }
                }}
                placeholder="支持 Markdown，Ctrl+Enter 提交"
              />
            </label>
            {replyError ? <p className="form-error">{replyError}</p> : null}
            <div>
              <button className="button button-primary" type="submit" disabled={submitting}>
                {submitting ? '回复中...' : '发表回复'}
              </button>
            </div>
          </form>
        ) : (
          <p className="form-hint"><Link to="/login">登录</Link> 后参与讨论。</p>
        )}
      </div>
    </section>
  )
}
