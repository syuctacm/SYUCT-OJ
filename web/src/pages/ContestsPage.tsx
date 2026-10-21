// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to query state, data mapping and small interactions.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { getContests, type Contest } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

const ruleNames: Record<string, string> = {
  acm: 'XCPC / ACM',
  oi: 'OI',
  ioi: 'IOI',
  ledo: 'Ledo',
  strictioi: 'Strict IOI',
}

function pad(value: number) {
  return String(value).padStart(2, '0')
}

function formatByPattern(value: Date, pattern: string) {
  return pattern
    .replace(/YYYY/g, String(value.getFullYear()))
    .replace(/MM/g, pad(value.getMonth() + 1))
    .replace(/M/g, String(value.getMonth() + 1))
    .replace(/DD/g, pad(value.getDate()))
    .replace(/D/g, String(value.getDate()))
    .replace(/HH/g, pad(value.getHours()))
    .replace(/H/g, String(value.getHours()))
    .replace(/mm/g, pad(value.getMinutes()))
}

export function ContestsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const rule = searchParams.get('rule') ?? ''
  const group = searchParams.get('group') ?? ''
  const page = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [items, setItems] = useState<Contest[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    setError('')
    getContests(query, rule, page)
      .then((data) => { setItems(data.items); setTotal(data.total) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
      .finally(() => setLoading(false))
  }, [query, rule, page])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form[name="filter-form"]')
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      const data = new FormData(form!)
      const params = new URLSearchParams()
      for (const key of ['q', 'group', 'rule']) {
        const value = String(data.get(key) ?? '').trim()
        if (value) params.set(key, value)
      }
      navigate(`/contest${params.toString() ? `?${params.toString()}` : ''}`)
    }
    form?.addEventListener('submit', onSubmit)
    const ruleSelect = form?.querySelector<HTMLSelectElement>('select[name="rule"]')
    const onRuleChange = () => form?.requestSubmit()
    ruleSelect?.addEventListener('change', onRuleChange)
    return () => {
      form?.removeEventListener('submit', onSubmit)
      ruleSelect?.removeEventListener('change', onRuleChange)
    }
  }, [navigate])

  const context = useMemo(() => {
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const tdocs = items.map((contest) => {
      const beginAt = new Date(contest.beginAt)
      const endAt = new Date(contest.endAt)
      return {
        ...contest,
        beginAt,
        endAt,
        duration: contest.duration || Math.max(0, (endAt.getTime() - beginAt.getTime()) / 3600000),
        domainId: 'system',
      }
    })
    const tsdict = Object.fromEntries(tdocs.map((contest) => [
      String(contest.docId),
      { startAt: contest.beginAt, attend: false },
    ]))
    const params = new URLSearchParams()
    if (query) params.set('q', query)
    if (group) params.set('group', group)
    if (rule) params.set('rule', rule)
    return {
      page_name: 'contest_main',
      tdocs,
      tsdict,
      tsdoc: {},
      page,
      tpcount: Math.max(1, Math.ceil(total / 20)),
      qs: params.toString(),
      q: query,
      rule,
      group,
      groups: [],
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
        request: { query: { q: query, rule, group }, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_CREATE_CONTEST: 1 },
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: {
        contest: {
          RULES: Object.fromEntries(Object.entries(ruleNames).map(([key, text]) => [key, { TEXT: text, hidden: false }])),
          isOngoing: (contest: { beginAt: Date; endAt: Date }) => contest.beginAt.getTime() <= Date.now() && contest.endAt.getTime() >= Date.now(),
          isUpcoming: (contest: { beginAt: Date }) => contest.beginAt.getTime() > Date.now(),
        },
      },
      utils: { getAlphabeticId: () => '' },
      datetimeSpan: (value: Date | string, _relative?: boolean, pattern = 'YYYY-M-D H:mm') => {
        const date = value instanceof Date ? value : new Date(value)
        return `<span class="time">${formatByPattern(date, pattern)}</span>`
      },
      avatarUrl,
      UserContext: user,
    }
  }, [group, items, query, rule, user])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>

  return (
    <HydroTemplate
      template="contest_main.html"
      context={context}
      onReady={handleReady}
      url={(name, args = {}) => {
        if (name === 'contest_detail') return `/contest/${encodeURIComponent(String(args.tid ?? ''))}`
        if (name === 'contest_create') return '/contest/new'
        if (name === 'contest_team') return '/contest'
        return '/'
      }}
    />
  )
}
