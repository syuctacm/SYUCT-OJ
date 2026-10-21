// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping and request handling.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  getContestProblems, getContestScoreboard, getUsersByIDs,
  type Contest, type ProblemSummary, type ScoreboardRow, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { readScoreboardStars, writeScoreboardStars } from '../upstream/scoreboard-star.ts'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'
import { translate } from '../upstream/render.ts'
import { unlockContest } from '../api.ts'
import { avatarUrl } from '../upstream/avatar.ts'

type ScoreboardColumn = {
  type: string
  value: string
  raw?: string
  hover?: string
  score?: number
  style?: string
}

function pad(value: number) {
  return value < 10 ? `0${value}` : String(value)
}

// Derived from `formatSeconds` in Hydro `framework/utils/lib/common.ts`.
function formatSeconds(seconds: number, showSeconds = true) {
  const value = Math.max(0, Math.floor(seconds))
  let result = `${showSeconds ? pad(Math.floor(value / 3600)) : Math.floor(value / 3600)}:${pad(Math.floor((value % 3600) / 60))}`
  if (showSeconds) result += `:${pad(value % 60)}`
  return result
}

export function ContestScoreboardPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [rows, setRows] = useState<ScoreboardRow[]>([])
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    setContest(null)
    setError('')
    getContestScoreboard(tid)
      .then((data) => { setContest(data.contest); setRows(data.rows) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '榜单加载失败'))
    getContestProblems(tid).then((data) => setProblems(data.problems)).catch(() => setProblems([]))
  }, [tid, reloadKey])

  useEffect(() => {
    if (!contest) return
    const uids = [...new Set(rows.map((row) => row.uid))].filter((uid) => uid > 0)
    getUsersByIDs(uids).then(setUsers).catch(() => setUsers([]))
  }, [contest, rows])

  const tdoc = useMemo(() => {
    if (!contest) return null
    return {
      ...contest,
      _id: contest.docId,
      beginAt: new Date(contest.beginAt),
      endAt: new Date(contest.endAt),
      lockAt: contest.lockAt ? new Date(contest.lockAt) : null,
      penaltySince: contest.penaltySince ? new Date(contest.penaltySince) : null,
      pids: contest.pids ?? [],
      domainId: 'system',
    }
  }, [contest])

  const udict = useMemo(() => Object.fromEntries(users.map((item) => [String(item.id), {
    _id: item.id,
    uname: item.username,
    displayName: item.displayName ?? '',
    avatar: item.avatar ?? '',
  }])), [users])

  const scoreboardRows = useMemo(() => {
    if (!contest) return []
    const pids = contest.pids ?? []
    const stats: Record<string, { accept: number, submit: number }> = {}
    for (const pid of pids) stats[String(pid)] = { accept: 0, submit: 0 }
    for (const row of rows) {
      for (const pid of pids) {
        const cell = row.problems[String(pid)]
        if (!cell) continue
        stats[String(pid)].submit += cell.attempts + (cell.solved ? 1 : 0)
        if (cell.solved) stats[String(pid)].accept += 1
      }
    }
    const header: ScoreboardColumn[] = [
      { type: 'rank', value: '#' },
      { type: 'user', value: translate('User') },
      { type: 'solved', value: `${translate('Solved')}\n${translate('Total Time')}` },
      ...pids.map((pid, index) => ({
        type: 'problem',
        value: String.fromCharCode(65 + index),
        raw: String(pid),
      })),
    ]
    const scoreBased = ['oi', 'ioi', 'strictioi'].includes(contest?.rule ?? '')
    const body = rows.map((row) => {
      const columns: ScoreboardColumn[] = [
        { type: 'rank', value: String(row.rank) },
        { type: 'user', value: udict[String(row.uid)]?.uname ?? `UID ${row.uid}`, raw: String(row.uid) },
        {
          type: 'time',
          value: `${row.solved}\n${formatSeconds(row.penalty * 60, false)}`,
          hover: formatSeconds(row.penalty * 60),
        },
      ]
      for (const pid of pids) {
        const cell = row.problems[String(pid)]
        if (!cell) {
          columns.push({ type: 'record', value: '' })
          continue
        }
        // Score based rules show the score itself, XCPC keeps the solved/penalty cell.
        if (scoreBased) {
          const configured = contest?.score?.[String(pid)]
          const score = cell.score > 0 ? cell.score : (cell.solved ? (configured ?? 100) : 0)
          columns.push({
            type: 'record',
            score,
            value: String(score),
            hover: cell.solved ? formatSeconds(cell.time * 60) : '',
            raw: cell.rid ?? '',
          })
          continue
        }
        const attempts = cell.attempts
        let value = attempts ? `-${attempts}` : ''
        if (cell.solved) {
          value = `${attempts ? `+${attempts}` : '<span class="icon icon-check"></span>'}\n${formatSeconds(cell.time * 60, false)}`
        }
        columns.push({
          type: 'record',
          score: cell.solved ? 100 : 0,
          value,
          hover: cell.solved ? formatSeconds(cell.time * 60) : '',
          raw: cell.rid ?? '',
        })
      }
      return columns
    })
    return [header, ...body]
  }, [contest, rows, udict])

  const pdict = useMemo(() => {
    const result: Record<string, Record<string, unknown>> = {}
    const stats: Record<string, { accept: number, submit: number }> = {}
    for (const row of rows) {
      for (const [pid, cell] of Object.entries(row.problems)) {
        const item = stats[pid] ?? { accept: 0, submit: 0 }
        item.submit += cell.attempts + (cell.solved ? 1 : 0)
        if (cell.solved) item.accept += 1
        stats[pid] = item
      }
    }
    for (const problem of problems) {
      const stat = stats[String(problem.docId)] ?? { accept: 0, submit: 0 }
      result[String(problem.docId)] = {
        docId: problem.docId,
        pid: problem.pid,
        title: problem.title,
        difficulty: problem.difficulty,
        nAccept: stat.accept,
        nSubmit: stat.submit,
      }
    }
    return result
  }, [problems, rows])

  const context = useMemo(() => {
    if (!contest || !tdoc) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const canEdit = Boolean(user && (user.id === contest.owner || isAdmin))
    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (permission === CONTEST_PERMS.PERM_VIEW_USER_PRIVATE_INFO) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD) return isAdmin
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD) return isAdmin
          if (permission === CONTEST_PERMS.PERM_EDIT_CONTEST) return canEdit
          if (permission === CONTEST_PERMS.PERM_READ_RECORD_CODE) return canEdit
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
      },
      args: { domainId: 'system' },
      request: {
        query: {},
        path: `/contest/${contest.docId}/scoreboard`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    return {
      page_name: 'contest_scoreboard',
      tdoc,
      tsdoc: {},
      rows: scoreboardRows,
      udict,
      pdict,
      groups: [],
      availableViews: [],
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [contest, pdict, scoreboardRows, tdoc, udict, user])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    switch (name) {
      case 'contest_scoreboard': return `/contest/${encodeURIComponent(id)}/scoreboard`
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'record_detail': return `/record/${encodeURIComponent(String(args.rid ?? ''))}`
      case 'problem_detail': {
        const raw = String(args.pid ?? '')
        const problem = problems.find((item) => String(item.docId) === raw || item.pid === raw)
        return `/p/${encodeURIComponent(problem?.pid || raw)}`
      }
      case 'contest_scoreboard_view':
      case 'homework_scoreboard_view': {
        if (args.view === 'html') return `/contest/${encodeURIComponent(id)}/scoreboard`
        if (args.view === 'csv') return '#export-csv'
        return '#export-ghost'
      }
      default: return '/'
    }
  }, [contest?.docId, problems, tid])

  const downloadCSV = useCallback(() => {
    if (!contest) return
    const pids = contest.pids ?? []
    const header = ['Rank', 'User', 'Solved', 'Penalty', ...pids.map((_, index) => String.fromCharCode(65 + index))]
    const lines = [header.join(',')]
    for (const row of rows) {
      const cells = [
        String(row.rank),
        udict[String(row.uid)]?.uname ?? String(row.uid),
        String(row.solved),
        String(row.penalty),
      ]
      for (const pid of pids) {
        const cell = row.problems[String(pid)]
        let value = ''
        if (cell?.solved) value = `AC(${cell.attempts})/${cell.time}`
        else if (cell?.attempts) value = `-${cell.attempts}`
        cells.push(value)
      }
      lines.push(cells.join(','))
    }
    const blob = new Blob([`\ufeff${lines.join('\n')}`], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `scoreboard-${contest.docId}.csv`
    link.click()
    URL.revokeObjectURL(url)
  }, [contest, rows, udict])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const contestKey = `system/${contest?.docId ?? tid}`
    const starButtons = [...root.querySelectorAll<HTMLElement>('.star[data-uid]')]
    const filterSelect = root.querySelector<HTMLSelectElement>('select.select.filter')
    let stars: number[] = []
    let cancelled = false

    const uidOfRow = (row: Element) => Number(row.querySelector('.star[data-uid]')?.getAttribute('data-uid') ?? 0)
    const rowOf = (element: Element | null) => element?.closest('tr') ?? null

    const applyStars = () => {
      for (const star of starButtons) {
        const uid = Number(star.getAttribute('data-uid') ?? 0)
        const active = stars.includes(uid)
        star.classList.toggle('activated', active)
        rowOf(star)?.classList.toggle('star-highlight', active)
      }
      if (user?.id) rowOf(root.querySelector(`.star.user--${user.id}`))?.classList.add('star-highlight')
    }

    const applyFilter = () => {
      if (!filterSelect) return
      const value = filterSelect.value
      const rows = [...root.querySelectorAll<HTMLElement>('.data-table tbody tr')]
      if (value === 'star') {
        for (const row of rows) row.style.display = stars.includes(uidOfRow(row)) ? '' : 'none'
        return
      }
      if (value === 'rank') {
        for (const row of rows) row.style.display = row.querySelector('.rank--unrank') ? 'none' : ''
        return
      }
      if (value === 'all' || !value) {
        for (const row of rows) row.style.display = ''
        return
      }
      const uids = value.split(',').map((item) => Number(item.trim())).filter(Boolean)
      for (const row of rows) row.style.display = uids.includes(uidOfRow(row)) ? '' : 'none'
    }

    const applyHashFilter = () => {
      if (!filterSelect) return
      const hash = location.hash.slice(1)
      if (!hash.startsWith('filter=')) return
      filterSelect.value = hash.slice(7)
      applyFilter()
    }

    void readScoreboardStars(contestKey).then((stored) => {
      if (cancelled) return
      stars = stored
      applyStars()
      applyFilter()
    })

    if (filterSelect) {
      const hash = location.hash.slice(1)
      if (hash.startsWith('filter=')) filterSelect.value = hash.slice(7)
      const onChange = () => {
        applyFilter()
        history.replaceState(null, '', `#filter=${filterSelect.value}`)
      }
      filterSelect.addEventListener('change', onChange)
      cleanups.push(() => filterSelect.removeEventListener('change', onChange))
    }
    const onHashChange = () => applyHashFilter()
    window.addEventListener('hashchange', onHashChange)
    cleanups.push(() => window.removeEventListener('hashchange', onHashChange))

    const onStarClick = (event: Event) => {
      const star = (event.target as HTMLElement).closest<HTMLElement>('.star[data-uid]')
      if (!star) return
      event.preventDefault()
      const uid = Number(star.getAttribute('data-uid') ?? 0)
      if (!uid) return
      stars = stars.includes(uid) ? stars.filter((item) => item !== uid) : [...stars, uid]
      applyStars()
      applyFilter()
      void writeScoreboardStars(contestKey, stars)
    }
    root.addEventListener('click', onStarClick)
    cleanups.push(() => root.removeEventListener('click', onStarClick))

    // The Ghost export comes from an addon this rewrite does not keep.
    root.querySelectorAll('a[href="#export-ghost"]').forEach((link) => link.remove())

    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => {
        const data = new FormData(form)
        if (data.get('operation') !== 'unlock') {
          event.preventDefault()
          return
        }
        event.preventDefault()
        void unlockContest(tid).then(() => setReloadKey((key) => key + 1))
      }
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    const onClick = (event: Event) => {
      const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>('a[href="#export-csv"]')
      if (!anchor) return
      event.preventDefault()
      downloadCSV()
    }
    root.addEventListener('click', onClick)
    cleanups.push(() => root.removeEventListener('click', onClick))
    return () => {
      cancelled = true
      cleanups.forEach((cleanup) => cleanup())
    }
  }, [contest?.docId, downloadCSV, tid, user?.id])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <HydroTemplate
      template="contest_scoreboard.html"
      context={context}
      url={buildUrl}
      onReady={handleReady}
    />
  )
}
