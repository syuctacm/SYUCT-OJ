// Page markup is rendered from the pinned Hydro ui-default template
// (manage_setting.html + partials/setting.html). The schema only lists the
// system settings this rewrite actually stores.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { getAdminSettings, updateAdminSettings } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'

const FLAG_HIDDEN = 1
const FLAG_DISABLED = 2
const FLAG_SECRET = 4

const SYSTEM_SETTINGS = [
  {
    family: 'setting_basic',
    key: 'bulletin',
    type: 'markdown',
    name: 'Bulletin',
    desc: 'Markdown shown at the top of the homepage.',
    value: '',
    flag: 0,
  },
]

export function AdminSettingsPage() {
  const { user } = useAuth()
  const [bulletin, setBulletin] = useState<string | null>(null)
  const [saved, setSaved] = useState('')
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    getAdminSettings()
      .then((settings) => setBulletin(settings.bulletin))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '设置加载失败'))
  }, [])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = [hydrateHydroEditForm(root)]
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const data = new FormData(event.target as HTMLFormElement)
    setSaved('')
    setError('')
    try {
      await updateAdminSettings({ bulletin: String(data.get('bulletin') ?? '') })
      setBulletin(String(data.get('bulletin') ?? ''))
      setSaved('已保存')
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败')
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  const buildUrl = useCallback((name: string) => {
    if (name === 'manage_dashboard') return '/admin'
    if (name === 'manage_user_priv') return '/admin/users'
    if (name === 'manage_user_import') return '/admin/users/import'
    if (name === 'manage_audit') return '/admin/audit'
    if (name === 'manage_setting') return '/admin/settings'
    return '#'
  }, [])

  const context = useMemo(() => ({
    page_name: 'manage_setting',
    current: { bulletin: bulletin ?? '' },
    settings: SYSTEM_SETTINGS,
    model: { setting: { FLAG_HIDDEN, FLAG_DISABLED, FLAG_SECRET } },
    handler: {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        hasPerm: () => false,
        own: () => false,
      },
      args: { domainId: 'system' },
      request: { query: {}, path: '/admin/settings', headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    },
    avatarUrl,
    UiContext: {},
    UserContext: user,
  }), [bulletin, user])

  if (!user || ((user.priv ?? 0) & 1) === 0) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">需要管理员权限</div></div></div>
  }
  if (bulletin === null) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '加载中...'}</div></div></div>
  }
  return (
    <>
      {saved ? <div className="row"><div className="columns"><blockquote className="note"><p>{saved}</p></blockquote></div></div> : null}
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="manage_setting.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
