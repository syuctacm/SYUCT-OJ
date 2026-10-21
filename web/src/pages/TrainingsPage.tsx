// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to query state, data mapping and small interactions.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { getTrainings, type Training } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

export function TrainingsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const page = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [items, setItems] = useState<Training[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    setError('')
    getTrainings(query, page)
      .then((data) => { setItems(data.items); setTotal(data.total) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
      .finally(() => setLoading(false))
  }, [query, page])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form[name="filter-form"]')
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      const data = new FormData(form!)
      const value = String(data.get('q') ?? '').trim()
      navigate(`/training${value ? `?q=${encodeURIComponent(value)}` : ''}`)
    }
    form?.addEventListener('submit', onSubmit)
    return () => form?.removeEventListener('submit', onSubmit)
  }, [navigate])

  const context = useMemo(() => {
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const tdocs = items.map((training) => {
      // The list API returns aggregate counts instead of the full DAG.
      const dag = Array.from({ length: training.sectionCount }, () => ({ pids: [] as number[] }))
      for (let index = 0; index < training.problemCount; index++) {
        dag[index % Math.max(1, dag.length)]?.pids.push(index + 1)
      }
      return { ...training, dag, domainId: 'system' }
    })
    const tsdict = Object.fromEntries(tdocs.map((training) => [
      String(training.docId),
      { docId: training.docId, enroll: false, done: false, donePids: [] },
    ]))
    const tdict = Object.fromEntries(tdocs.map((training) => [String(training.docId), training]))
    return {
      page_name: 'training_main',
      tdocs,
      tdict,
      tsdict,
      q: query,
      page,
      tpcount: Math.max(1, Math.ceil(total / 20)),
      qs: query ? `q=${encodeURIComponent(query)}` : '',
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => permission === 1 && isAdmin,
          hasPriv: () => false,
          own: () => false,
        },
        args: { domainId: 'system' },
        request: { query: { q: query }, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_CREATE_TRAINING: 1 },
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: {
        training: {
          getPids: (dag: Array<{ pids?: Array<number | string> }>) => dag.flatMap((node) => node.pids ?? []),
        },
      },
      utils: { getAlphabeticId: () => '' },
      avatarUrl,
      UserContext: user,
    }
  }, [items, query, user])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>

  return (
    <HydroTemplate
      template="training_main.html"
      context={context}
      onReady={handleReady}
      url={(name, args = {}) => {
        if (name === 'training_detail') return `/training/${encodeURIComponent(String(args.tid ?? ''))}`
        if (name === 'training_create') return '/training/new'
        return '/'
      }}
    />
  )
}
