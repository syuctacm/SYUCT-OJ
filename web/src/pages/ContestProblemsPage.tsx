// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping, request handling and small interactions.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  createClarification, getClarifications, getContest, getContestProblems, getSubmissions, getUsersByIDs,
  type Clarification, type Contest, type ContestStatus, type ProblemStatus, type ProblemSummary, type Submission, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { languageOptions } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'
import { CONTEST_PERMS, createContestModel, isDone } from '../upstream/contest.ts'
import { useHydroContestTimer } from '../upstream/contest-timer.ts'
import { avatarUrl } from '../upstream/avatar.ts'

const languageMap = Object.fromEntries(languageOptions.map((language) => [
  language._id,
  { display: language.name, highlight: language._id },
]))

export function ContestProblemsPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [statuses, setStatuses] = useState<Record<string, ProblemStatus>>({})
  const [attended, setAttended] = useState(false)
  const [problemPageStatus, setProblemPageStatus] = useState<ContestStatus | undefined>()
  const [clarifications, setClarifications] = useState<Clarification[]>([])
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [postError, setPostError] = useState('')
  const [posting, setPosting] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  useHydroContestTimer(contest)

  const load = useCallback(async () => {
    setError('')
    try {
      const [detail, problemPage, tcdocs] = await Promise.all([
        getContest(tid),
        getContestProblems(tid),
        getClarifications(tid).catch(() => [] as Clarification[]),
      ])
      setContest(detail)
      setProblems(problemPage.problems)
      setStatuses(problemPage.statuses ?? {})
      setAttended(problemPage.attended)
      setProblemPageStatus(problemPage.status)
      setClarifications(tcdocs)
    } catch (err) {
      setError(err instanceof Error ? err.message : '比赛题目加载失败')
    }
  }, [tid])

  useEffect(() => {
    setContest(null)
    void load()
  }, [load])

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
      privateFiles: [],
      domainId: 'system',
    }
  }, [contest])

  const handler = useMemo(() => {
    if (!contest) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const canEdit = Boolean(user && (user.id === contest.owner || isAdmin))
    return {
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
          if (permission === CONTEST_PERMS.PERM_CREATE_DISCUSSION) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_DISCUSSION) return true
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
      },
      args: { domainId: 'system' },
      request: {
        query: {},
        path: `/contest/${contest.docId}/problems`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
  }, [contest, user])

  const contestModel = useMemo(() => (handler ? createContestModel(handler) : null), [handler])
  const canViewRecord = useMemo(
    () => Boolean(contestModel && tdoc && contestModel.canShowSelfRecord(tdoc)),
    [contestModel, tdoc],
  )

  useEffect(() => {
    if (!canViewRecord || !user) {
      setSubmissions([])
      return
    }
    getSubmissions({ tid, uidOrName: String(user.id), limit: 100 })
      .then((page) => setSubmissions(page.items))
      .catch(() => setSubmissions([]))
  }, [canViewRecord, tid, user])

  useEffect(() => {
    if (!contest) return
    const ids = new Set<number>([contest.owner])
    for (const item of clarifications) ids.add(item.owner)
    for (const item of submissions) {
      if (item.submitter) ids.add(item.submitter.id)
    }
    getUsersByIDs([...ids].filter((id) => id > 0))
      .then(setUsers)
      .catch(() => setUsers([]))
  }, [clarifications, contest, submissions])

  const context = useMemo(() => {
    if (!contest || !tdoc || !handler || !contestModel) return {}
    const pdict = Object.fromEntries(problems.map((problem) => [String(problem.docId), {
      docId: problem.docId,
      pid: problem.pid,
      title: problem.title,
      hidden: false,
      nSubmit: problem.nSubmit,
      nAccept: problem.nAccept,
      difficulty: problem.difficulty,
      config: {
        type: problem.problemType ?? '',
        timeMin: problem.timeMin || '1000',
        timeMax: problem.timeMax || problem.timeMin || '1000',
        memoryMin: problem.memoryMin || '256',
        memoryMax: problem.memoryMax || problem.memoryMin || '256',
      },
    }]))
    const psdict = Object.fromEntries(Object.entries(statuses).map(([pid, status]) => [pid, {
      rid: status.rid,
      score: status.score,
    }]))
    const rdict = Object.fromEntries(Object.values(statuses).map((status) => [status.rid, {
      _id: status.rid,
      status: status.status,
      score: status.score,
      domainId: 'system',
    }]))
    const udict = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    }]))
    const tcdocs = clarifications.map((item) => ({
      ...item,
      _id: item.id,
      reply: item.reply.map((reply) => ({ ...reply, _id: reply.id })),
    }))
    const rdocs = submissions.map((item) => ({
      _id: item.id,
      uid: item.uid,
      pid: item.pid,
      status: item.status,
      score: item.score,
      time: item.time,
      memory: item.memory,
      lang: item.lang,
      contest: contest.docId,
      domainId: 'system',
    }))
    return {
      page_name: 'contest_problemlist',
      tdoc,
      tsdoc: {
        attend: attended,
        startAt: problemPageStatus?.startAt ? new Date(problemPageStatus.startAt) : undefined,
        endAt: problemPageStatus?.endAt ? new Date(problemPageStatus.endAt) : undefined,
      },
      pdict,
      psdict,
      rdict,
      udict,
      rdocs,
      tcdocs,
      showScore: false,
      canViewRecord,
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: contestModel, setting: { langs: languageMap } },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [attended, canViewRecord, clarifications, contest, contestModel, handler, problemPageStatus, problems, statuses, submissions, tdoc, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'contest_problemlist': return `/contest/${encodeURIComponent(id)}/problems`
      case 'contest_scoreboard': return `/contest/${encodeURIComponent(id)}/scoreboard`
      case 'contest_edit': return `/contest/${encodeURIComponent(id)}/edit`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'record_detail': return `/record/${encodeURIComponent(String(args.rid ?? ''))}`
      case 'problem_detail': {
        const raw = String(args.pid ?? '')
        const problem = problems.find((item) => String(item.docId) === raw || item.pid === raw)
        return `/p/${encodeURIComponent(problem?.pid || raw)}?tid=${encodeURIComponent(id)}`
      }
      case 'problem_submit': {
        const raw = String(args.pid ?? '')
        const problem = problems.find((item) => String(item.docId) === raw || item.pid === raw)
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        if (!params.has('tid')) params.set('tid', id)
        return `/p/${encodeURIComponent(problem?.pid || raw)}/submit?${params.toString()}`
      }
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'discussion_node':
      case 'wiki':
      case 'wiki_help':
        return '/discuss'
      // Contest management is not ported yet.
      case 'contest_manage': return `/contest/${encodeURIComponent(id)}/clarification`
      case 'contest_code': return `/api/v1/contests/${encodeURIComponent(id)}/code`
      case 'contest_print': return `/contest/${encodeURIComponent(id)}/print`
      case 'contest_file_download':
        return '#'
      default: return '/'
    }
  }, [contest?.docId, problems, tid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = [hydrateHydroEditForm(root)]
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    if (data.get('operation') !== 'clarification') return
    const subject = Number(data.get('subject') ?? 0)
    const content = String(data.get('content') ?? '').trim()
    if (!content) return
    setPosting(true)
    setPostError('')
    try {
      await createClarification(tid, { subject, content })
      await load()
    } catch (err) {
      setPostError(err instanceof Error ? err.message : '提问失败')
    } finally {
      setPosting(false)
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!contest || !tdoc) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  const canManage = Boolean(user && (user.id === contest.owner || isAdmin))
  if (!attended && !isDone(tdoc) && !canManage) {
    return (
      <div className="row"><div className="columns">
        <blockquote className="warn">
          <p>{user ? '您还没有报名参加这场比赛，请先到比赛详情页报名。' : '请先登录并报名参加比赛后再查看题目列表。'}</p>
        </blockquote>
      </div></div>
    )
  }
  return (
    <>
      {postError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{postError}</p></blockquote></div></div> : null}
      {posting ? <div className="row"><div className="columns"><blockquote className="note"><p>提交中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_problemlist.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
