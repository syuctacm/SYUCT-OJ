// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping and the reply/broadcast interactions.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  broadcastClarification, getClarifications, getContest, getContestProblems, getUsersByIDs,
  replyClarification,
  type Clarification, type Contest, type ProblemSummary, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'
import { avatarUrl } from '../upstream/avatar.ts'

export function ContestClarificationPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [clarifications, setClarifications] = useState<Clarification[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const clickRef = useRef<(event: Event) => void>(() => {})

  const load = useCallback(async () => {
    const [detail, problemPage, tcdocs] = await Promise.all([
      getContest(tid),
      getContestProblems(tid),
      getClarifications(tid),
    ])
    setContest(detail)
    setProblems(problemPage.problems)
    setClarifications(tcdocs)
  }, [tid])

  useEffect(() => {
    setContest(null)
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '答疑加载失败'))
  }, [load])

  useEffect(() => {
    const ids = new Set<number>()
    if (contest) ids.add(contest.owner)
    for (const item of clarifications) ids.add(item.owner)
    if (!ids.size) {
      setUsers([])
      return
    }
    getUsersByIDs([...ids].filter((id) => id > 0)).then(setUsers).catch(() => setUsers([]))
  }, [clarifications, contest])

  const isManager = Boolean(user && contest && (
    user.id === contest.owner
    || ((user.priv ?? 0) & 1) !== 0
    || (contest.maintainer ?? []).includes(user.id)
  ))

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

  const context = useMemo(() => {
    if (!contest || !tdoc) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (permission === CONTEST_PERMS.PERM_VIEW_USER_PRIVATE_INFO) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD) return isAdmin
          if (permission === CONTEST_PERMS.PERM_EDIT_CONTEST) return isManager
          if (permission === CONTEST_PERMS.PERM_READ_RECORD_CODE) return isManager
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
      },
      args: { domainId: 'system' },
      request: {
        query: {},
        path: `/contest/${contest.docId}/clarification`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    const pdict = Object.fromEntries(problems.map((problem) => [String(problem.docId), {
      docId: problem.docId,
      pid: problem.pid,
      title: problem.title,
      hidden: false,
      nSubmit: problem.nSubmit,
      nAccept: problem.nAccept,
      difficulty: problem.difficulty,
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
    return {
      page_name: 'contest_clarification',
      tdoc,
      tsdoc: { attend: Boolean(contest.attended) },
      owner_udoc: udict[String(contest.owner)] ?? null,
      pdict,
      tcdocs,
      udict,
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [clarifications, contest, isManager, problems, tdoc, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'discussion_node': return `/discuss/contest/${encodeURIComponent(id)}`
      case 'contest_clarification': return `/contest/${encodeURIComponent(id)}/clarification`
      case 'contest_manage': return `/contest/${encodeURIComponent(id)}/clarification`
      case 'contest_edit': return `/contest/${encodeURIComponent(id)}/edit`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'problem_detail': {
        const raw = String(args.pid ?? '')
        const problem = problems.find((item) => String(item.docId) === raw || item.pid === raw)
        return `/p/${encodeURIComponent(problem?.pid || raw)}`
      }
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
      default: return '#'
    }
  }, [contest?.docId, problems, tid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = [hydrateHydroEditForm(root)]
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    const onClick = (event: Event) => clickRef.current(event)
    root.addEventListener('click', onClick)
    cleanups.push(() => root.removeEventListener('click', onClick))
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  function handleClick(event: Event) {
    const target = (event.target as HTMLElement).closest<HTMLElement>('a[name="reply"]')
    if (!target) return
    event.preventDefault()
    const did = target.dataset.did ?? ''
    const form = document.querySelector<HTMLFormElement>('#reply_or_broadcast form')
    if (!form) return
    const hidden = form.querySelector<HTMLInputElement>('input[name="did"]')
    if (hidden) hidden.value = did
    form.scrollIntoView({ behavior: 'smooth', block: 'start' })
    form.querySelector<HTMLTextAreaElement>('textarea[name="content"]')?.focus()
  }
  clickRef.current = handleClick

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    if (data.get('operation') !== 'clarification') return
    const content = String(data.get('content') ?? '').trim()
    const did = String(data.get('did') ?? '').trim()
    const subject = Number(data.get('subject') ?? 0)
    if (!content) return
    setActionError('')
    try {
      if (did) await replyClarification(tid, did, content)
      else await broadcastClarification(tid, { subject, content })
      form.reset()
      const hidden = form.querySelector<HTMLInputElement>('input[name="did"]')
      if (hidden) hidden.value = ''
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '提交失败')
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest || !tdoc) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!isManager) {
    return <div className="row"><div className="columns"><blockquote className="warn"><p>需要比赛管理权限。</p></blockquote></div></div>
  }
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_clarification.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
