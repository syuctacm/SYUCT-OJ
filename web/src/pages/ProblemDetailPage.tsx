// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and interaction adapters.
import { useCallback, useEffect, useLayoutEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  downloadProblemsArchive,
  getProblem,
  getUsersByIDs,
  rejudgeProblem,
  starProblem,
  type ProblemDetail,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { notifyError } from '../components/Notification.tsx'
import { applyPageMeta } from '../page-meta.ts'
import { activityRoute } from '../upstream/activity-url.ts'
import { avatarUrl } from '../upstream/avatar.ts'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'
import { useHydroContestTimer } from '../upstream/contest-timer.ts'
import { renderMarkdown } from '../upstream/markdown.ts'
import { selectLocalizedContent } from '../upstream/problem-content.ts'

const VIEW_PROBLEM_SOLUTION_PERM = 16

const LANGUAGE_NAMES: Record<string, string> = {
  zh: '中文',
  en: 'English',
  ja: '日本語',
  ko: '한국어',
  ru: 'Русский',
}

const PROBLEM_PERMS = {
  PERM_EDIT_PROBLEM: 1,
  PERM_EDIT_PROBLEM_SELF: 2,
  PERM_SUBMIT_PROBLEM: 3,
  PERM_READ_PROBLEM_DATA: 4,
  PERM_VIEW_DISCUSSION: 5,
  PERM_CREATE_DISCUSSION: 6,
  PERM_REJUDGE_PROBLEM: 7,
  PERM_VIEW_PROBLEM_SOLUTION: 8,
} as const

function configSummary(problem: ProblemDetail) {
  return {
    type: problem.problemType || 'default',
    timeMin: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    timeMax: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    memoryMin: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
    memoryMax: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
  }
}

function appendQuery(path: string, values: Record<string, unknown>) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  }
  return `${path}${params.toString() ? `?${params.toString()}` : ''}`
}

