// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and event adapters; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { createDiscussion, getDiscussionNodes, type DiscussionNode } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'

export function DiscussionCreatePage() {
  const { name = '' } = useParams()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { user, loading: authLoading } = useAuth()
  const [nodes, setNodes] = useState<DiscussionNode[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const legacyNode = searchParams.get('node') ?? ''

  useEffect(() => {
    if (!name) return
    getDiscussionNodes()
      .then(setNodes)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论节点加载失败'))
      .finally(() => setLoading(false))
  }, [name])

  const vnode = nodes.find((node) => node.docId === name)
  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    const stopHydration = hydrateHydroEditForm(root)
    return () => {
      form?.removeEventListener('submit', onSubmit)
      stopHydration()
    }
  }, [])

  const context = useMemo(() => {
    if (!vnode) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      vnode: { ...vnode, _id: vnode.docId, title: vnode.title || vnode.docId },
      ddoc: {},
      handler: {
        user: {
          hasPerm: () => isAdmin,
          hasPriv: () => isAdmin,
          own: () => false,
        },
      },
      perm: {
        PERM_HIGHLIGHT_DISCUSSION: 1,
        PERM_PIN_DISCUSSION: 2,
      },
      UserContext: user,
    }
  }, [user, vnode])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const title = String(data.get('title') ?? '').trim()
    const content = String(data.get('content') ?? '')
    setSaving(true)
    setError('')
    try {
      const discussion = await createDiscussion({
        nodeId: name,
        title,
        content,
        pin: data.get('pin') !== null,
        highlight: data.get('highlight') !== null,
      })
      navigate(`/discuss/${discussion.docId}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : '发帖失败')
    } finally {
      setSaving(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  if (!name) {
    return <Navigate to={legacyNode ? `/discuss/node/${encodeURIComponent(legacyNode)}/create` : '/discuss'} replace />
  }
  if (authLoading) return <div className="row"><div className="columns"><div className="nothing-placeholder">正在确认登录状态...</div></div></div>
  if (!user) return <Navigate to="/login" replace />
  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  if (!vnode || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '讨论节点不存在'}</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {saving ? <div className="row"><div className="columns"><blockquote className="note"><p>发布中...</p></blockquote></div></div> : null}
      <HydroTemplate template="discussion_create.html" context={context} onReady={handleReady} />
    </>
  )
}
