// Page markup is rendered from the pinned Hydro ui-default template
// (manage_dashboard.html). Keep this file limited to data mapping and the small
// interactions that template expects.
import { useCallback, useMemo } from 'react'
import { useAuth } from '../auth.tsx'
import { ErrorPage } from './ErrorPage.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

export function AdminPage() {
  const { user } = useAuth()

  const handleReady = useCallback((root: HTMLDivElement) => {
    // Restarting the service from the web UI is not ported; drop the toolbar
    // instead of leaving a dead button behind.
    root.querySelectorAll('form').forEach((form) => {
      if (form.querySelector('input[value="restart"]')) form.remove()
    })
  }, [])

  const buildUrl = useCallback((name: string) => {
    if (name === 'manage_dashboard') return '/admin'
    if (name === 'manage_user_priv') return '/admin/users'
    if (name === 'manage_user_import') return '/admin/users/import'
    if (name === 'manage_setting') return '/admin/settings'
    if (name === 'manage_audit') return '/admin/audit'
    return '/'
  }, [])

  const context = useMemo(() => ({
    page_name: 'manage_dashboard',
    domain: { name: 'SYUCT ACM', avatar: '' },
    avatarUrl,
    handler: {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        hasPerm: () => false,
        own: () => false,
      },
      args: { domainId: 'system' },
      request: { query: {}, path: '/admin', headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    },
    UiContext: {},
    UserContext: user,
  }), [user])

  if (!user || ((user.priv ?? 0) & 1) === 0) {
    return <ErrorPage status={403} />
  }
  return (
    <HydroTemplate
      template="manage_dashboard.html"
      context={context}
      url={buildUrl}
      onReady={handleReady}
    />
  )
}
