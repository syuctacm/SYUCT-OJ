// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping, request handling and small interactions.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  attendHomework, getHomeworkProblems, getUsersByIDs,
  type HomeworkProblemPage, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { CONTEST_PERMS, createContestModel, isDone, isNotStarted } from '../upstream/contest.ts'
import { avatarUrl } from '../upstream/avatar.ts'

export function HomeworkDetailPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [data, setData] = useState<HomeworkProblemPage | null>(null)
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [claimError, setClaimError] = useState('')
  const [claiming, setClaiming] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    setData(null)
    setError('')
    getHomeworkProblems(tid)
      .then(setData)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
  }, [tid])

  useEffect(() => {
    if (!data) return
    getUsersByIDs([data.homework.owner]).then(setUsers).catch(() => setUsers([]))
  }, [data])

  const tdoc = useMemo(() => {
    if (!data) return null
    const homework = data.homework
    return {
      ...homework,
      _id: homework.id,
      beginAt: new Date(homework.beginAt),
      endAt: new Date(homework.endAt),
      penaltySince: new Date(homework.penaltySince ?? homework.endAt),
      lockAt: homework.lockAt ? new Date(homework.lockAt) : null,
      pids: homework.pids ?? [],
      domainId: 'system',
    }
  }, [data])

  const context = useMemo(() => {
    if (!data || !tdoc) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const canEdit = Boolean(user && (user.id === data.homework.owner || isAdmin))
    // Mirrors the upstream handler: problems are only exposed to the owner,
    // admins, or after the homework started and the user claimed it.
    const canSeeProblems = canEdit
      || !(isNotStarted(tdoc) || (!data.attended && !isDone(tdoc)))
    const pdict = canSeeProblems ? Object.fromEntries(data.problems.map((problem) => [String(problem.docId), {
      docId: problem.docId,
      pid: problem.pid,
      title: problem.title,
      hidden: false,
      nSubmit: problem.nSubmit,
      nAccept: problem.nAccept,
      difficulty: problem.difficulty,
    }])) : undefined
    const statuses = data.statuses ?? {}
    const psdict = Object.fromEntries(Object.entries(statuses).map(([pid, status]) => [pid, {
      rid: status.rid,
      score: status.score,
    }]))
    const rdict = Object.fromEntries(Object.values(statuses).map((status) => [status.rid, {
      _id: status.rid,
      status: status.status,
      score: status.score,
    }]))
    const udict = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    }]))
    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (permission === CONTEST_PERMS.PERM_VIEW_USER_PRIVATE_INFO) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD) return canEdit
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_SCOREBOARD) return true
          if (permission === CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD) return isAdmin
          if (permission === CONTEST_PERMS.PERM_ATTEND_HOMEWORK) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_EDIT_HOMEWORK) return canEdit
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
        path: `/homework/${data.homework.docId}`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    return {
      page_name: 'homework_detail',
      tdoc,
      tsdoc: { attend: data.attended },
      pdict,
      psdict,
      rdict,
      udict,
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [data, tdoc, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? data?.homework.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'homework_detail': return `/homework/${encodeURIComponent(id)}`
      case 'homework_edit': return `/homework/${encodeURIComponent(id)}/edit`
      case 'homework_files': return `/homework/${encodeURIComponent(id)}/file`
      case 'homework_scoreboard': return `/homework/${encodeURIComponent(id)}/scoreboard`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'record_detail': return `/record/${encodeURIComponent(String(args.rid ?? ''))}`
      case 'problem_detail': {
        const raw = String(args.pid ?? '')
        const problem = data?.problems.find((item) => String(item.docId) === raw || item.pid === raw)
        return `/p/${encodeURIComponent(problem?.pid || raw)}`
      }
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'wiki':
      case 'wiki_help':
        return '/discuss'
      // Homework code export is not ported yet.
      case 'homework_code':
        return '#'
      default: return '/'
    }
  }, [data, tid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
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
    const operation = new FormData(form).get('operation')
    if (operation !== 'attend') return
    setClaiming(true)
    setClaimError('')
    try {
      await attendHomework(tid)
      setData(await getHomeworkProblems(tid))
    } catch (err) {
      setClaimError(err instanceof Error ? err.message : '认领失败')
    } finally {
      setClaiming(false)
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!data) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {claimError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{claimError}</p></blockquote></div></div> : null}
      {claiming ? <div className="row"><div className="columns"><blockquote className="note"><p>认领中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="homework_detail.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