export function ProblemDetailPage() {
  const { pid = '' } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { user } = useAuth()
  const tid = searchParams.get('tid') ?? ''
  const language = searchParams.get('lang') ?? user?.viewLang ?? 'zh'
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [users, setUsers] = useState<Array<{ id: number; username: string; displayName?: string; avatar?: string }>>([])
  const [error, setError] = useState('')
  const timerContest = useMemo(() => problem?.contest ? {
    ...problem.contest,
    status: problem.contestStatus,
  } : null, [problem])
  useHydroContestTimer(timerContest)

  useEffect(() => {
    setProblem(null)
    setError('')
    getProblem(pid, tid)
      .then(setProblem)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
  }, [pid, tid])

  useEffect(() => {
    if (!problem) {
      setUsers([])
      return
    }
    const ids = [...new Set([problem.owner, problem.contest?.owner].filter((id): id is number => Boolean(id)))]
    getUsersByIDs(ids).then(setUsers).catch(() => setUsers([]))
  }, [problem])

  useLayoutEffect(() => {
    if (!problem) return
    const name = !problem.contest
      ? 'problem_detail'
      : problem.contest.rule === 'homework'
        ? 'homework_detail_problem'
        : 'contest_detail_problem'
    applyPageMeta({ name, layout: 'basic', title: problem.title })
  }, [problem])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    root.querySelectorAll('[name="problem-sidebar__copy"]')
      .forEach((element) => element.closest('li')?.remove())
    const download = root.querySelector<HTMLElement>('[name="problem-sidebar__download"]')
    if (download) {
      const onClick = async (event: Event) => {
        event.preventDefault()
        try {
          const blob = await downloadProblemsArchive([pid])
          const url = URL.createObjectURL(blob)
          const anchor = document.createElement('a')
          anchor.href = url
          anchor.download = 'problem.zip'
          document.body.append(anchor)
          anchor.click()
          anchor.remove()
          window.setTimeout(() => URL.revokeObjectURL(url), 1000)
        } catch (err) {
          notifyError(err instanceof Error ? err.message : '题目下载失败')
        }
      }
      download.addEventListener('click', onClick)
      cleanups.push(() => download.removeEventListener('click', onClick))
    }

    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const operation = form.querySelector<HTMLInputElement>('input[name="operation"]')?.value
      if (operation === 'star') {
        const onSubmit = (event: SubmitEvent) => {
          event.preventDefault()
          const next = form.querySelector<HTMLInputElement>('input[name="star"]')?.value === 'true'
          void starProblem(pid, next)
            .then(() => getProblem(pid, tid))
            .then(setProblem)
            .catch((err: unknown) => setError(err instanceof Error ? err.message : '收藏失败'))
        }
        form.addEventListener('submit', onSubmit)
        cleanups.push(() => form.removeEventListener('submit', onSubmit))
      } else if (operation === 'rejudge') {
        const onSubmit = async (event: SubmitEvent) => {
          event.preventDefault()
          if (!(await confirmDialog('确定重测该题的全部提交吗？'))) return
          void rejudgeProblem(pid)
            .then(() => getProblem(pid, tid))
            .then(setProblem)
            .catch((err: unknown) => setError(err instanceof Error ? err.message : '重测失败'))
        }
        form.addEventListener('submit', onSubmit)
        cleanups.push(() => form.removeEventListener('submit', onSubmit))
      }
    }

    root.querySelectorAll<HTMLElement>('[data-lang]').forEach((tab) => {
      const onClick = (event: Event) => {
        event.preventDefault()
        const next = new URL(window.location.href)
        next.searchParams.set('lang', tab.dataset.lang ?? 'zh')
        navigate(`${next.pathname}${next.search}${next.hash}`)
      }
      tab.addEventListener('click', onClick)
      cleanups.push(() => tab.removeEventListener('click', onClick))
    })

    const tagToggle = root.querySelector<HTMLElement>('[name="show_tags"]')
    if (tagToggle) {
      const onClick = () => {
        const tags = root.querySelectorAll<HTMLElement>('.problem__tag-item.tags')
        const visible = tags.length > 0 && !tags[0].classList.contains('hasjs--hide')
        tags.forEach((tag) => {
          tag.classList.toggle('hasjs--hide', visible)
          tag.style.display = visible ? 'none' : ''
        })
      }
      tagToggle.addEventListener('click', onClick)
      cleanups.push(() => tagToggle.removeEventListener('click', onClick))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [navigate, pid, tid])

  const context = useMemo(() => {
    if (!problem) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const contest = problem.contest
    const isHomework = contest?.rule === 'homework'
    const canEditProblem = Boolean(user && (problem.owner === user.id || isAdmin))
    const canEditContest = Boolean(user && contest && (contest.owner === user.id || isAdmin))
    const pdoc = {
      ...problem,
      tag: contest ? [] : problem.tags ?? [],
      config: configSummary(problem),
      // The server-side template only uses this list to decide whether to show a warning.
      data: [{ name: '__testdata__', size: 0 }],
      reference: false,
    }
    const udict = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    }]))
    const ownerID = contest?.owner ?? problem.owner
    const ownerUser = users.find((item) => item.id === ownerID)
    const owner = ownerID ? {
      _id: ownerID,
      uname: ownerUser?.username ?? `UID ${ownerID}`,
      displayName: ownerUser?.displayName ?? '',
      avatar: ownerUser?.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    } : null
    const tdoc = contest ? {
      ...contest,
      _id: contest.docId,
      docType: 30,
      beginAt: new Date(contest.beginAt),
      endAt: new Date(contest.endAt),
      lockAt: contest.lockAt ? new Date(contest.lockAt) : null,
      penaltySince: contest.penaltySince ? new Date(contest.penaltySince) : null,
      pids: contest.pids ?? [],
      domainId: 'system',
    } : null
    const tsdoc = contest ? {
      attend: Boolean(problem.contestStatus?.attend),
      startAt: problem.contestStatus?.startAt ? new Date(problem.contestStatus.startAt) : undefined,
      endAt: problem.contestStatus?.endAt ? new Date(problem.contestStatus.endAt) : undefined,
      detail: problem.contestStatuses ?? {},
    } : null

    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: user?.viewLang || language,
        codeLang: user?.codeLang || 'cpp',
        codeTemplate: user?.codeTemplate ?? '',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (!contest) {
            if (permission === VIEW_PROBLEM_SOLUTION_PERM) return true
            if (permission === PROBLEM_PERMS.PERM_REJUDGE_PROBLEM) return canEditProblem
            return Boolean(user) && [1, 2, 3, 4, 5, 6].includes(permission)
          }
          if (permission === CONTEST_PERMS.PERM_ATTEND_HOMEWORK) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_EDIT_HOMEWORK) return canEditContest
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_SCOREBOARD) return true
          if (permission === CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD) return canEditContest
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD) return canEditContest
          if (permission === CONTEST_PERMS.PERM_EDIT_CONTEST) return canEditContest
          if (permission === CONTEST_PERMS.PERM_READ_RECORD_CODE) return canEditContest || canEditProblem
          if (permission === CONTEST_PERMS.PERM_CREATE_DISCUSSION) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_DISCUSSION) return true
          if (permission === CONTEST_PERMS.PERM_ATTEND_CONTEST) return true
          if (permission === PROBLEM_PERMS.PERM_EDIT_PROBLEM) return isAdmin
          if (permission === PROBLEM_PERMS.PERM_EDIT_PROBLEM_SELF) return canEditProblem
          if (permission === PROBLEM_PERMS.PERM_SUBMIT_PROBLEM) return Boolean(user)
          if (permission === PROBLEM_PERMS.PERM_READ_PROBLEM_DATA) return canEditProblem
          if (permission === PROBLEM_PERMS.PERM_REJUDGE_PROBLEM) return canEditProblem
          return false
        },
        hasPriv: (privilege: number) => {
          if (privilege === 1) return Boolean(user)
          if (privilege === 2) return canEditProblem
          return false
        },
        own: (doc?: unknown) => {
          if (!doc) return canEditProblem
          const ownerID = (doc as { owner?: number }).owner
          return Boolean(user && ownerID === user.id)
        },
      },
      args: { domainId: 'system' },
      request: {
        query: { ...(tid ? { tid } : {}), lang: language },
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: user?.viewLang || language },
      ctx: {
        setting: { get: () => '' },
        i18n: { get: (key: string, value: string) => key === '__langname' ? LANGUAGE_NAMES[value] ?? value : '' },
      },
    }
    const contestModel = contest ? createContestModel(handler) : null
    const target = encodeURIComponent(problem.pid || String(problem.docId))
    const contestQuery = contest ? `?tid=${encodeURIComponent(contest.docId)}` : ''
    const canViewRecord = contestModel && tdoc ? contestModel.canShowSelfRecord(tdoc) : true

    return {
      page_name: !contest
        ? 'problem_detail'
        : isHomework
          ? 'homework_detail_problem'
          : 'contest_detail_problem',
      pdoc,
      tdoc,
      tsdoc,
      udict,
      rdoc: null,
      psdoc: contest ? null : { star: Boolean(problem.starred), status: 0 },
      udoc: owner,
      owner_udoc: owner,
      mode: contest ? problem.contestMode ?? 'contest' : 'normal',
      showAccept: canViewRecord,
      // Imported FPS problemsets keep pre-rendered HTML; the rest is markdown.
      renderredContent: problem.html
        ? selectLocalizedContent(problem.content, language)
        : renderMarkdown(selectLocalizedContent(problem.content, language)),
      handler,
      perm: { ...PROBLEM_PERMS, ...CONTEST_PERMS, PERM_VIEW_PROBLEM_SOLUTION: VIEW_PROBLEM_SOLUTION_PERM },
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_READ_PROBLEM_DATA: 2, PRIV_REGISTER_USER: 3 },
      STATUS: { STATUS_ACCEPTED: 1 },
      model: {
        builtin: { LEVELS: {}, STATUS_CODES: {}, STATUS_TEXTS: {} },
        document: { TYPE_CONTEST: 30 },
        setting: { langs: {} },
        system: { get: (key: string) => key === 'server.login' },
        contest: contestModel,
      },
      lib: { difficulty: () => 0 },
      avatarUrl,
      utils: { getAlphabeticId: (index: number) => String.fromCharCode(65 + index) },
      UiContext: {
        cdn_prefix: '/',
        problemId: problem.pid || String(problem.docId),
        problemNumId: problem.docId,
        codeLang: user?.codeLang || 'cpp',
        codeTemplate: user?.codeTemplate ?? '',
        pdoc,
        ...(contest ? { tdoc, tsdoc } : {}),
        canViewRecord,
        postSubmitUrl: `/p/${target}/submit${contestQuery}`,
        getSubmissionsUrl: `/record?pid=${encodeURIComponent(String(problem.docId))}${contest ? `&tid=${encodeURIComponent(contest.docId)}` : ''}`,
        getRecordDetailUrl: contest ? `/record/{rid}?tid=${encodeURIComponent(contest.docId)}` : '/record/{rid}',
        pretestConnUrl: '',
      },
      UserContext: user,
    }
  }, [language, problem, tid, user, users])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!problem || !context) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <HydroTemplate
      template="problem_detail.html"
      context={context}
      url={(name, args = {}) => {
        const id = String(args.pid ?? problem.pid ?? problem.docId)
        const query = (args.query ?? {}) as Record<string, unknown>
        const contestTid = problem.contest?.docId ?? ''
        const activity = activityRoute(name, contestTid, args)
        if (activity !== undefined) return activity
        switch (name) {
          case 'problem_main':
            return query.q ? appendQuery('/p', { q: query.q }) : '/p'
          case 'problem_detail':
            return `/p/${encodeURIComponent(id)}${contestTid ? `?tid=${encodeURIComponent(String(query.tid ?? contestTid))}` : ''}`
          case 'problem_submit':
            return `/p/${encodeURIComponent(id)}/submit${appendQuery('', {
              ...query,
              ...(contestTid && !query.tid ? { tid: contestTid } : {}),
            })}`
          case 'problem_files':
            return `/p/${encodeURIComponent(id)}/files`
          case 'problem_solution':
            return `/p/${encodeURIComponent(id)}/solution`
          case 'problem_config':
            return `/p/${encodeURIComponent(id)}/config`
          case 'problem_edit':
            return `/p/${encodeURIComponent(id)}/edit`
          case 'problem_statistics':
            return `/p/${encodeURIComponent(id)}/statistics`
          case 'problem_hack':
            return `/p/${encodeURIComponent(id)}${contestTid ? `?tid=${encodeURIComponent(contestTid)}` : ''}`
          case 'record_detail':
            return appendQuery(`/record/${encodeURIComponent(String(args.rid ?? ''))}`, {
              ...query,
              ...(contestTid && !query.tid ? { tid: contestTid } : {}),
            })
          case 'user_detail':
            return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
          default:
            return '/'
        }
      }}
      onReady={handleReady}
    />
  )
}
