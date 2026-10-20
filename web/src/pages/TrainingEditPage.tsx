// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and event adapters; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  createTraining,
  getTrainingEditor,
  updateTraining,
  type Training,
  type TrainingEditInput,
  type TrainingEditNode,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'

type TrainingDraft = Training & { dagText?: string }

function dagText(item?: Training) {
  if (!item?.dag?.length) return ''
  return JSON.stringify(item.dag.map((node) => ({
    _id: node.id,
    title: node.title,
    ...(node.content ? { content: node.content } : {}),
    requireNids: node.requireNids,
    pids: node.pids,
  })), null, 2)
}

export function TrainingEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { tid = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [item, setItem] = useState<TrainingDraft | null>(mode === 'create' ? {
    id: '',
    docId: '',
    title: '',
    description: '',
    content: '',
    owner: user?.id ?? 0,
    attend: 0,
    pin: false,
    sectionCount: 0,
    problemCount: 0,
    dagText: '',
  } : null)
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    getTrainingEditor(tid)
      .then((training) => setItem({ ...training, dagText: dagText(training) }))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
      .finally(() => setLoading(false))
  }, [mode, tid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    const stopHydration = hydrateHydroEditForm(root, {
      resource: mode === 'edit' ? 'trainings' : undefined,
      tid: mode === 'edit' ? tid : undefined,
    })
    return () => {
      form?.removeEventListener('submit', onSubmit)
      stopHydration()
    }
  }, [mode, tid])

  const context = useMemo(() => {
    if (!item) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      page_name: mode === 'edit' ? 'training_edit' : 'training_create',
      tdoc: mode === 'edit' ? { ...item, pin: item.pin ? 1 : 0 } : null,
      dag: item.dagText ?? '',
      handler: {
        user: {
          // Delete is hidden until the Go API exposes resource deletion.
          hasPerm: () => false,
          hasPriv: () => isAdmin,
          own: () => false,
        },
      },
      perm: { PERM_EDIT_TRAINING: 1 },
      UserContext: user,
    }
  }, [item, mode, user])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const text = (name: string) => String(data.get(name) ?? '')
    let dag: TrainingEditNode[]
    try {
      const parsed = JSON.parse(text('dag')) as unknown
      if (!Array.isArray(parsed)) throw new Error('训练计划必须是 JSON 数组')
      dag = parsed as TrainingEditNode[]
    } catch (err) {
      setError(err instanceof Error ? err.message : '训练计划 JSON 格式不正确')
      return
    }
    const pin = Number(text('pin'))
    const input: TrainingEditInput = {
      title: text('title').trim(),
      content: text('content'),
      description: text('description'),
      pin: Number.isFinite(pin) && pin > 0,
      dag,
    }
    setSaving(true)
    setError('')
    try {
      const saved = mode === 'create' ? await createTraining(input) : await updateTraining(tid, input)
      navigate(`/training/${saved.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '训练保存失败')
    } finally {
      setSaving(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (mode === 'create' && user && !isAdmin) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有创建训练的权限</div></div></div>
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (!item || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '训练不存在'}</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {saving ? <div className="row"><div className="columns"><blockquote className="note"><p>保存中...</p></blockquote></div></div> : null}
      <HydroTemplate template="training_edit.html" context={context} onReady={handleReady} />
    </>
  )
}
