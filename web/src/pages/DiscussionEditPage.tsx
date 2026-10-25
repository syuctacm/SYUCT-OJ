// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and event adapters; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  deleteDiscussion,
  getDiscussion,
  updateDiscussion,
  type Discussion,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'

export function DiscussionEditPage() {
  const { did = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [discussion, setDiscussion] = useState<Discussion | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    setLoading(true)
    getDiscussion(did)
      .then(setDiscussion)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
      .finally(() => setLoading(false))
  }, [did])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    const stopHydration = hydrateHydroEditForm(root, { uploadToUserFiles: true })
    return () => {
      form?.removeEventListener('submit', onSubmit)
      stopHydration()
    }
  }, [])

  const context = useMemo(() => {
    if (!discussion) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      ddoc: { ...discussion, content: discussion.content ?? '' },
      handler: {
        user: {
          hasPerm: () => isAdmin,
          hasPriv: () => isAdmin,
          own: () => Boolean(user && discussion.owner === user.id),
        },
      },
      perm: {
        PERM_DELETE_DISCUSSION: 1,
        PERM_HIGHLIGHT_DISCUSSION: 2,
        PERM_PIN_DISCUSSION: 3,
      },
      UserContext: user,
    }
  }, [discussion, user])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const submitter = event.submitter as HTMLButtonElement | null
    if (submitter?.value === 'delete') {
      if (!(await confirmDialog('确认删除这个讨论？所有回复也会被删除。'))) return
      setSaving(true)
      setError('')
      try {
        await deleteDiscussion(did)
        navigate('/discuss', { replace: true })
      } catch (err) {
        setError(err instanceof Error ? err.message : '讨论删除失败')
        setSaving(false)
      }
      return
    }
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const title = String(data.get('title') ?? '').trim()
    const content = String(data.get('content') ?? '')
    setSaving(true)
    setError('')
    try {
      const item = await updateDiscussion(did, {
        title,
        content,
        pin: data.get('pin') !== null,
        highlight: data.get('highlight') !== null,
      })
      navigate(`/discuss/${item.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '讨论保存失败')
    } finally {
      setSaving(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (!discussion || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '讨论不存在'}</div></div></div>
  }

  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (user && user.id !== discussion.owner && !isAdmin) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有编辑这个讨论的权限</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {saving ? <div className="row"><div className="columns"><blockquote className="note"><p>保存中...</p></blockquote></div></div> : null}
      <HydroTemplate template="discussion_edit.html" context={context} onReady={handleReady} />
    </>
  )
}
