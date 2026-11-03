// Page markup is rendered from the pinned Hydro ui-default template
// (manage_setting.html + partials/setting.html). Only settings consumed by this
// single-site frontend are exposed.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { getAdminSettings, updateAdminSettings, type PublicSettings } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { useSiteSettings } from '../site-settings.tsx'
import { ErrorPage } from './ErrorPage.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'

const FLAG_HIDDEN = 1
const FLAG_DISABLED = 2
const FLAG_SECRET = 4

const SYSTEM_SETTINGS = [
  { family: 'setting_basic', key: 'siteName', type: 'text', name: 'Site Name', value: 'SYUCT ACM OJ', flag: 0 },
  { family: 'setting_basic', key: 'navLogo', type: 'text', name: 'Navigation Logo URL', value: '/nav_logo_dark.png', flag: 0 },
  { family: 'setting_basic', key: 'gravatarUrl', type: 'text', name: 'Gravatar URL Prefix', value: '//cn.gravatar.com/avatar/', flag: 0 },
  { family: 'setting_basic', key: 'footerExtraHtml', type: 'textarea', name: 'Footer Extra HTML', value: '', flag: 0 },
  { family: 'setting_basic', key: 'bulletin', type: 'markdown', name: 'Bulletin', value: '', flag: 0 },
  { family: 'setting_basic', key: 'aboutMarkdown', type: 'markdown', name: 'About', value: '', flag: 0 },
  { family: 'setting_basic', key: 'enableScratchpad', type: 'boolean', name: 'Enable Scratchpad Mode', value: true, flag: 0 },
  { family: 'setting_basic', key: 'homepageYaml', type: 'textarea', subType: 'yaml', name: 'Homepage Config', value: '', flag: 0 },
  { family: 'setting_limits', key: 'paginationProblem', type: 'number', name: 'Problems per page', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'paginationContest', type: 'number', name: 'Contests per page', value: 20, flag: 0 },
  { family: 'setting_limits', key: 'paginationDiscussion', type: 'number', name: 'Discussions per page', value: 50, flag: 0 },
  { family: 'setting_limits', key: 'paginationRecord', type: 'number', name: 'Records per page', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'paginationRanking', type: 'number', name: 'Users per page', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'paginationSolution', type: 'number', name: 'Solutions per page', value: 20, flag: 0 },
  { family: 'setting_limits', key: 'paginationTraining', type: 'number', name: 'Trainings per page', value: 10, flag: 0 },
]

const STRING_KEYS = ['siteName', 'navLogo', 'gravatarUrl', 'footerExtraHtml', 'bulletin', 'aboutMarkdown', 'homepageYaml'] as const
const NUMBER_KEYS = ['paginationProblem', 'paginationContest', 'paginationDiscussion', 'paginationRecord', 'paginationRanking', 'paginationSolution', 'paginationTraining'] as const

export function AdminSettingsPage() {
  const { user } = useAuth()
  const { refresh: refreshSiteSettings } = useSiteSettings()
  const [settings, setSettings] = useState<PublicSettings | null>(null)
  const [saved, setSaved] = useState('')
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    getAdminSettings()
      .then(setSettings)
      .catch((reason: unknown) => setError(reason instanceof Error ? reason.message : '设置加载失败'))
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
    const payload: Partial<PublicSettings> = {}
    for (const key of STRING_KEYS) payload[key] = String(data.get(key) ?? '')
    for (const key of NUMBER_KEYS) {
      const value = Number(data.get(key))
      if (!Number.isInteger(value) || value < 1 || value > 1000) {
        setError('分页大小必须是 1 到 1000 之间的整数')
        return
      }
      payload[key] = value
    }
    payload.enableScratchpad = data.has('enableScratchpad')
    setSaved('')
    setError('')
    try {
      const next = await updateAdminSettings(payload)
      setSettings(next)
      await refreshSiteSettings()
      setSaved('已保存')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '保存失败')
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  const buildUrl = useCallback((name: string) => {
    if (name === 'manage_dashboard') return '/admin'
    if (name === 'manage_user_priv') return '/admin/users'
    if (name === 'manage_user_import') return '/admin/users/import'
    if (name === 'manage_audit') return '/admin/audit'
    if (name === 'manage_setting') return '/admin/settings'
    if (name === 'manage_discussion_nodes') return '/admin/discussion-nodes'
    if (name === 'manage_ip_login') return '/admin/ip-login'
    return '#'
  }, [])

  const context = useMemo(() => ({
    page_name: 'manage_setting',
    current: settings ?? {},
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
  }), [settings, user])

  if (!user || ((user.priv ?? 0) & 1) === 0) {
    return <ErrorPage status={403} />
  }
  if (!settings) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '加载中...'}</div></div></div>
  }
  return (
    <>
      {saved ? <div className="row"><div className="columns"><blockquote className="note"><p>{saved}</p></blockquote></div></div> : null}
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate template="manage_setting.html" context={context} url={buildUrl} onReady={handleReady} />
    </>
  )
}
