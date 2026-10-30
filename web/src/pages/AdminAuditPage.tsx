import { useEffect, useMemo, useState } from 'react'
import { useAuth } from '../auth.tsx'
import { ErrorPage } from './ErrorPage.tsx'
import { getAdminAudit, type AdminAuditPage as AuditPage } from '../api.ts'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function AdminAuditPage() {
  const { user } = useAuth()
  const [data, setData] = useState<AuditPage | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    setError('')
    getAdminAudit(1, 100)
      .then(setData)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '审计日志加载失败'))
  }, [])

  const context = useMemo(() => ({
    page_name: 'manage_audit',
    logs: data?.items ?? [],
    handler: {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        hasPerm: () => false,
        own: () => false,
      },
      args: { domainId: 'system' },
      request: { query: {}, path: '/admin/audit', headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    },
    UiContext: {},
    UserContext: user,
  }), [data, user])

  const buildUrl = useMemo(() => (name: string) => {
    if (name === 'manage_dashboard') return '/admin'
    if (name === 'manage_user_priv') return '/admin/users'
    if (name === 'manage_user_import') return '/admin/users/import'
    if (name === 'manage_setting') return '/admin/settings'
    if (name === 'manage_audit') return '/admin/audit'
    return '/'
  }, [])

  if (!user || ((user.priv ?? 0) & 1) === 0) {
    return <ErrorPage status={403} />
  }
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!data) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  return <HydroTemplate template="manage_audit.html" context={context} url={buildUrl} />
}
