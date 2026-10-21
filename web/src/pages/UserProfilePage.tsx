// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping; the activity calendar is mounted as its tab.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { useParams } from 'react-router-dom'
import { getPublicUser, type PublicUser } from '../api.ts'
import { ActivityCalendar } from '../components/ActivityCalendar.tsx'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

const USER_PERMS = {
  PERM_VIEW_USER_PRIVATE_INFO: 1,
  PERM_MOD_BADGE: 2,
} as const

export function UserProfilePage() {
  const { uid = '' } = useParams()
  const { user: viewer } = useAuth()
  const [profile, setProfile] = useState<PublicUser | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    setProfile(null)
    setError('')
    getPublicUser(uid)
      .then(setProfile)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '用户加载失败'))
  }, [uid])

  const context = useMemo(() => {
    if (!profile) return {}
    const udoc = {
      _id: profile.id,
      uname: profile.username,
      displayName: profile.displayName ?? '',
      avatar: profile.avatar ?? '',
      bio: profile.bio ?? '',
      regat: new Date(profile.regAt),
      loginat: new Date(profile.loginAt ?? profile.regAt),
      nSubmit: profile.nSubmit,
      nAccept: profile.nAccept,
      nLiked: 0,
      rp: profile.rp ?? 0,
      level: profile.level ?? 0,
      rank: profile.rank && profile.rank > 0 ? profile.rank : '?',
      gender: 0,
      hasPriv: (privilege: number) => privilege === 1,
      hasPerm: () => false,
    }
    const pdocs = (profile.acceptedProblems ?? []).map((problem) => ({
      docId: problem.docId,
      pid: problem.pid,
      title: problem.title,
      hidden: false,
    }))
    return {
      page_name: 'user_detail',
      udoc,
      sdoc: null,
      pdocs,
      tags: [],
      // The account sidebar (messages/settings/files) is not ported yet.
      isSelfProfile: false,
      handler: {
        user: {
          _id: viewer?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => permission === USER_PERMS.PERM_VIEW_USER_PRIVATE_INFO && Boolean(viewer),
          hasPriv: (privilege: number) => privilege === 1 && Boolean(viewer),
          own: (doc?: unknown) => Boolean(viewer && doc && (doc as { owner?: number }).owner === viewer.id),
        },
        args: { domainId: 'system' },
        request: {
          query: {},
          path: `/user/${profile.id}`,
          headers: { 'user-agent': navigator.userAgent },
        },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: USER_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2 },
      model: {
        builtin: { USER_GENDER_OTHER: 0, USER_GENDER_RANGE: {}, USER_GENDER_ICONS: {} },
      },
      avatarUrl,
      UiContext: {},
      UserContext: viewer,
    }
  }, [profile, viewer])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    switch (name) {
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? uid))}`
      case 'problem_detail': return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
      case 'contest_detail': return `/contest/${encodeURIComponent(String(args.tid ?? ''))}`
      case 'homework_detail': return `/homework/${encodeURIComponent(String(args.tid ?? ''))}`
      default: return '#'
    }
  }, [uid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const tabs = root.querySelector('.section__tabs')
    if (!tabs || !uid) return
    const item = document.createElement('li')
    item.className = 'section__tab-item'
    item.innerHTML = '<h1 class="section__tab-title">活跃日历</h1><div class="section__tab-main"><div class="section__body"></div></div>'
    tabs.appendChild(item)
    const mount = item.querySelector('.section__body') as HTMLElement
    const calendar = createRoot(mount)
    calendar.render(<ActivityCalendar uid={Number(uid)} />)
    return () => {
      calendar.unmount()
      item.remove()
    }
  }, [uid])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!profile) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return <HydroTemplate template="user_detail.html" context={context} url={buildUrl} onReady={handleReady} />
}
