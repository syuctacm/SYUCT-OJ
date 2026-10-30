// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to query state, data mapping and small interactions.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { getHomeworks, type Contest } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import Calendar from '../upstream/calendar.ts'

function pad(value: number) {
  return String(value).padStart(2, '0')
}

function colorIndexFor(docId: string) {
  const timestamp = Number.parseInt(docId.slice(0, 8), 16)
  return Number.isFinite(timestamp) ? timestamp % 12 : 0
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

export function HomeworksPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const group = searchParams.get('group') ?? ''
  const page = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [items, setItems] = useState<Contest[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    setError('')
    getHomeworks(query, page)
      .then((data) => { setItems(data.items); setTotal(data.total) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
      .finally(() => setLoading(false))
  }, [query, page])

  const calendarEvents = useMemo(() => items.map((homework) => ({
    beginAt: homework.beginAt,
    endAt: homework.endAt,
    title: homework.title,
    maskFrom: homework.penaltySince ?? null,
    maskTitle: '延期',
    colorIndex: colorIndexFor(homework.docId),
    link: `/homework/${encodeURIComponent(homework.docId)}`,
  })), [items])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const form = root.querySelector<HTMLFormElement>('form[name="filter-form"]')
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      const data = new FormData(form!)
      const params = new URLSearchParams()
      for (const key of ['q', 'group']) {
        const value = String(data.get(key) ?? '').trim()
        if (value) params.set(key, value)
      }
      navigate(`/homework${params.toString() ? `?${params.toString()}` : ''}`)
    }
    form?.addEventListener('submit', onSubmit)
    cleanups.push(() => form?.removeEventListener('submit', onSubmit))

    const list = root.querySelector<HTMLElement>('.homework__list')
    const calendarEntry = root.querySelector<HTMLElement>('[name="calendar_entry"]')
    const displaySelect = root.querySelector<HTMLSelectElement>('[name="homework_display"]')
    let calendar: Calendar | null = null
    if (calendarEntry && calendarEvents.length) {
      calendar = new Calendar(calendarEvents)
      calendar.getDom().appendTo(calendarEntry)
      const applyDisplay = (view: string) => {
        const showCalendar = view === 'calendar'
        if (list) list.style.display = showCalendar ? 'none' : ''
        calendarEntry.style.display = showCalendar ? '' : 'none'
        if (displaySelect) displaySelect.value = showCalendar ? 'calendar' : 'list'
      }
      applyDisplay(localStorage.getItem('homework-view') ?? 'list')
      const onChange = () => {
        const view = displaySelect?.value === 'calendar' ? 'calendar' : 'list'
        applyDisplay(view)
        localStorage.setItem('homework-view', view)
      }
      displaySelect?.addEventListener('change', onChange)
      cleanups.push(() => displaySelect?.removeEventListener('change', onChange))
      cleanups.push(() => calendar?.getDom().remove())
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [calendarEvents, navigate])

  const context = useMemo(() => {
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const tdocs = items.map((homework) => ({
      ...homework,
      beginAt: new Date(homework.beginAt),
      endAt: new Date(homework.endAt),
      penaltySince: new Date(homework.penaltySince || homework.endAt),
      domainId: 'system',
    }))
    const params = new URLSearchParams()
    if (query) params.set('q', query)
    if (group) params.set('group', group)
    return {
      page_name: 'homework_main',
      tdocs,
      calendar: calendarEvents,
      q: query,
      groups: [],
      group,
      page,
      tpcount: Math.max(1, Math.ceil(total / 20)),
      qs: params.toString(),
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
        request: { query: { q: query, group }, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_CREATE_HOMEWORK: 1 },
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: {
        contest: {
          statusText: (homework: { beginAt: Date; endAt: Date }) => {
            const now = Date.now()
            if (now < homework.beginAt.getTime()) return 'Ready (☆▽☆)'
            if (now > homework.endAt.getTime()) return 'Done'
            return 'Live...'
          },
          isExtended: (homework: { penaltySince: Date; endAt: Date }) => Date.now() > homework.penaltySince.getTime() && Date.now() <= homework.endAt.getTime(),
          isDone: (homework: { endAt: Date }) => Date.now() > homework.endAt.getTime(),
        },
      },
      datetimeSpan: (value: Date | string, _relative?: boolean, pattern = 'YYYY-M-D H:mm') => {
        const date = value instanceof Date ? value : new Date(value)
        return `<span class="time">${formatByPattern(date, pattern)}</span>`
      },
      avatarUrl,
      UiContext: { docs: calendarEvents },
      UserContext: user,
    }
  }, [calendarEvents, group, items, query, user])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>

  return (
    <HydroTemplate
      template="homework_main.html"
      context={context}
      onReady={handleReady}
      url={(name, args = {}) => {
        if (name === 'homework_detail') return `/homework/${encodeURIComponent(String(args.tid ?? ''))}`
        if (name === 'homework_create') return '/homework/new'
        return '/'
      }}
    />
  )
}
