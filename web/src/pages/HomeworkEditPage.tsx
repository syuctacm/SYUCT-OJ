// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and event adapters; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  createHomework,
  getHomeworkEditor,
  updateHomework,
  type Contest,
  type ContestEditInput,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'

const defaultPenaltyRules = `1: 0.9
3: 0.8
12: 0.75
9999: 0.5`

type HomeworkDraft = Contest & {
  penaltyRulesText?: string
}

function addDays(date: Date, days: number) {
  const next = new Date(date)
  next.setDate(next.getDate() + days)
  return next
}

function defaultHomework(owner = 0): HomeworkDraft {
  const begin = new Date()
  begin.setDate(begin.getDate() + 1)
  begin.setHours(0, 0, 0, 0)
  const penalty = addDays(begin, 7)
  penalty.setHours(23, 59, 0, 0)
  return {
    id: '',
    docId: '',
    title: '',
    content: '',
    beginAt: begin.toISOString(),
    endAt: addDays(penalty, 1).toISOString(),
    rule: 'homework',
    owner,
    attend: 0,
    pids: [],
    penaltySince: penalty.toISOString(),
    penaltyRules: { 1: 0.9, 3: 0.8, 12: 0.75, 9999: 0.5 },
    assign: [],
    maintainer: [],
    langs: [],
    penaltyRulesText: defaultPenaltyRules,
  }
}

function splitList(value: string) {
  return value.replace(/，/g, ',').split(',').map((item) => item.trim()).filter(Boolean)
}

function listOfNumbers(value: string) {
  return splitList(value).map((item) => Number.parseInt(item, 10)).filter((item) => item > 0)
}

function rulesToText(rules?: Record<string, number>) {
  if (!rules || !Object.keys(rules).length) return defaultPenaltyRules
  return Object.entries(rules)
    .sort(([left], [right]) => Number(left) - Number(right))
    .map(([hours, coefficient]) => `${hours}: ${coefficient}`)
    .join('\n')
}

function parsePenaltyRules(value: string): Record<string, number> | null {
  const rules: Record<string, number> = {}
  for (const rawLine of value.split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue
    const match = /^([^:]+):\s*(-?\d+(?:\.\d+)?)$/.exec(line)
    if (!match) return null
    const hours = Number(match[1])
    const coefficient = Number(match[2])
    if (!Number.isFinite(hours) || hours < 0 || !Number.isFinite(coefficient) || coefficient < 0) return null
    rules[String(hours)] = coefficient
  }
  return Object.keys(rules).length ? rules : null
}

function parts(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return {
    date: `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`,
    time: `${pad(date.getHours())}:${pad(date.getMinutes())}`,
  }
}

export function HomeworkEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { tid = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const { user } = useAuth()
  const draft = (location.state as { draft?: Partial<HomeworkDraft> } | null)?.draft
  const [item, setItem] = useState<HomeworkDraft | null>(() => (
    mode === 'create' ? { ...defaultHomework(user?.id ?? 0), ...draft } : null
  ))
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    getHomeworkEditor(tid)
      .then((homework) => {
        setItem({ ...homework, penaltyRulesText: rulesToText(homework.penaltyRules) })
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
      .finally(() => setLoading(false))
  }, [mode, tid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    const stopHydration = hydrateHydroEditForm(root, {
      resource: mode === 'edit' ? 'homeworks' : undefined,
      tid: mode === 'edit' ? tid : undefined,
      autocomplete: { assign: 'assign', users: 'maintainer', problems: 'pids', langs: 'langs' },
      dateTime: true,
    })
    return () => {
      form?.removeEventListener('submit', onSubmit)
      stopHydration()
    }
  }, [mode, tid])

  const context = useMemo(() => {
    if (!item) return null
    const begin = new Date(item.beginAt)
    const penalty = new Date(item.penaltySince || item.endAt)
    const end = new Date(item.endAt)
    const extensionDays = Math.max(0, Math.round(((end.getTime() - penalty.getTime()) / 86400000) * 100) / 100)
    const beginParts = parts(begin)
    const penaltyParts = parts(penalty)
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      page_name: mode === 'edit' ? 'homework_edit' : 'homework_create',
      tdoc: mode === 'edit' ? {
        ...item,
        assign: (item.assign ?? []).join(','),
        maintainer: (item.maintainer ?? []).join(','),
        langs: (item.langs ?? []).join(','),
      } : null,
      dateBeginText: beginParts.date,
      timeBeginText: beginParts.time,
      datePenaltyText: penaltyParts.date,
      timePenaltyText: penaltyParts.time,
      extensionDays,
      penaltyRules: item.penaltyRulesText ?? rulesToText(item.penaltyRules),
      pids: (item.pids ?? []).join(','),
      handler: {
        user: {
          // Delete is hidden until the Go API exposes resource deletion.
          hasPerm: () => false,
          hasPriv: () => isAdmin,
          own: () => false,
        },
      },
      perm: { PERM_EDIT_HOMEWORK: 1 },
      UserContext: user,
    }
  }, [item, mode, user])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const submitter = event.submitter as HTMLButtonElement | null
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const text = (name: string) => String(data.get(name) ?? '')
    const begin = new Date(`${text('beginAtDate')}T${text('beginAtTime') || '00:00'}:00`)
    const penalty = new Date(`${text('penaltySinceDate')}T${text('penaltySinceTime') || '00:00'}:00`)
    const extensionDays = Number(text('extensionDays'))
    if (Number.isNaN(begin.getTime()) || Number.isNaN(penalty.getTime()) || penalty <= begin) {
      setError('截止时间必须晚于开始时间')
      return
    }
    if (!Number.isFinite(extensionDays) || extensionDays < 0) {
      setError('延期天数不正确')
      return
    }
    const pids = listOfNumbers(text('pids'))
    if (!pids.length) {
      setError('至少需要一道题目')
      return
    }
    const penaltyRules = parsePenaltyRules(text('penaltyRules'))
    if (!penaltyRules) {
      setError('延期递交扣分规则格式不正确')
      return
    }
    const end = addDays(penalty, extensionDays)
    const input: ContestEditInput = {
      title: text('title').trim(),
      content: text('content'),
      beginAt: begin.toISOString(),
      endAt: end.toISOString(),
      rule: 'homework',
      pids,
      duration: Math.max(1, Math.round((end.getTime() - begin.getTime()) / 3600000)),
      rated: false,
      unlocked: false,
      penaltySince: penalty.toISOString(),
      penaltyRules,
      maintainer: listOfNumbers(text('maintainer')),
      assign: splitList(text('assign')),
      langs: splitList(text('langs')),
    }
    if (submitter?.formAction && mode === 'edit' && submitter.formAction.endsWith('/homework/new')) {
      navigate('/homework/new', { state: { draft: { ...item, ...input } } })
      return
    }
    setSaving(true)
    setError('')
    try {
      const saved = mode === 'create' ? await createHomework(input) : await updateHomework(tid, input)
      navigate(`/homework/${saved.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '作业保存失败')
    } finally {
      setSaving(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (mode === 'create' && user && !isAdmin) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有创建作业的权限</div></div></div>
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (!item || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '作业不存在'}</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {saving ? <div className="row"><div className="columns"><blockquote className="note"><p>保存中...</p></blockquote></div></div> : null}
      <HydroTemplate template="homework_edit.html" context={context} onReady={handleReady} />
    </>
  )
}
