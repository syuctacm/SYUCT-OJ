// Page markup comes from the pinned ui-default template; the messagepad app
// itself is mounted into #messagePad exactly like home_messages.page.tsx does.
import { useCallback, useEffect, useMemo } from 'react'
import { createRoot } from 'react-dom/client'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { MessagePad } from '../components/MessagePad.tsx'

export function MessagesPage() {
  const { user, loading } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const target = Number(params.get('target')) || null

  useEffect(() => {
    if (!loading && !user) navigate('/login')
  }, [loading, navigate, user])

  const context = useMemo(() => ({
    page_name: 'home_messages',
    handler: {
      user: {
        _id: user?.id ?? 0,
        uname: user?.username ?? '',
        avatar: user?.avatar ?? '',
        viewLang: 'zh',
        hasPriv: (privilege: number) => Boolean(user) && privilege === 1,
        hasPerm: () => false,
      },
      request: { path: '/home/messages', query: {} },
    },
    PRIV: { PRIV_CREATE_FILE: 1 },
  }), [user])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const mount = root.querySelector<HTMLElement>('#messagePad')
    if (!mount) return
    mount.innerHTML = ''
    const app = createRoot(mount)
    app.render(<MessagePad user={user} target={target} />)
    return () => queueMicrotask(() => app.unmount())
  }, [target, user])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'homepage': return '/'
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? user?.id ?? ''))}`
      case 'home_messages': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/home/messages${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'home_settings': {
        const category = String(args.category ?? 'account')
        return `/home/settings?category=${encodeURIComponent(category)}`
      }
      case 'home_files': return '/file'
      case 'home_security': return '#'
      case 'problem_detail': return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
      default: return '#'
    }
  }, [user?.id])

  if (loading || !user) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return <HydroTemplate template="home_messages.html" context={context} url={buildUrl} onReady={handleReady} />
}
