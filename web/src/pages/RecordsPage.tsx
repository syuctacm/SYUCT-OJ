// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping, filters and pagination state.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { getSubmissions, type SubmissionEvent, type SubmissionPage } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { languageOptions } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { statusClass, statusLabels } from '../status.ts'
import { avatarUrl } from '../upstream/avatar.ts'
import { loadLanguages } from '../upstream/languages.ts'

const languageMap = Object.fromEntries(languageOptions.map((language) => [
  language._id,
  { display: language.name, highlight: language._id },
]))

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

function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

export function RecordsPage() {
  const { user, loading: authLoading } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const filters = {
    uidOrName: searchParams.get('uidOrName') ?? '',
    pid: searchParams.get('pid') ?? '',
    tid: searchParams.get('tid') ?? '',
    lang: searchParams.get('lang') ?? '',
    status: searchParams.get('status') ?? '',
    page: Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1),
  }
  const [page, setPage] = useState<SubmissionPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (authLoading) return
    if (!user) {
      setLoading(false)
      return
    }
    setLoading(true)
    setError('')
    getSubmissions(filters)
      .then(setPage)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '提交记录加载失败'))
      .finally(() => setLoading(false))
  }, [authLoading, user, filters.uidOrName, filters.pid, filters.tid, filters.lang, filters.status, filters.page])

  useEffect(() => {
    if (!user) return
    let disposed = false
    let socket: WebSocket | null = null
    let reconnect: number | undefined
    const connect = () => {
      if (disposed) return
      const url = new URL('/api/v1/submissions/events', window.location.href)
      url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
      socket = new WebSocket(url)
      socket.onmessage = (message) => {
        try {
          const event = JSON.parse(String(message.data)) as SubmissionEvent
          setPage((current) => {
            if (!current) return current
            let changed = false
            const items = current.items.map((item) => {
              if (item.id !== event.rid) return item
              changed = true
              return {
                ...item,
                status: event.status,
                score: event.score,
                time: event.time,
                memory: event.memory,
                judgeAt: event.judgeAt,
              }
            })
            return changed ? { ...current, items } : current
          })
        } catch {
          // Ignore malformed event frames.
        }
      }
      socket.onclose = () => {
        if (!disposed) reconnect = window.setTimeout(connect, 3000)
      }
    }
    connect()
    return () => {
      disposed = true
      if (reconnect) window.clearTimeout(reconnect)
      socket?.close()
    }
  }, [user])

  const hasMore = Boolean(page && page.page * page.limit < page.total)

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form[method="get"]')
    const langSelect = root.querySelector<HTMLSelectElement>('select[name="lang"]')
    if (langSelect) {
      const fill = (items: Array<{ _id: string; name: string }>) => {
        langSelect.replaceChildren()
        const all = document.createElement('option')
        all.value = ''
        all.textContent = '所有语言'
        langSelect.appendChild(all)
        for (const language of items) {
          const option = document.createElement('option')
          option.value = language._id
          option.textContent = language.name
          langSelect.appendChild(option)
        }
        langSelect.value = filters.lang
      }
      fill(languageOptions)
      void loadLanguages().then(fill)
    }
    if (!hasMore) root.querySelector('.pager')?.remove()
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      const data = new FormData(form!)
      const params = new URLSearchParams()
      for (const key of ['uidOrName', 'pid', 'tid', 'lang', 'status']) {
        const value = String(data.get(key) ?? '').trim()
        if (value) params.set(key, value)
      }
      navigate(`/record${params.toString() ? `?${params.toString()}` : ''}`)
    }
    form?.addEventListener('submit', onSubmit)
    return () => form?.removeEventListener('submit', onSubmit)
  }, [filters.lang, hasMore, navigate])

  const context = useMemo(() => {
    if (!page) return null
    const rdocs = page.items.map((submission) => ({
      _id: submission.id,
      uid: submission.uid,
      pid: submission.pid,
      status: submission.status,
      score: submission.score,
      time: submission.time,
      memory: submission.memory,
      lang: submission.lang,
      contest: null,
      judgeAt: submission.judgeAt,
      createdAt: submission.createdAt,
      files: {},
    }))
    const udict = Object.fromEntries(page.items.map((submission) => [
      String(submission.uid),
      {
        _id: submission.uid,
        uname: submission.submitter?.displayName || submission.submitter?.username || `UID ${submission.uid}`,
        avatar: '',
        hasPriv: () => false,
        hasPerm: () => false,
      },
    ]))
    const pdict = Object.fromEntries(page.items.map((submission) => [
      String(submission.pid),
      submission.problem ? {
        docId: submission.problem.docId,
        pid: submission.problem.pid,
        title: submission.problem.title,
        hidden: false,
        tag: [],
      } : null,
    ]))
    const dates = new Map(page.items.map((submission) => [submission.id, submission.createdAt]))
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      page_name: 'record_main',
      rdocs,
      udict,
      pdict,
      page: page.page,
      filterUidOrName: filters.uidOrName,
      filterPid: filters.pid,
      filterTid: filters.tid,
      filterLang: filters.lang,
      filterStatus: filters.status ? Number(filters.status) : '',
      all: false,
      allDomain: false,
      statistics: null,
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: () => false,
          hasPriv: (privilege: number) => Boolean(user) && privilege === 1 && hasMore,
          own: () => Boolean(user && isAdmin),
        },
        args: { domainId: 'system' },
        request: { query: filters, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_REJUDGE: 1, PERM_VIEW_PROBLEM_HIDDEN: 2, PERM_VIEW_USER_PRIVATE_INFO: 3, PERM_MOD_BADGE: 4 },
      PRIV: { PRIV_VIEW_JUDGE_STATISTICS: 1, PRIV_USER_PROFILE: 2, PRIV_READ_PROBLEM_DATA: 3 },
      STATUS: {
        STATUS_ACCEPTED: 1,
        STATUS_JUDGING: 20,
        STATUS_TIME_LIMIT_EXCEEDED: 3,
        STATUS_MEMORY_LIMIT_EXCEEDED: 4,
        STATUS_OUTPUT_LIMIT_EXCEEDED: 5,
      },
      model: {
        builtin: {
          LEVELS: {},
          STATUS_CODES: statusCodes,
          STATUS_TEXTS: statusLabels,
          STATUS_SHORT_TEXTS: statusShortTexts,
        },
        setting: { langs: languageMap },
        system: { get: () => false },
      },
      utils: {
        status: {
          STATUS_TEXTS: statusLabels,
          getScoreColor: (score: number) => score >= 100 ? '#25ad40' : score > 0 ? '#e64c72' : '#999',
        },
      },
      datetimeSpan: (id: string) => `<span class="time">${formatDate(dates.get(String(id)) ?? '')}</span>`,
      size: (bytes: number, base = 1) => {
        let value = bytes * base
        const units = ['Bytes', 'KiB', 'MiB', 'GiB']
        let index = 0
        while (value >= 1024 && index < units.length - 1) {
          value /= 1024
          index++
        }
        return `${Math.round(value * 10) / 10} ${units[index]}`
      },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [filters.lang, filters.pid, filters.status, filters.tid, filters.uidOrName, hasMore, page, user])

  if (authLoading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!user) return <div className="row"><div className="columns"><div className="nothing-placeholder">请先 <Link to="/login">登录</Link> 查看评测记录。</div></div></div>
  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!context) return null

  return (
    <HydroTemplate
      template="record_main.html"
      context={context}
      onReady={handleReady}
      url={(name, args = {}) => {
        const query = (args.query ?? {}) as Record<string, unknown>
        if (name === 'record_detail') return `/record/${encodeURIComponent(String(args.rid ?? ''))}`
        if (name === 'record_main') {
          const params = new URLSearchParams()
          for (const [key, value] of Object.entries(query)) if (value !== undefined && value !== '') params.set(key, String(value))
          return `/record${params.toString() ? `?${params.toString()}` : ''}`
        }
        if (name === 'problem_detail') return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
        if (name === 'user_detail') return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
        return '/'
      }}
    />
  )
}
