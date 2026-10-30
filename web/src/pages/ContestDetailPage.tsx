// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping, request handling and small interactions.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  attendContest, earlyEndContest, getContest, getUsersByIDs,
  type Contest, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { promptDialog } from '../components/dialogs.tsx'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'
import { useHydroContestTimer } from '../upstream/contest-timer.ts'
import { avatarUrl } from '../upstream/avatar.ts'

export function ContestDetailPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [attendError, setAttendError] = useState('')
  const [attending, setAttending] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  useHydroContestTimer(contest)

  useEffect(() => {
    setContest(null)
    setError('')
    getContest(tid)
      .then(setContest)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
  }, [tid])

  useEffect(() => {
    if (!contest) return
    getUsersByIDs([contest.owner]).then(setUsers).catch(() => setUsers([]))
  }, [contest])

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
      _code: contest.requiresInvitationCode ? '__required__' : '',
      domainId: 'system',
    }
  }, [contest])

  const context = useMemo(() => {
    if (!contest || !tdoc) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const canEdit = Boolean(user && (user.id === contest.owner || isAdmin))
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
            if (permission === CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD) return isAdmin
            if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD) return isAdmin
            if (permission === CONTEST_PERMS.PERM_ATTEND_CONTEST) return Boolean(user)
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
        path: `/contest/${contest.docId}`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    return {
      page_name: 'contest_detail',
      tdoc,
      tsdoc: {
        attend: Boolean(contest.attended),
        startAt: contest.status?.startAt ? new Date(contest.status.startAt) : undefined,
        endAt: contest.status?.endAt ? new Date(contest.status.endAt) : undefined,
      },
      udict,
      // The upstream detail page only lists contest materials, and the API
      // returns them for owners, administrators and participants.
      files: (contest.privateFiles ?? []).map((file) => ({ name: file.name, size: file.size })),
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [contest, tdoc, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'contest_problemlist': return `/contest/${encodeURIComponent(id)}/problems`
      case 'contest_scoreboard': return `/contest/${encodeURIComponent(id)}/scoreboard`
      case 'contest_edit': return `/contest/${encodeURIComponent(id)}/edit`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'discussion_node':
      case 'wiki_help':
        return '/discuss'
      case 'contest_manage': return `/contest/${encodeURIComponent(id)}/management`
      case 'contest_code': return `/api/v1/contests/${encodeURIComponent(id)}/code`
      // Contest printing is not ported yet; keep the upstream entry inert.
      case 'contest_print': return `/contest/${encodeURIComponent(id)}/print`
      default: return '/'
    }
  }, [contest?.docId, tid])

  const urlForFile = useCallback((name: string) => (
    `/api/v1/contests/${encodeURIComponent(tid)}/files/${encodeURIComponent(name)}?type=private`
  ), [tid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  async function submitAttend(code: string) {
    if (attending) return
    setAttending(true)
    setAttendError('')
    try {
      await attendContest(tid, code)
      setContest(await getContest(tid))
    } catch (err) {
      setAttendError(err instanceof Error ? err.message : '报名失败')
    } finally {
      setAttending(false)
    }
  }

  async function handleSubmit(event: SubmitEvent) {
    // The upstream template's inline confirm cancels the default action before
    // this listener runs. Respect that decision instead of asking twice.
    if (event.defaultPrevented) return
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const operation = new FormData(form).get('operation')
    if (operation === 'early_end') {
      setAttendError('')
      try {
        await earlyEndContest(tid)
        setContest(await getContest(tid))
      } catch (err) {
        setAttendError(err instanceof Error ? err.message : '提前结束比赛失败')
      }
      return
    }
    if (operation !== 'attend') return
    let code = ''
    if (form.querySelector('[data-contest-code]')) {
      const input = await promptDialog('邀请码：')
      if (input === null) return
      code = input.trim()
      if (!code) return
    }
    await submitAttend(code)
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!contest) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {attendError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{attendError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_detail.html"
        context={context}
        url={buildUrl}
        urlForFile={urlForFile}
        onReady={handleReady}
      />
    </>
  )
}
