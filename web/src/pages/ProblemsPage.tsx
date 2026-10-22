// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to query state, data mapping and small interactions.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { getProblems, type ProblemPage } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { statusClass, statusLabels } from '../status.ts'
import { avatarUrl } from '../upstream/avatar.ts'

const statusCodes = Object.fromEntries(Object.keys(statusLabels).map((status) => [
  status,
  statusClass(Number(status)),
]))

const statusShortTexts: Record<number, string> = {
  1: 'AC',
  2: 'WA',
  3: 'TLE',
  4: 'MLE',
  5: 'OLE',
  6: 'RE',
  7: 'CE',
  8: 'SE',
  9: 'Canceled',
  10: 'Unknown',
  11: 'Hacked',
  20: 'Running',
  21: 'Compiling',
  22: 'Fetched',
  30: 'Ignored',
}

export function ProblemsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const tag = searchParams.get('tag') ?? ''
  const sort = searchParams.get('sort') === 'recent' ? 'recent' : 'default'
  const pageNumber = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [page, setPage] = useState<ProblemPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    setError('')
    getProblems(query, pageNumber, sort, tag)
      .then(setPage)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
      .finally(() => setLoading(false))
  }, [query, pageNumber, sort, tag])

  const ppcount = page ? Math.max(1, Math.ceil(page.total / page.limit)) : 0

  const handleReady = useCallback((root: HTMLDivElement) => {
    root.querySelector('.section.side.nojs--hide')?.remove()
    const form = root.querySelector<HTMLFormElement>('form#searchForm')
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      const data = new FormData(form!)
      const params = new URLSearchParams()
      const q = String(data.get('q') ?? '').trim()
      const nextSort = String(data.get('sort') ?? 'default')
      if (q) params.set('q', q)
      if (tag) params.set('tag', tag)
      if (nextSort === 'recent') params.set('sort', 'recent')
      navigate(`/p${params.toString() ? `?${params.toString()}` : ''}`)
    }
    form?.addEventListener('submit', onSubmit)
    const sortSelect = root.querySelector<HTMLSelectElement>('select[name="sort"]')
    const onSort = () => form?.requestSubmit()
    sortSelect?.addEventListener('change', onSort)

    const toggleTags = () => {
      const table = root.querySelector<HTMLElement>('table.data-table')
      if (table) table.classList.toggle('hide-problem-tag')
    }
    const tagButtons = Array.from(root.querySelectorAll<HTMLElement>('.toggle-tag'))
    tagButtons.forEach((button) => button.addEventListener('click', toggleTags))

    return () => {
      form?.removeEventListener('submit', onSubmit)
      sortSelect?.removeEventListener('change', onSort)
      tagButtons.forEach((button) => button.removeEventListener('click', toggleTags))
    }
  }, [navigate, tag])

  const context = useMemo(() => {
    if (!page) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const pdocs = page.items.map((problem) => ({
      ...problem,
      tag: problem.tags ?? [],
      domainId: 'system',
    }))
    const psdict = Object.fromEntries(page.items.map((problem) => [
      String(problem.docId),
      {
        rid: problem.userStatus?.rid,
        status: problem.userStatus?.status,
        star: false,
      },
    ]))
    return {
      page_name: 'problem_main',
      qs: query,
      sort,
      pdocs,
      ppcount,
      page: page.page,
      psdict,
      pcount: page.total,
      pcountRelation: 'eq',
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => permission === 1 && isAdmin,
          hasPriv: () => Boolean(user),
          own: () => false,
        },
        args: { domainId: 'system' },
        request: { query: { q: query, sort, tag }, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_CREATE_PROBLEM: 1, PERM_EDIT_PROBLEM: 2 },
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: {
        builtin: {
          LEVELS: {},
          STATUS_CODES: statusCodes,
          STATUS_TEXTS: statusLabels,
          STATUS_SHORT_TEXTS: statusShortTexts,
        },
        setting: { langs: {} },
        system: { get: () => false },
      },
      lib: { difficulty: () => 0 },
      ui: {
        getNodes: (name: string) => name === 'ProblemAdd' ? [
          { name: 'problem_create', args: { text: 'Create Problem', icon: 'add' } },
          { name: 'problem_import_fps', args: { text: 'From FPS File', icon: 'copy' } },
          { name: 'problem_import_hydro', args: { text: 'Import From Hydro', icon: 'copy' } },
        ] : [],
      },
      utils: {
        buildQueryString: (values: Record<string, unknown>) => {
          const params = new URLSearchParams()
          for (const [key, value] of Object.entries(values)) {
            if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
          }
          if (tag && !params.has('tag')) params.set('tag', tag)
          return params.toString()
        },
        status: {
          getScoreColor: (score: number) => score >= 100 ? '#25ad40' : score > 0 ? '#e64c72' : '#999',
        },
      },
      avatarUrl,
      UserContext: user,
    }
  }, [page, ppcount, query, sort, tag, user])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!context) return null

  return (
    <HydroTemplate
      template="problem_main.html"
      context={context}
      onReady={handleReady}
      url={(name, args = {}) => {
        const queryArgs = (args.query ?? {}) as Record<string, unknown>
        if (name === 'problem_main') {
          const q = String(queryArgs.q ?? '')
          if (q.startsWith('category:')) return `/p?tag=${encodeURIComponent(q.slice('category:'.length))}`
          const params = new URLSearchParams()
          for (const [key, value] of Object.entries(queryArgs)) if (value !== undefined && value !== '') params.set(key, String(value))
          return `/p${params.toString() ? `?${params.toString()}` : ''}`
        }
        if (name === 'problem_detail') return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
        if (name === 'problem_create') return '/p/new'
        if (name === 'problem_import_fps') return '/problem/import/fps'
        if (name === 'problem_import_hydro') return '/problem/import/hydro'
        if (name === 'problem_random') {
          const q = String(queryArgs.q ?? '')
          return `/problem/random${q ? `?q=${encodeURIComponent(q)}` : ''}`
        }
        if (name === 'record_detail') return `/record/${encodeURIComponent(String(args.rid ?? ''))}`
        return '/'
      }}
    />
  )
}
