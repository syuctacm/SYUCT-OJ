// Page markup is rendered from the pinned Hydro ui-default template
// (contest_user.html). Keep this file limited to data mapping and the attendee
// actions that template expects.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  addContestAttendees, cancelContestAttendee, getContest, getContestAttendees,
  resumeContestAttendee, setContestAttendeeRank,
  type Contest, type ContestAttendee, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { Dialog } from '../components/Dialog.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { UserSelectAutoComplete } from '../components/autocomplete/UserSelectAutoComplete.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'

export function ContestUserPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [statuses, setStatuses] = useState<ContestAttendee[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [picking, setPicking] = useState(false)
  const [pickedUID, setPickedUID] = useState(0)
  const clickRef = useRef<(event: Event) => void>(() => {})

  const load = useCallback(async () => {
    const [detail, attendees] = await Promise.all([getContest(tid), getContestAttendees(tid)])
    setContest(detail)
    setStatuses(attendees.items)
    setUsers(attendees.users)
  }, [tid])

  useEffect(() => {
    setContest(null)
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '参赛者加载失败'))
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
      domainId: 'system',
    }
  }, [contest])

  const isManager = Boolean(user && contest && (
    user.id === contest.owner
    || ((user.priv ?? 0) & 1) !== 0
    || (contest.maintainer ?? []).includes(user.id)
  ))

  const handleReady = useCallback((root: HTMLDivElement) => {
    const onClick = (event: Event) => clickRef.current(event)
    root.addEventListener('click', onClick)
    return () => root.removeEventListener('click', onClick)
  }, [])

  function handleClick(event: Event) {
    const target = (event.target as HTMLElement).closest<HTMLElement>('[name="add_user"], [name="edit_rank"], [name="cancel_registration"], [name="resume_contest"]')
    if (!target) return
    event.preventDefault()
    const action = target.getAttribute('name')
    if (action === 'add_user') {
      setPickedUID(0)
      setPicking(true)
      return
    }
    const uid = Number(target.dataset.uid ?? 0)
    if (!uid) return
    if (action === 'resume_contest') {
      void run(() => resumeContestAttendee(tid, uid))
      return
    }
    if (action === 'edit_rank') {
      const current = statuses.find((item) => item.uid === uid)
      void run(() => setContestAttendeeRank(tid, uid, !current?.unrank))
      return
    }
    if (!window.confirm('确认取消该用户的报名？')) return
    void run(() => cancelContestAttendee(tid, uid))
  }
  clickRef.current = handleClick

  async function run(action: () => Promise<void>) {
    setActionError('')
    try {
      await action()
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '操作失败')
    }
  }

  async function confirmAddUser() {
    if (!pickedUID) return
    setPicking(false)
    await run(() => addContestAttendees(tid, [pickedUID]))
  }

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'contest_manage': return `/contest/${encodeURIComponent(id)}/management`
      case 'contest_edit': return `/contest/${encodeURIComponent(id)}/edit`
      case 'contest_user': return `/contest/${encodeURIComponent(id)}/user`
      case 'contest_clarification': return `/contest/${encodeURIComponent(id)}/clarification`
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'contest_code': return `/api/v1/contests/${encodeURIComponent(id)}/code`
      case 'contest_balloon':
        return '#'
      default: return '/'
    }
  }, [contest?.docId, tid])

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
        path: `/contest/${contest.docId}/user`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    const udict = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    }]))
    const owner = users.find((item) => item.id === contest.owner)
    return {
      page_name: 'contest_user',
      tdoc,
      tsdocs: statuses.map((item) => ({
        uid: item.uid,
        unrank: Boolean(item.unrank),
        startAt: item.startAt ? new Date(item.startAt) : null,
        endAt: item.endAt ? new Date(item.endAt) : null,
      })),
      udict,
      owner_udoc: owner ? udict[String(owner.id)] : null,
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [contest, isManager, statuses, tdoc, user, users])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest || !tdoc) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!isManager) {
    return <div className="row"><div className="columns"><blockquote className="warn"><p>需要比赛管理权限。</p></blockquote></div></div>
  }
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_user.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
      {picking ? (
        <Dialog onOk={() => void confirmAddUser()} onCancel={() => setPicking(false)} okLabel="添加" cancelLabel="取消">
          <h1>添加参赛者</h1>
          <UserSelectAutoComplete
            selectedKeys={[]}
            multi={false}
            placeholder="输入用户名搜索"
            onChange={(value) => setPickedUID(Number(value) || 0)}
          />
        </Dialog>
      ) : null}
    </>
  )
}
