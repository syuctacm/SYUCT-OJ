// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping and request handling.
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { getPublicUser, getRanking, type PublicUser, type RankingUser } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

export function RankingPage() {
  const { user } = useAuth()
  const [searchParams] = useSearchParams()
  const page = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [items, setItems] = useState<RankingUser[]>([])
  const [total, setTotal] = useState(0)
  const [viewer, setViewer] = useState<PublicUser | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoaded(false)
    setError('')
    Promise.all([
      getRanking(page, 20),
      user ? getPublicUser(String(user.id)) : Promise.resolve(null),
    ])
      .then(([ranking, profile]) => {
        setItems(ranking.items)
        setTotal(ranking.total)
        setViewer(profile)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '榜单加载失败'))
      .finally(() => setLoaded(true))
  }, [page, user])

  const context = useMemo(() => {
    const udocs = items.map((item) => ({
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
      bio: item.bio ?? '',
      rp: item.rp,
      level: item.level,
      nAccept: item.nAccept,
      hasPriv: () => false,
      hasPerm: () => false,
    }))
    return {
      page_name: 'ranking',
      udocs,
      page,
      upcount: Math.max(1, Math.ceil(total / 20)),
      handler: {
        user: {
          _id: viewer?.id ?? 0,
          uname: viewer?.username ?? '',
          avatar: viewer?.avatar ?? '',
          bio: viewer?.bio ?? '',
          rp: viewer?.rp ?? 0,
          level: viewer?.level ?? 0,
          rank: viewer?.rank ?? 0,
          nAccept: viewer?.nAccept ?? 0,
          rpInfo: {},
          viewLang: 'zh',
          hasPriv: (privilege: number) => privilege === 1 && Boolean(viewer),
          hasPerm: () => false,
          own: () => false,
        },
        args: { domainId: 'system' },
        request: { query: {}, path: '/ranking', headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: {},
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: { rp: {}, system: { get: (key: string) => (key === 'pagination.ranking' ? 20 : '') } },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [items, page, total, user, viewer])

  const buildUrl = useMemo(() => (name: string, args: Record<string, unknown> = {}) => {
    if (name === 'user_detail') return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
    if (name === 'ranking') return '/ranking'
    return '/'
  }, [])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!loaded) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  return <HydroTemplate template="ranking.html" context={context} url={buildUrl} />
}
