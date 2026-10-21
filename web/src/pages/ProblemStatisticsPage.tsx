// Page markup is rendered from the pinned Hydro ui-default template
// (problem_statistics.html). This file supplies the data, the language filter
// and the two echarts instances the upstream page builds.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  getProblem, getProblemLanguages, getProblemStatistics,
  type ProblemDetail, type ProblemStatisticsData, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { getScoreColor } from '../upstream/status.ts'

const PERMS = {
  PERM_EDIT_PROBLEM: 1,
  PERM_EDIT_PROBLEM_SELF: 2,
  PERM_SUBMIT_PROBLEM: 3,
  PERM_READ_PROBLEM_DATA: 4,
  PERM_VIEW_DISCUSSION: 5,
  PERM_REJUDGE_PROBLEM: 7,
  PERM_VIEW_PROBLEM_SOLUTION: 8,
  PERM_VIEW_USER_PRIVATE_INFO: 9,
} as const

const PAGE_SIZE = 20

function configSummary(problem: ProblemDetail) {
  return {
    type: problem.problemType || 'default',
    timeMin: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    timeMax: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    memoryMin: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
    memoryMax: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
  }
}

export function ProblemStatisticsPage() {
  const { pid = '' } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [data, setData] = useState<ProblemStatisticsData | null>(null)
  const [languages, setLanguages] = useState<Array<{ id: string; display: string }>>([])
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  const params = new URLSearchParams(location.search)
  const page = Math.max(1, Number(params.get('page')) || 1)
  const sort = params.get('sort') || 'time'
  const direction = Number(params.get('direction')) === -1 ? -1 : 1
  const lang = params.get('lang') || ''

  useEffect(() => {
    setError('')
    Promise.all([
      getProblem(pid),
      getProblemStatistics(pid, { page, sort, direction, lang }),
      getProblemLanguages(pid).catch(() => []),
    ])
      .then(([pdoc, statistics, langs]) => {
        setProblem(pdoc)
        setData(statistics)
        setLanguages(langs)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '统计加载失败'))
  }, [direction, lang, page, pid, sort])

  const context = useMemo(() => {
    if (!problem || !data) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const pdoc = {
      ...problem,
      tag: problem.tags ?? [],
      config: configSummary(problem),
      data: [{ name: '__testdata__', size: 0 }],
      reference: false,
    }
    const udict = Object.fromEntries(data.users.map((item: UserChoice) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    }]))
    const langs: Record<string, { display: string }> = Object.fromEntries(
      languages.map((item) => [item.id, { display: item.display }]),
    )
    // Records may use a language outside the problem's configured list (and
    // legacy rows may miss `lang` entirely); keep the upstream template from
    // reading `.display` off undefined.
    langs.unknown = { display: '未知' }
    for (const item of data.items) {
      if (item.lang && !langs[item.lang]) langs[item.lang] = { display: item.lang }
    }
    return {
      page_name: 'problem_statistics',
      pdoc,
      psdoc: { status: 0 },
      rsdocs: data.items.map((item) => ({
        ...item,
        _id: item.id,
        lang: item.lang || 'unknown',
        length: item.length ?? 0,
      })),
      udict,
      page,
      pcount: Math.max(0, Math.ceil(data.total / PAGE_SIZE)),
      sort: data.sort,
      direction: data.direction,
      types: data.sorts,
      tdoc: null,
      handler: {
        user: {
          _id: user?.id ?? 0,
          uname: user?.username ?? '',
          avatar: user?.avatar ?? '',
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => {
            if (permission === PERMS.PERM_VIEW_PROBLEM_SOLUTION) return true
            if (permission === PERMS.PERM_VIEW_USER_PRIVATE_INFO) return Boolean(user)
            if (permission === PERMS.PERM_VIEW_DISCUSSION) return false
            if (permission === PERMS.PERM_EDIT_PROBLEM || permission === PERMS.PERM_REJUDGE_PROBLEM) return isAdmin
            const loggedIn = [PERMS.PERM_EDIT_PROBLEM_SELF, PERMS.PERM_SUBMIT_PROBLEM, PERMS.PERM_READ_PROBLEM_DATA] as number[]
            return Boolean(user) && loggedIn.includes(permission)
          },
          hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
          own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
        },
        args: { domainId: 'system' },
        request: { query: { page, sort, direction, lang }, path: `/p/${pid}/statistics` },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_READ_PROBLEM_DATA: 2, PRIV_REGISTER_USER: 3 },
      STATUS: { STATUS_ACCEPTED: 1, STATUS_TIME_LIMIT_EXCEEDED: 3, STATUS_MEMORY_LIMIT_EXCEEDED: 4, STATUS_OUTPUT_LIMIT_EXCEEDED: 5 },
      model: {
        builtin: { LEVELS: {}, STATUS_CODES: {}, STATUS_TEXTS: {} },
        setting: { langs },
        system: { get: (key: string) => key === 'server.login' },
        contest: { canShowSelfRecord: () => true },
      },
      lib: { difficulty: () => 0 },
      avatarUrl,
      utils: { getAlphabeticId: () => '' },
      // problem_statistics.html starts with set(UiContext, 'pdoc', pdoc).
      UiContext: {
        cdn_prefix: '/',
        pdoc,
        problemId: problem.pid || String(problem.docId),
        problemNumId: problem.docId,
        ws_prefix: '/',
      },
    }
  }, [data, direction, lang, languages, page, pid, problem, sort, user])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = encodeURIComponent(String(args.pid ?? pid))
    const query = (args.query ?? {}) as Record<string, unknown>
    const routes: Record<string, string> = {
      problem_main: query.q ? `/p?q=${encodeURIComponent(String(query.q))}` : '/p',
      problem_detail: `/p/${id}`,
      problem_submit: `/p/${id}/submit`,
      problem_files: `/p/${id}/files`,
      problem_config: `/p/${id}/config`,
      problem_edit: `/p/${id}/edit`,
      problem_solution: `/p/${id}/solution`,
      problem_statistics: `/p/${id}/statistics`,
      record_main: `/record${query.pid ? `?pid=${encodeURIComponent(String(query.pid))}` : ''}`,
      record_detail: `/record/${encodeURIComponent(String(args.rid ?? ''))}`,
      user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
      discussion_node: '/discuss',
    }
    return routes[name] ?? '/'
  }, [pid])

  function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    if (!(form instanceof HTMLFormElement)) return
    const formData = new FormData(form)
    const next = new URLSearchParams()
    const sortValue = String(formData.get('sort') ?? '')
    const directionValue = String(formData.get('direction') ?? '1')
    const langValue = String(formData.get('lang') ?? '')
    if (sortValue) next.set('sort', sortValue)
    if (directionValue) next.set('direction', directionValue)
    if (langValue) next.set('lang', langValue)
    navigate(`/p/${encodeURIComponent(pid)}/statistics${next.toString() ? `?${next.toString()}` : ''}`)
  }
  submitRef.current = (event) => handleSubmit(event)

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const form = root.querySelector<HTMLFormElement>('form[name="filter-form"]')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    if (form) cleanups.push(() => form.removeEventListener('submit', onSubmit))
    form?.querySelectorAll('select').forEach((select) => {
      const onChange = () => form.requestSubmit()
      select.addEventListener('change', onChange)
      cleanups.push(() => select.removeEventListener('change', onChange))
    })
    const langSelect = root.querySelector<HTMLSelectElement>('select[name="lang"]')
    if (langSelect) {
      for (const language of languages) {
        const option = document.createElement('option')
        option.value = language.id
        option.textContent = language.display
        langSelect.appendChild(option)
      }
      langSelect.value = lang
    }
    const statusElement = root.querySelector<HTMLElement>('#submission-status-placeholder')
    const scoreElement = root.querySelector<HTMLElement>('#submission-score-placeholder')
    const chartSection = statusElement?.closest('.section')
    if (chartSection && (data?.total ?? 0) === 0) {
      chartSection.remove()
    } else if (statusElement && scoreElement && data) {
      let disposed = false
      let charts: Array<{ dispose: () => void; resize: () => void }> = []
      const onResize = () => charts.forEach((chart) => chart.resize())
      void import('echarts').then((echarts) => {
        if (disposed) return
        const stats = data.stats
        const statusChart = echarts.init(statusElement)
        statusChart.setOption({
          tooltip: { trigger: 'item' },
          series: [{
            name: 'Submissions',
            type: 'pie',
            radius: '70%',
            label: { show: false },
            labelLine: { show: false },
            data: [
              { value: stats.TLE ?? 0, name: 'TLE' },
              { value: stats.AC ?? 0, name: 'AC' },
              { value: stats.MLE ?? 0, name: 'MLE' },
              { value: stats.WA ?? 0, name: 'WA' },
              { value: stats.RE ?? 0, name: 'RE' },
              { value: stats.CE ?? 0, name: 'CE' },
            ],
          }],
        })
        const scores = Array.from({ length: 101 }, (_, index) => index)
          .filter((index) => (stats[`s${index}`] ?? 0) > 0)
        const scoreChart = echarts.init(scoreElement)
        scoreChart.setOption({
          tooltip: { trigger: 'item' },
          xAxis: { data: scores },
          yAxis: {},
          series: [{
            type: 'bar',
            data: scores.map((index) => ({
              value: stats[`s${index}`],
              itemStyle: { color: getScoreColor(index) },
            })),
          }],
        })
        if (configSummary(problem ?? ({} as ProblemDetail)).type === 'objective') {
          statusElement.style.display = 'none'
          statusChart.resize()
        }
        charts = [statusChart, scoreChart]
        window.addEventListener('resize', onResize)
      }).catch(() => undefined)
      cleanups.push(() => {
        disposed = true
        window.removeEventListener('resize', onResize)
        charts.forEach((chart) => chart.dispose())
      })
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [data, lang, languages, problem])

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!problem || !data || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <HydroTemplate
      template="problem_statistics.html"
      context={context}
      url={buildUrl}
      onReady={handleReady}
    />
  )
}
