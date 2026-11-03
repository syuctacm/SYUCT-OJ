// Page markup is vendored from Hydro ui-default contest_team.html. React owns
// the team CRUD adapters and Hydro dialogs only.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  acceptContestTeamInvite, createContestTeam, getContestTeams, inviteContestTeamMember,
  leaveContestTeam, rejectContestTeamInvite, renameContestTeam,
  type ContestTeamsSnapshot,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { promptDialog } from '../components/dialogs.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { ErrorPage } from './ErrorPage.tsx'
import '../legacy/contest-team.styl'

export function ContestTeamsPage() {
  const { user } = useAuth()
  const [data, setData] = useState<ContestTeamsSnapshot | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const busyRef = useRef(false)
  const dataRef = useRef<ContestTeamsSnapshot | null>(null)
  dataRef.current = data
  busyRef.current = busy

  useEffect(() => {
    if (!user) return
    getContestTeams()
      .then((next) => {
        setData(next)
        setError('')
      })
      .catch((reason: unknown) => setError(reason instanceof Error ? reason.message : '团队加载失败'))
  }, [user])

  const run = useCallback(async (operation: () => Promise<ContestTeamsSnapshot>) => {
    if (busyRef.current) return
    busyRef.current = true
    setBusy(true)
    setError('')
    try {
      setData(await operation())
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '操作失败')
    } finally {
      busyRef.current = false
      setBusy(false)
    }
  }, [])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const onForm = (event: SubmitEvent) => {
      event.preventDefault()
      const form = event.target as HTMLFormElement
      const values = new FormData(form)
      const operation = String(values.get('operation') ?? '')
      const vuid = Number(values.get('vuid'))
      if (operation === 'create') {
        void run(() => createContestTeam(String(values.get('name') ?? '')))
      } else if (operation === 'rename') {
        void run(() => renameContestTeam(vuid, String(values.get('name') ?? '')))
      } else if (operation === 'accept') {
        void run(() => acceptContestTeamInvite(vuid))
      } else if (operation === 'reject') {
        void run(() => rejectContestTeamInvite(vuid))
      } else if (operation === 'leave') {
        const target = Number(values.get('uid') ?? user?.id ?? 0)
        void run(() => leaveContestTeam(vuid, target))
      }
    }
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      form.addEventListener('submit', onForm)
      cleanups.push(() => form.removeEventListener('submit', onForm))
    }

    const onInvite = (event: Event) => {
      const button = event.currentTarget as HTMLButtonElement
      const vuid = Number(button.dataset.vuid)
      void promptDialog('用户名 / UID').then((value) => {
        const target = value?.trim()
        if (target) void run(() => inviteContestTeamMember(vuid, target))
      })
    }
    root.querySelectorAll<HTMLButtonElement>('[data-contest-team-invite]').forEach((button) => {
      button.addEventListener('click', onInvite)
      cleanups.push(() => button.removeEventListener('click', onInvite))
    })

    root.querySelectorAll<HTMLButtonElement>('[data-contest-team-rename]').forEach((button) => {
      const onClick = () => {
        const card = button.closest<HTMLElement>('[data-contest-team]')
        const form = card?.querySelector<HTMLFormElement>('[data-contest-team-rename-form]')
        if (!form) return
        form.classList.remove('is-hidden')
        button.classList.add('is-hidden')
        form.querySelector<HTMLInputElement>('input[name="name"]')?.focus()
      }
      button.addEventListener('click', onClick)
      cleanups.push(() => button.removeEventListener('click', onClick))
    })
    root.querySelectorAll<HTMLButtonElement>('[data-contest-team-rename-cancel]').forEach((button) => {
      const onClick = () => {
        const card = button.closest<HTMLElement>('[data-contest-team]')
        const form = card?.querySelector<HTMLFormElement>('[data-contest-team-rename-form]')
        if (!form) return
        form.classList.add('is-hidden')
        card?.querySelector<HTMLButtonElement>('[data-contest-team-rename]')?.classList.remove('is-hidden')
      }
      button.addEventListener('click', onClick)
      cleanups.push(() => button.removeEventListener('click', onClick))
    })
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [run, user?.id])

  const context = useMemo(() => {
    if (!data) return null
    const mine = data.mine.map((team) => ({ ...team, _id: team.id }))
    const invites = data.invites.map((team) => ({ ...team, _id: team.id }))
    const udict = Object.fromEntries(Object.entries(data.users).map(([id, item]) => [id, {
      _id: item.id,
      uname: item.displayName || item.username,
      avatar: item.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    }]))
    return {
      page_name: 'contest_team',
      mine,
      invites,
      udict,
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: () => false,
          hasPriv: (privilege: number) => Boolean(user) && privilege === 1,
          own: () => false,
        },
        args: { domainId: 'system' },
        request: { query: {}, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      PRIV: { PRIV_USER_PROFILE: 1 },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [data, user])

  if (!user) return <ErrorPage status={403} />
  if (!data || !context) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '加载中...'}</div></div></div>

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {busy ? <div className="row"><div className="columns"><blockquote className="note"><p>正在处理团队操作...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_team.html"
        context={context}
        onReady={handleReady}
        url={(name, args = {}) => {
          if (name === 'contest_team') return '/contest/team'
          if (name === 'user_detail') return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
          return '/'
        }}
      />
    </>
  )
}
