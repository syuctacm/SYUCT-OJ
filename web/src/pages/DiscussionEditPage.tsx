import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  deleteDiscussion,
  getDiscussion,
  updateDiscussion,
  type Discussion,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { FormCheckbox, FormText, FormTextarea, MarkdownHint } from '../components/LegacyForm.tsx'

export function DiscussionEditPage() {
  const { did = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [discussion, setDiscussion] = useState<Discussion | null>(null)
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [pin, setPin] = useState(false)
  const [highlight, setHighlight] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const formRef = useRef<HTMLFormElement>(null)

  useEffect(() => {
    setLoading(true)
    getDiscussion(did)
      .then((item) => {
        setDiscussion(item)
        setTitle(item.title)
        setContent(item.content || '')
        setPin(item.pin)
        setHighlight(item.highlight)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
      .finally(() => setLoading(false))
  }, [did])

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
        event.preventDefault()
        formRef.current?.requestSubmit()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaving(true)
    setError('')
    try {
      const item = await updateDiscussion(did, { title: title.trim(), content, pin, highlight })
      navigate(`/discuss/${item.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '讨论保存失败')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    if (!window.confirm('确认删除这个讨论？所有回复也会被删除。')) return
    setSaving(true)
    setError('')
    try {
      await deleteDiscussion(did)
      navigate('/discuss', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '讨论删除失败')
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (error && !discussion) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!discussion) return null

  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (user && user.id !== discussion.owner && !isAdmin) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有编辑这个讨论的权限</div></div></div>
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__body">
            <form ref={formRef} method="post" onSubmit={handleSubmit}>
              <div className="row">
                <FormText
                  columns={8}
                  row={false}
                  label="标题"
                  name="title"
                  placeholder="标题"
                  value={title}
                  autoFocus
                  onChange={(event) => setTitle(event.target.value)}
                />
                <FormCheckbox
                  columns={2}
                  row={false}
                  label=""
                  name="highlight"
                  checked={highlight}
                  placeholder="高亮"
                  disabled={!isAdmin}
                  onChange={(event) => setHighlight(event.target.checked)}
                />
                <FormCheckbox
                  columns={2}
                  row={false}
                  lastColumn
                  label=""
                  name="pin"
                  checked={pin}
                  placeholder="置顶"
                  disabled={!isAdmin}
                  onChange={(event) => setPin(event.target.checked)}
                />
              </div>
              <FormTextarea
                columns={12}
                label="内容 (必填)"
                name="content"
                value={content}
                markdown
                required
                onValueChange={setContent}
              />
              <div className="row">
                <div className="columns">
                  {error ? <p className="form-error">{error}</p> : null}
                  <button name="operation" value="update" type="submit" className="rounded primary button" disabled={saving}>
                    {saving ? '保存中...' : '更新'} (Ctrl+Enter)
                  </button>
                  {user && (user.id === discussion.owner || isAdmin) ? (
                    <button name="operation" value="delete" type="button" className="rounded button" onClick={handleDelete} disabled={saving}>
                      删除
                    </button>
                  ) : null}
                  <button type="button" className="rounded button" onClick={() => window.history.go(-1)}>取消</button>
                </div>
              </div>
            </form>
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <MarkdownHint />
      </div>
    </div>
  )
}
