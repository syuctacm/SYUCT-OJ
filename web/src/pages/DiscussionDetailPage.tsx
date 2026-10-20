import { useEffect, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import {
  createDiscussionReply,
  deleteDiscussionReply,
  getDiscussion,
  getDiscussionReplies,
  getPublicUser,
  setDiscussionLock,
  updateDiscussionReply,
  type Discussion,
  type DiscussionReply,
  type PublicUser,
} from '../api.ts'
import { useAuth } from '../auth.tsx'

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function Time({ value }: { value: string }) {
  return <span className="time" data-timestamp={Math.floor(new Date(value).getTime() / 1000)}>{formatTime(value)}</span>
}

export function DiscussionDetailPage() {
  const { did = '' } = useParams()
  const { user, loading: authLoading } = useAuth()
  const [discussion, setDiscussion] = useState<Discussion | null>(null)
  const [owner, setOwner] = useState<PublicUser | null>(null)
  const [replies, setReplies] = useState<DiscussionReply[]>([])
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [replyError, setReplyError] = useState('')
  const [actionError, setActionError] = useState('')
  const [locking, setLocking] = useState(false)
  const [editingReplyID, setEditingReplyID] = useState('')
  const [editingReplyContent, setEditingReplyContent] = useState('')
  const [replyActionBusy, setReplyActionBusy] = useState('')
  const [replyActionError, setReplyActionError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    Promise.all([getDiscussion(did), getDiscussionReplies(did)])
      .then(([nextDiscussion, nextReplies]) => {
        if (cancelled) return
        setDiscussion(nextDiscussion)
        setReplies(nextReplies)
        setError('')
        getPublicUser(String(nextDiscussion.owner)).then((profile) => {
          if (!cancelled) setOwner(profile)
        }).catch(() => {
          if (!cancelled) setOwner(null)
        })
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

  async function handleLockToggle() {
    if (!discussion || !user) return
    setLocking(true)
    setActionError('')
    try {
      const next = await setDiscussionLock(did, !discussion.lock)
      setDiscussion(next)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '讨论状态更新失败')
    } finally {
      setLocking(false)
    }
  }

  function canEditReply(ownerID: number) {
    return Boolean(user && (user.id === ownerID || ((user.priv ?? 0) & 1) !== 0))
  }

  function canDeleteReply(ownerID: number) {
    return Boolean(user && discussion && (user.id === ownerID || user.id === discussion.owner || ((user.priv ?? 0) & 1) !== 0))
  }

  function startReplyEdit(id: string, currentContent: string) {
    setEditingReplyID(id)
    setEditingReplyContent(currentContent)
    setReplyActionError('')
  }

  function cancelReplyEdit() {
    setEditingReplyID('')
    setEditingReplyContent('')
    setReplyActionError('')
  }

  async function saveReplyEdit(id: string) {
    if (!editingReplyContent.trim()) {
      setReplyActionError('回复内容不能为空')
      return
    }
    setReplyActionBusy(id)
    setReplyActionError('')
    try {
      const updated = await updateDiscussionReply(did, id, editingReplyContent)
      setReplies((current) => current.map((reply) => {
        if (reply.id === id) return { ...reply, content: updated.content, edited: true }
        if (reply.replies?.some((tail) => tail.id === id)) {
          return {
            ...reply,
            replies: reply.replies.map((tail) => tail.id === id ? { ...tail, content: updated.content, edited: true } : tail),
          }
        }
        return reply
      }))
      cancelReplyEdit()
    } catch (err) {
      setReplyActionError(err instanceof Error ? err.message : '回复保存失败')
    } finally {
      setReplyActionBusy('')
    }
  }

  async function removeReply(id: string) {
    if (!window.confirm('确认删除这条回复？')) return
    setReplyActionBusy(id)
    setReplyActionError('')
    try {
      await deleteDiscussionReply(did, id)
      const wasMain = replies.some((reply) => reply.id === id)
      setReplies((current) => current
        .filter((reply) => reply.id !== id)
        .map((reply) => reply.replies?.some((tail) => tail.id === id)
          ? { ...reply, replies: reply.replies.filter((tail) => tail.id !== id) }
          : reply))
      if (wasMain) {
        setDiscussion((current) => current ? { ...current, nReply: Math.max(0, current.nReply - 1) } : current)
      }
    } catch (err) {
      setReplyActionError(err instanceof Error ? err.message : '回复删除失败')
    } finally {
      setReplyActionBusy('')
    }
  }

  if (loading) {
    return (
      <div className="row"><div className="medium-9 columns"><div className="section"><div className="section__body">
        <div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div>
      </div></div></div></div>
    )
  }
  if (error || !discussion) {
    return (
      <div className="row"><div className="medium-9 columns"><div className="section"><div className="section__body">
        <div className="nothing-placeholder">{error || '讨论不存在'}</div>
      </div></div></div></div>
    )
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__body">
            <ul className="supplementary list">
              <li>
                <Link className="discussion-node-tag" to={`/discuss?node=${encodeURIComponent(String(discussion.parentId ?? ''))}`}>
                  <span className="v-center icon icon-flag" />
                  {discussion.parentTitle || String(discussion.parentId || '讨论')}
                </Link>
              </li>
              <li><h2>{discussion.title}</h2></li>
              <li>
                <Link to={`/user/${discussion.owner}`}>{owner?.displayName || owner?.username || `UID ${discussion.owner}`}</Link>
                {' @ '}
                <Time value={discussion.updateAt} />
                {discussion.edited ? '（已编辑）' : ''}
              </li>
              {user && (user.id === discussion.owner || ((user.priv ?? 0) & 1) !== 0) ? (
                <li><Link to={`/discuss/${encodeURIComponent(discussion.docId)}/edit`}><span className="icon icon-edit" /> 编辑</Link></li>
              ) : null}
            </ul>
            <div className="typo topic__content richmedia">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{discussion.content || '暂无内容'}</ReactMarkdown>
            </div>
          </div>
          <ul className="section__footer supplementary dot list">
            {discussion.lock ? <li>该讨论已锁定</li> : null}
            {actionError ? <li className="form-error">{actionError}</li> : null}
            <li>{discussion.views} 浏览</li>
            {user && ((user.priv ?? 0) & 1) !== 0 ? (
              <li>
                <button type="button" className="link-button" onClick={handleLockToggle} disabled={locking}>
                  {locking ? '处理中...' : discussion.lock ? '解锁讨论' : '锁定讨论'}
                </button>
              </li>
            ) : null}
            <li><Link to="/discuss"><span className="icon icon-arrow-left" /> 返回讨论区</Link></li>
          </ul>
        </div>

        <div className="section">
          <div className="section__header">
            <h1 className="section__title">{discussion.nReply} 条评论</h1>
          </div>
          <div className="section__body">
            <ul className="dczcomments__list view--discussion">
              <li className="dczcomments__item">
                <div className="media">
                  <div className="media__left top">
                    {user?.avatar ? (
                      <img src={user.avatar} width="60" height="60" className="medium user-profile-avatar" alt="" />
                    ) : (
                      <div className="medium user-profile-avatar" aria-hidden="true" />
                    )}
                  </div>
                  <div className="media__body top">
                    {discussion.lock ? (
                      <textarea className="textbox" name="dczcomments__dummy-box" readOnly placeholder="该讨论已锁定，不能再回复。" />
                    ) : authLoading ? (
                      <div className="nothing-placeholder compact">正在确认登录状态...</div>
                    ) : user ? (
                      <form className="dczcomments__box" onSubmit={handleReply}>
                        <div>
                          <textarea
                            name="content"
                            className="textbox"
                            value={content}
                            onChange={(event) => setContent(event.target.value)}
                            onKeyDown={(event) => {
                              if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                                event.preventDefault()
                                event.currentTarget.form?.requestSubmit()
                              }
                            }}
                            placeholder="写下评论"
                            required
                          />
                        </div>
                        {replyError ? <blockquote className="warn"><p>{replyError}</p></blockquote> : null}
                        <div>
                          <input
                            type="submit"
                            className="rounded primary button dczcomments__box__submit"
                            value={submitting ? '发表中...' : '评论 (Ctrl+Enter)'}
                            disabled={submitting}
                          />
                        </div>
                      </form>
                    ) : (
                      <div className="nothing-placeholder compact"><Link to="/login">登录</Link> 后参与讨论。</div>
                    )}
                  </div>
                </div>
              </li>

              {replyActionError ? <blockquote className="warn"><p>{replyActionError}</p></blockquote> : null}
              {replies.map((reply) => (
                <li className="dczcomments__item" key={reply.id}>
                  <div className="media">
                    <div className="media__left top">
                      <div className="medium user-profile-avatar" aria-hidden="true" />
                    </div>
                    <div className="media__body top" id={`comment-${reply.docId}`}>
                      <div className="clearfix">
                        <div className="supplementary dczcomments__supplementary">
                          <Link to={`/user/${reply.owner}`}>UID {reply.owner}</Link>
                          {' @ '}
                          <Time value={reply.createdAt} />
                          {reply.edited ? <div className="history"><a>已编辑</a></div> : null}
                          {canEditReply(reply.owner) ? (
                            <div className="history"><button type="button" className="link-button" onClick={() => startReplyEdit(reply.id, reply.content)}>编辑</button></div>
                          ) : null}
                          {canDeleteReply(reply.owner) ? (
                            <div className="history"><button type="button" className="link-button" onClick={() => removeReply(reply.id)} disabled={replyActionBusy === reply.id}>{replyActionBusy === reply.id ? '处理中...' : '删除'}</button></div>
                          ) : null}
                        </div>
                      </div>
                      {editingReplyID === reply.id ? (
                        <div className="typo richmedia">
                          <textarea className="textbox" style={{ width: '100%', minHeight: 100 }} value={editingReplyContent} onChange={(event) => setEditingReplyContent(event.target.value)} />
                          <div>
                            <button type="button" className="compact primary button" onClick={() => saveReplyEdit(reply.id)} disabled={replyActionBusy === reply.id}>{replyActionBusy === reply.id ? '保存中...' : '保存'}</button>
                            <button type="button" className="compact button" onClick={cancelReplyEdit}>取消</button>
                          </div>
                        </div>
                      ) : (
                        <div className="typo richmedia">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>{reply.content}</ReactMarkdown>
                        </div>
                      )}
                      {reply.replies?.length ? (
                        <ul className="dczcomments__replies">
                          {reply.replies.map((tail) => (
                            <li className="dczcomments__reply" key={tail.id}>
                              <div className="media">
                                <div className="media__left top">
                                  <div className="medium user-profile-avatar" aria-hidden="true" />
                                </div>
                                <div className="media__body top">
                                  <div className="clearfix">
                                    <div className="supplementary dczcomments__supplementary">
                                      <Link to={`/user/${tail.owner}`}>UID {tail.owner}</Link>
                                      {' @ '}
                                      <Time value={tail.createdAt} />
                                      {tail.edited ? <div className="history"><a>已编辑</a></div> : null}
                                      {canEditReply(tail.owner) ? (
                                        <div className="history"><button type="button" className="link-button" onClick={() => startReplyEdit(tail.id, tail.content)}>编辑</button></div>
                                      ) : null}
                                      {canDeleteReply(tail.owner) ? (
                                        <div className="history"><button type="button" className="link-button" onClick={() => removeReply(tail.id)} disabled={replyActionBusy === tail.id}>{replyActionBusy === tail.id ? '处理中...' : '删除'}</button></div>
                                      ) : null}
                                    </div>
                                  </div>
                                  {editingReplyID === tail.id ? (
                                    <div className="typo richmedia">
                                      <textarea className="textbox" style={{ width: '100%', minHeight: 90 }} value={editingReplyContent} onChange={(event) => setEditingReplyContent(event.target.value)} />
                                      <div>
                                        <button type="button" className="compact primary button" onClick={() => saveReplyEdit(tail.id)} disabled={replyActionBusy === tail.id}>{replyActionBusy === tail.id ? '保存中...' : '保存'}</button>
                                        <button type="button" className="compact button" onClick={cancelReplyEdit}>取消</button>
                                      </div>
                                    </div>
                                  ) : (
                                    <div className="typo richmedia">
                                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{tail.content}</ReactMarkdown>
                                    </div>
                                  )}
                                </div>
                              </div>
                            </li>
                          ))}
                        </ul>
                      ) : null}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
            {replies.length === 0 ? (
              <div className="nothing-placeholder"><div className="nothing-icon" />还没有评论</div>
            ) : null}
          </div>
        </div>
      </div>

      <div className="medium-3 columns">
        <div className="section side">
          <div className="profile__bg user-profile-bg--1" />
          <div className="section__body">
            <div className="media sidebar-user-profile">
              <div className="media__left">
                {owner?.avatar ? (
                  <img src={owner.avatar} width="80" height="80" className="medium user-profile-avatar" alt="" />
                ) : (
                  <div className="medium user-profile-avatar" aria-hidden="true" />
                )}
              </div>
              <div className="media__body">
                <p><Link to={`/user/${discussion.owner}`}>{owner?.displayName || owner?.username || `UID ${discussion.owner}`}</Link></p>
              </div>
            </div>
          </div>
          <div className="section__body">
            <div className="balancer sidebar-user-stat">
              <div className="balancer__body">
                <div className="numbox">
                  <div className="numbox__num medium">{owner?.nAccept ?? 0}</div>
                  <div className="numbox__text">通过</div>
                </div>
              </div>
              <div className="balancer__body">
                <div className="numbox">
                  <div className="numbox__num medium">0</div>
                  <div className="numbox__text">获赞题解</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
