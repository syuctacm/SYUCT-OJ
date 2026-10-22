// Page markup is rendered from the pinned Hydro ui-default template
// (contest_balloon.html). Keep this file limited to data mapping and the
// balloon interactions the template expects.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  getContest, getContestBalloons, getContestProblems, getUsersByIDs, markContestBalloonSent,
  setContestBalloonColors,
  type Contest, type ContestBalloon, type ContestBalloonColor, type ProblemSummary, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { Dialog } from '../components/Dialog.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'

function colorsToYaml(colors: Record<string, ContestBalloonColor>) {
  return Object.entries(colors)
    .map(([pid, color]) => `'${pid}':\n  name: ${color.name}\n  color: '${color.color}'`)
    .join('\n')
}

export function ContestBalloonPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [balloons, setBalloons] = useState<ContestBalloon[]>([])
  const [colors, setColors] = useState<Record<string, ContestBalloonColor>>({})
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [editing, setEditing] = useState(false)
  const [colorDraft, setColorDraft] = useState('')

  const load = useCallback(async () => {
    const [detail, problemPage, balloonPage] = await Promise.all([
      getContest(tid),
      getContestProblems(tid),
      getContestBalloons(tid),
    ])
    setContest(detail)
    setProblems(problemPage.problems)
    setBalloons(balloonPage.items)
    setColors(balloonPage.colors ?? {})
  }, [tid])

  useEffect(() => {
    setContest(null)
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '气球状态加载失败'))
  }, [load])

  useEffect(() => {
    const ids = new Set<number>()
    for (const balloon of balloons) {
      if (balloon.uid) ids.add(balloon.uid)
      if (balloon.sent) ids.add(balloon.sent)
    }
    if (!ids.size) {
      setUsers([])
      return
    }
    getUsersByIDs([...ids]).then(setUsers).catch(() => setUsers([]))
  }, [balloons])

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
      balloon: colors,
      domainId: 'system',
    }
  }, [colors, contest])

  const isManager = Boolean(user && contest && (
    user.id === contest.owner
    || ((user.priv ?? 0) & 1) !== 0
    || (contest.maintainer ?? []).includes(user.id)
  ))

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    root.querySelectorAll<HTMLButtonElement>('[name="set_color"]').forEach((button) => {
      const onClick = () => {
        setColorDraft(colorsToYaml(colors))
        setEditing(true)
      }
      button.addEventListener('click', onClick)
      cleanups.push(() => button.removeEventListener('click', onClick))
    })
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => {
        event.preventDefault()
        const bid = String(new FormData(form).get('balloon') ?? '')
        if (!bid) return
        void (async () => {
          setActionError('')
          try {
            await markContestBalloonSent(tid, bid)
            await load()
          } catch (err) {
            setActionError(err instanceof Error ? err.message : '气球状态更新失败')
          }
        })()
      }
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [colors, load, tid])

  async function saveColors() {
    let parsed: unknown
    try {
      const yaml = await import('js-yaml')
      parsed = yaml.load(colorDraft)
    } catch {
      setActionError('气球配置不是合法的 YAML。')
      return
    }
    if (!parsed || typeof parsed !== 'object') {
      setActionError('气球配置需要是 "题目 ID: {name, color}" 的形式。')
      return
    }
    setActionError('')
    try {
      await setContestBalloonColors(tid, parsed as Record<string, ContestBalloonColor>)
      setEditing(false)
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '气球配置保存失败')
    }
  }

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'discussion_node': return `/discuss/contest/${encodeURIComponent(id)}`
      case 'contest_manage': return `/contest/${encodeURIComponent(id)}/management`
      case 'contest_edit': return `/contest/${encodeURIComponent(id)}/edit`
      case 'contest_user': return `/contest/${encodeURIComponent(id)}/user`
      case 'contest_clarification': return `/contest/${encodeURIComponent(id)}/clarification`
      case 'contest_balloon': return `/contest/${encodeURIComponent(id)}/balloon`
      case 'contest_code': return `/api/v1/contests/${encodeURIComponent(id)}/code`
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
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
        path: `/contest/${contest.docId}/balloon`,
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
    const owner = users.find((item) => item.id === contest.owner)
    return {
      page_name: 'contest_balloon',
      tdoc,
      tsdoc: { attend: Boolean(contest.attended) },
      pdict,
      bdocs: balloons.map((balloon) => ({
        _id: {
          toHexString: () => balloon.id,
          // ObjectId timestamps are the first four bytes of the hex string.
          getTimestamp: () => new Date(Number.parseInt(balloon.id.slice(0, 8), 16) * 1000),
        },
        bid: balloon.id,
        uid: balloon.uid,
        pid: balloon.pid,
        first: Boolean(balloon.first),
        sent: balloon.sent ?? 0,
        sentAt: balloon.sentAt ? new Date(balloon.sentAt) : null,
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
  }, [balloons, colors, contest, isManager, problems, tdoc, user, users])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest || !tdoc) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!isManager) {
    return <div className="row"><div className="columns"><blockquote className="warn"><p>需要比赛管理权限。</p></blockquote></div></div>
  }
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_balloon.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
      {editing ? (
        <Dialog
          onOk={() => void saveColors()}
          onCancel={() => setEditing(false)}
          okLabel="保存"
          cancelLabel="取消"
          width="720px"
        >
          <h1>设置气球颜色</h1>
          <textarea
            className="textbox"
            value={colorDraft}
            onChange={(event) => setColorDraft(event.target.value)}
            spellCheck={false}
            style={{ width: '100%', minHeight: 260, fontFamily: 'var(--code-font-family)' }}
          />
        </Dialog>
      ) : null}
    </>
  )
}
