// Page markup is rendered from the pinned Hydro ui-default template
// (contest_print.html). The `#printKioskContainer` is filled by
// PrintKioskPanel; the toolbar buttons queue tasks and toggle the kiosk.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { clearRendered, renderInto } from '../upstream/roots.ts'
import { useParams } from 'react-router-dom'
import {
  addPrintTask, getContest, getUsersByIDs,
  type Contest, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { Dialog } from '../components/Dialog.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { PrintKioskPanel } from '../components/PrintKioskPanel.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'

export function ContestPrintPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [creating, setCreating] = useState(false)
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [kiosk, setKiosk] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

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

  const isManager = Boolean(user && contest && (
    user.id === contest.owner
    || ((user.priv ?? 0) & 1) !== 0
    || (contest.maintainer ?? []).includes(user.id)
  ))

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    root.querySelectorAll<HTMLButtonElement>('[name="add_print_task"]').forEach((button) => {
      const onClick = () => {
        setTitle('')
        setContent('')
        setCreating(true)
      }
      button.addEventListener('click', onClick)
      cleanups.push(() => button.removeEventListener('click', onClick))
    })
    root.querySelectorAll<HTMLButtonElement>('[name="enable_print_kiosk"]').forEach((button) => {
      const onClick = () => setKiosk((value) => !value)
      button.addEventListener('click', onClick)
      cleanups.push(() => button.removeEventListener('click', onClick))
    })

    const container = root.querySelector<HTMLElement>('#printKioskContainer')
    if (container) {
      container.replaceChildren()
      const host = document.createElement('div')
      container.appendChild(host)
      renderInto(host, <PrintKioskPanel tid={tid} kiosk={kiosk} key={refreshKey} />)
      cleanups.push(() => {
        clearRendered(host)
        host.remove()
      })
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [kiosk, refreshKey, tid])

  async function submitTask() {
    if (!content.trim()) {
      setActionError('打印内容不能为空。')
      return
    }
    setActionError('')
    try {
      await addPrintTask(tid, title, content)
      setCreating(false)
      setRefreshKey((key) => key + 1)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '打印任务提交失败')
    }
  }

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'contest_problemlist': return `/contest/${encodeURIComponent(id)}/problems`
      case 'contest_scoreboard': return `/contest/${encodeURIComponent(id)}/scoreboard`
      case 'contest_edit': return `/contest/${encodeURIComponent(id)}/edit`
      case 'contest_print': return `/contest/${encodeURIComponent(id)}/print`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
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
    if (!contest) return {}
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
        path: `/contest/${contest.docId}/print`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    const owner = users.find((item) => item.id === contest.owner)
    return {
      page_name: 'contest_print',
      tdoc: {
        ...contest,
        _id: contest.docId,
        beginAt: new Date(contest.beginAt),
        endAt: new Date(contest.endAt),
        lockAt: contest.lockAt ? new Date(contest.lockAt) : null,
        penaltySince: contest.penaltySince ? new Date(contest.penaltySince) : null,
        pids: contest.pids ?? [],
        domainId: 'system',
      },
      tsdoc: { attend: Boolean(contest.attended) },
      owner_udoc: owner ? {
        _id: owner.id, uname: owner.username,
        displayName: owner.displayName ?? '', avatar: owner.avatar ?? '',
      } : null,
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [contest, isManager, user, users])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  // Upstream returns NotFoundError when printing is disabled for the contest.
  if (contest && !contest.allowPrint) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">404 Not Found</div></div></div>
  }
  if (!contest) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_print.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
      {creating ? (
        <Dialog
          onOk={() => void submitTask()}
          onCancel={() => setCreating(false)}
          okLabel="提交"
          cancelLabel="取消"
          width="720px"
        >
          <h1>打印新文件</h1>
          <label>
            文件名
            <input className="textbox" value={title} onChange={(event) => setTitle(event.target.value)} placeholder="main.cpp" />
          </label>
          <label>
            内容
            <textarea
              className="textbox"
              value={content}
              onChange={(event) => setContent(event.target.value)}
              spellCheck={false}
              style={{ width: '100%', minHeight: 240, fontFamily: 'var(--code-font-family)' }}
            />
          </label>
        </Dialog>
      ) : null}
    </>
  )
}
