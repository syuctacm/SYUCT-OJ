// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to query state, data mapping and the page's interactions.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import {
  deleteProblem,
  getProblemEditor,
  getProblemTags,
  getProblems,
  updateProblem,
  type ProblemPage,
  type ProblemTagStat,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { notifyError, notifySuccess } from '../components/Notification.tsx'
import { statusClass, statusLabels } from '../status.ts'
import { avatarUrl } from '../upstream/avatar.ts'
import { ProblemSelection } from '../upstream/problem-selection.tsx'
import { clearRendered, renderInto } from '../upstream/roots.ts'

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

type BatchOperation = 'hide' | 'unhide' | 'delete'

async function setProblemHidden(pid: string, hidden: boolean) {
  const editor = await getProblemEditor(pid)
  const problem = editor.problem
  await updateProblem(pid, {
    pid: problem.pid,
    title: problem.title,
    content: problem.content,
    tags: problem.tags ?? [],
    difficulty: problem.difficulty ?? 0,
    hidden,
    config: editor.config,
  })
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
  const [tagStats, setTagStats] = useState<ProblemTagStat[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const selectedPidsRef = useRef<string[]>([])
  const clearSelectionRef = useRef<() => void>(() => {})
  const operationBusyRef = useRef(false)

  const load = useCallback(async () => {
    const [problems, tags] = await Promise.all([
      getProblems(query, pageNumber, sort, tag),
      getProblemTags(),
    ])
    setPage(problems)
    setTagStats(tags)
  }, [pageNumber, query, sort, tag])

  useEffect(() => {
    setLoading(true)
    setError('')
    load()
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
      .finally(() => setLoading(false))
  }, [load])

  useEffect(() => () => {
    document.body.classList.remove('display-mode', 'edit-mode')
  }, [])

  const ppcount = page ? Math.max(1, Math.ceil(page.total / page.limit)) : 0

  const handleSelectionChange = useCallback((pids: string[]) => {
    selectedPidsRef.current = pids
  }, [])

  const handleSelectionClear = useCallback((handler: () => void) => {
    clearSelectionRef.current = handler
  }, [])

  const handleOperation = useCallback(async (operation: BatchOperation) => {
    if (operationBusyRef.current) return
    const pids = [...selectedPidsRef.current]
    if (!pids.length) {
      notifyError('请至少选择一道题目。')
      return
    }
    if (operation === 'delete' && !(await confirmDialog(`确认删除选中的 ${pids.length} 道题目？`))) {
      return
    }
    operationBusyRef.current = true
    try {
      const results = await Promise.allSettled(pids.map(async (pid) => {
        if (operation === 'delete') await deleteProblem(pid)
        else await setProblemHidden(pid, operation === 'hide')
      }))
      const failures = results.filter((result): result is PromiseRejectedResult => result.status === 'rejected')
      clearSelectionRef.current()
      selectedPidsRef.current = []
      await load()
      if (failures.length) {
        const message = failures[0].reason instanceof Error ? failures[0].reason.message : String(failures[0].reason)
        notifyError(`${failures.length} 道题目操作失败：${message}`)
        return
      }
      const labels: Record<BatchOperation, string> = {
        hide: '已隐藏',
        unhide: '已取消隐藏',
        delete: '已删除',
      }
      notifySuccess(`${labels[operation]} ${pids.length} 道题目。`)
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '批量操作失败')
    } finally {
      operationBusyRef.current = false
    }
  }, [load])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const listen = (element: Element, type: string, listener: EventListener) => {
      element.addEventListener(type, listener)
      cleanups.push(() => element.removeEventListener(type, listener))
    }

    if (!document.body.classList.contains('display-mode') && !document.body.classList.contains('edit-mode')) {
      document.body.classList.add('display-mode')
    }
    root.querySelector('.section.display-mode')?.classList.remove('display-mode')

    const enterEdit = root.querySelector<HTMLElement>('[name="enter-edit-mode"]')
    const leaveEdit = root.querySelector<HTMLElement>('[name="leave-edit-mode"]')
    const onEnterEdit = (event: Event) => {
      event.preventDefault()
      document.body.classList.remove('display-mode')
      document.body.classList.add('edit-mode')
    }
    const onLeaveEdit = (event: Event) => {
      event.preventDefault()
      document.body.classList.remove('edit-mode')
      document.body.classList.add('display-mode')
    }
    if (enterEdit) listen(enterEdit, 'click', onEnterEdit)
    if (leaveEdit) listen(leaveEdit, 'click', onLeaveEdit)

    for (const [name, operation] of [
      ['hide_selected_problems', 'hide'],
      ['unhide_selected_problems', 'unhide'],
      ['delete_selected_problems', 'delete'],
    ] as const) {
      const button = root.querySelector<HTMLElement>(`[name="${name}"]`)
      if (!button) continue
      listen(button, 'click', (event) => {
        event.preventDefault()
        void handleOperation(operation)
      })
    }

    const selection = root.querySelector<HTMLElement>('#problem_selection')
    if (selection) {
      renderInto(
        selection,
        <ProblemSelection
          initialPids={selectedPidsRef.current}
          onChange={handleSelectionChange}
          onClear={handleSelectionClear}
        />,
      )
      cleanups.push(() => clearRendered(selection))
    }

    const form = root.querySelector<HTMLFormElement>('form#searchForm')
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      const data = new FormData(form!)
      const params = new URLSearchParams()
      const q = String(data.get('q') ?? '').trim()
      const nextSort = String(data.get('sort') ?? 'default')
      if (q.startsWith('category:')) {
        const nextTag = q.slice('category:'.length).trim()
        navigate(`/p${nextTag ? `?tag=${encodeURIComponent(nextTag)}` : ''}`)
        return
      }
      if (q) params.set('q', q)
      if (tag) params.set('tag', tag)
      if (nextSort === 'recent') params.set('sort', 'recent')
      navigate(`/p${params.toString() ? `?${params.toString()}` : ''}`)
    }
    form?.addEventListener('submit', onSubmit)
    cleanups.push(() => form?.removeEventListener('submit', onSubmit))
    const sortSelect = root.querySelector<HTMLSelectElement>('select[name="sort"]')
    const onSort = () => form?.requestSubmit()
    sortSelect?.addEventListener('change', onSort)
    cleanups.push(() => sortSelect?.removeEventListener('change', onSort))

    const toggleProblemTags = () => {
      const table = root.querySelector<HTMLElement>('.section.display-mode table.data-table')
      if (table) table.classList.toggle('hide-problem-tag')
    }
    const tagButtons = Array.from(root.querySelectorAll<HTMLElement>('.toggle-tag'))
    tagButtons.forEach((button) => listen(button, 'click', toggleProblemTags))

    const tagStatToggle = root.querySelector<HTMLAnchorElement>('#tagStatToggleBtn')
    const tagStatExtras = Array.from(root.querySelectorAll<HTMLElement>('.tag-stat-row.tag-stat-extra'))
    const onTagStatToggle = (event: Event) => {
      event.preventDefault()
      if (!tagStatToggle) return
      const expanded = tagStatToggle.dataset.expanded === 'true'
      tagStatExtras.forEach((row) => { row.style.display = expanded ? 'none' : 'table-row' })
      tagStatToggle.dataset.expanded = expanded ? 'false' : 'true'
      const total = root.querySelectorAll('.tag-stat-row').length
      tagStatToggle.textContent = expanded ? `展开全部标签 (${total})` : '收起部分标签'
    }
    if (tagStatToggle) listen(tagStatToggle, 'click', onTagStatToggle)

    return () => cleanups.forEach((cleanup) => cleanup())
  }, [handleOperation, handleSelectionChange, handleSelectionClear, navigate, tag])

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
      qs: tag ? `category:${tag}` : query,
      tagStats,
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
          hasPerm: (permission: number) => isAdmin && (permission === 1 || permission === 2),
          hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
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
  }, [page, ppcount, query, sort, tag, tagStats, user])

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
        if (name === 'problem_tag_stat') return `/p/tag/${encodeURIComponent(String(args.tag ?? ''))}`
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
