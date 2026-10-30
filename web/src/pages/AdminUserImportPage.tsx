// Page markup is rendered from the pinned ui-default template
// (manage_user_import.html); this file only wires the Preview/Submit buttons.
import { useCallback, useMemo } from 'react'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { importUsers } from '../api.ts'
import { avatarUrl } from '../upstream/avatar.ts'

export function AdminUserImportPage() {
  const { user } = useAuth()

  const handleReady = useCallback((root: HTMLDivElement) => {
    const textarea = root.querySelector<HTMLTextAreaElement>('textarea[name="users"]')
    const messages = root.querySelector<HTMLElement>('pre[name="messages"]')
    if (!textarea || !messages) return
    const cleanups: Array<() => void> = []
    for (const button of root.querySelectorAll<HTMLButtonElement>('button[name="preview"], button[name="submit"]')) {
      const onClick = async () => {
        const draft = button.name === 'preview'
        messages.textContent = draft ? '正在检查...' : '正在导入...'
        try {
          const result = await importUsers(textarea.value, draft)
          messages.textContent = result.messages.join('\n')
        } catch (err: unknown) {
          messages.textContent = err instanceof Error ? err.message : '导入失败'
        }
      }
      button.addEventListener('click', onClick)
      cleanups.push(() => button.removeEventListener('click', onClick))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  const context = useMemo(() => ({
    page_name: 'manage_user_import',
    handler: {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        hasPerm: () => false,
        own: () => false,
      },
      args: { domainId: 'system' },
      request: { query: {}, path: '/admin/users/import', headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    },
    avatarUrl,
    UiContext: {},
    UserContext: user,
  }), [user])

  const buildUrl = useCallback((name: string) => {
    if (name === 'manage_dashboard') return '/admin'
    if (name === 'manage_user_priv') return '/admin/users'
    if (name === 'manage_user_import') return '/admin/users/import'
    if (name === 'manage_audit') return '/admin/audit'
    if (name === 'manage_setting') return '/admin/settings'
    return '/'
  }, [])

  return <HydroTemplate template="manage_user_import.html" context={context} url={buildUrl} onReady={handleReady} />
}
