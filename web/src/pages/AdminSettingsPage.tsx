// Page markup is rendered from the pinned Hydro ui-default template
// (manage_setting.html + partials/setting.html). Only settings consumed by this
// single-site frontend are exposed.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { getAdminSettings, updateAdminSettings, type AdminSettings } from '../api.ts'
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
  { family: 'setting_basic', key: 'serverUrl', type: 'text', name: 'Server URL', desc: 'Canonical base URL, for example https://oj.example.com', value: '/', flag: 0 },
  { family: 'setting_basic', key: 'navLogo', type: 'text', name: 'Navigation Logo URL', value: '/nav_logo_dark.png', flag: 0 },
  { family: 'setting_basic', key: 'gravatarUrl', type: 'text', name: 'Gravatar URL Prefix', value: '//cn.gravatar.com/avatar/', flag: 0 },
  { family: 'setting_basic', key: 'footerExtraHtml', type: 'textarea', name: 'Footer Extra HTML', value: '', flag: 0 },
  { family: 'setting_basic', key: 'bulletin', type: 'markdown', name: 'Bulletin', value: '', flag: 0 },
  { family: 'setting_basic', key: 'aboutMarkdown', type: 'markdown', name: 'About', value: '', flag: 0 },
  { family: 'setting_basic', key: 'enableScratchpad', type: 'boolean', name: 'Enable Scratchpad Mode', value: true, flag: 0 },
  { family: 'setting_basic', key: 'homepageYaml', type: 'textarea', subType: 'yaml', name: 'Homepage Config', value: '', flag: 0 },
  { family: 'setting_basic', key: 'languageYaml', type: 'textarea', subType: 'yaml', name: 'Language Config', value: '', flag: 0 },
  { family: 'setting_basic', key: 'problemCategoriesYaml', type: 'textarea', subType: 'yaml', name: 'Problem Categories', value: '{}', flag: 0 },
  { family: 'setting_basic', key: 'trainingEnrolledUsers', type: 'boolean', name: 'Show enrolled users for training', value: true, flag: 0 },
  { family: 'setting_smtp', key: 'smtpHost', type: 'text', name: 'SMTP Host', value: '', flag: 0 },
  { family: 'setting_smtp', key: 'smtpPort', type: 'number', name: 'SMTP Port', value: 587, flag: 0 },
  { family: 'setting_smtp', key: 'smtpUsername', type: 'text', name: 'SMTP Username', value: '', flag: 0 },
  { family: 'setting_smtp', key: 'smtpPassword', type: 'password', name: 'SMTP Password', desc: 'Leave blank to keep the current password.', value: '', flag: FLAG_SECRET },
  { family: 'setting_smtp', key: 'smtpFrom', type: 'text', name: 'Mail From', value: '', flag: 0 },
  { family: 'setting_smtp', key: 'smtpSecure', type: 'boolean', name: 'Use Implicit TLS (usually port 465)', value: false, flag: 0 },
  { family: 'setting_smtp', key: 'smtpClearPassword', type: 'boolean', name: 'Clear SMTP Password', value: false, flag: 0 },
  { family: 'setting_limits', key: 'paginationProblem', type: 'number', name: 'Problems per page', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'paginationContest', type: 'number', name: 'Contests per page', value: 20, flag: 0 },
  { family: 'setting_limits', key: 'paginationDiscussion', type: 'number', name: 'Discussions per page', value: 50, flag: 0 },
  { family: 'setting_limits', key: 'paginationReply', type: 'number', name: 'Replies per page', value: 50, flag: 0 },
  { family: 'setting_limits', key: 'paginationRecord', type: 'number', name: 'Records per page', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'paginationRanking', type: 'number', name: 'Users per page', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'paginationSolution', type: 'number', name: 'Solutions per page', value: 20, flag: 0 },
  { family: 'setting_limits', key: 'paginationTraining', type: 'number', name: 'Trainings per page', value: 10, flag: 0 },
  { family: 'setting_limits', key: 'limitUserFiles', type: 'number', name: 'Max user files', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'limitUserFileSize', type: 'number', name: 'Max total user file size (bytes)', value: 134217728, flag: 0 },
  { family: 'setting_limits', key: 'limitContestFiles', type: 'number', name: 'Max contest/training files', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'limitContestFileSize', type: 'number', name: 'Max total contest/training file size (bytes)', value: 134217728, flag: 0 },
  { family: 'setting_limits', key: 'limitProblemFiles', type: 'number', name: 'Max problem files', value: 100, flag: 0 },
  { family: 'setting_limits', key: 'limitProblemFileSize', type: 'number', name: 'Max total problem file size (bytes)', value: 268435456, flag: 0 },
  { family: 'setting_limits', key: 'limitCodeLength', type: 'number', name: 'Max code length (bytes)', value: 131072, flag: 0 },
  { family: 'setting_limits', key: 'limitSubmission', type: 'number', name: 'Max submissions per minute (IP)', value: 60, flag: 0 },
  { family: 'setting_limits', key: 'limitSubmissionUser', type: 'number', name: 'Max submissions per minute (user)', value: 15, flag: 0 },
  { family: 'setting_limits', key: 'limitPretest', type: 'number', name: 'Max pretests per minute', value: 60, flag: 0 },
  { family: 'setting_session', key: 'sessionSavedExpireSeconds', type: 'number', name: 'Saved session expiry (seconds)', value: 2592000, flag: 0 },
  { family: 'setting_session', key: 'sessionUnsavedExpireSeconds', type: 'number', name: 'Unsaved/judge session expiry (seconds)', value: 10800, flag: 0 },
  { family: 'setting_limits', key: 'limitByUser', type: 'boolean', name: 'Use per-user rate limits', value: false, flag: 0 },
]

const STRING_KEYS = ['siteName', 'serverUrl', 'navLogo', 'gravatarUrl', 'footerExtraHtml', 'bulletin', 'aboutMarkdown', 'homepageYaml', 'languageYaml', 'problemCategoriesYaml'] as const
const PAGINATION_KEYS = ['paginationProblem', 'paginationContest', 'paginationDiscussion', 'paginationReply', 'paginationRecord', 'paginationRanking', 'paginationSolution', 'paginationTraining'] as const
const FILE_COUNT_KEYS = ['limitUserFiles', 'limitContestFiles', 'limitProblemFiles'] as const
const SIZE_KEYS = ['limitUserFileSize', 'limitContestFileSize', 'limitProblemFileSize', 'limitCodeLength'] as const
const RATE_KEYS = ['limitSubmission', 'limitSubmissionUser', 'limitPretest'] as const
const SESSION_KEYS = ['sessionSavedExpireSeconds', 'sessionUnsavedExpireSeconds'] as const

export function AdminSettingsPage() {
  const { user } = useAuth()
  const { refresh: refreshSiteSettings } = useSiteSettings()
  const [settings, setSettings] = useState<AdminSettings | null>(null)
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
    const payload: Partial<AdminSettings> = {}
    for (const key of STRING_KEYS) payload[key] = String(data.get(key) ?? '')
    for (const key of PAGINATION_KEYS) {
      const value = Number(data.get(key))
      if (!Number.isInteger(value) || value < 1 || value > 1000) {
        setError('分页大小必须是 1 到 1000 之间的整数')
        return
      }
      payload[key] = value
    }
    for (const key of FILE_COUNT_KEYS) {
      const value = Number(data.get(key))
      if (!Number.isInteger(value) || value < 1 || value > 10000) {
        setError('文件数量限制必须是 1 到 10000 之间的整数')
        return
      }
      payload[key] = value
    }
    for (const key of SIZE_KEYS) {
      const value = Number(data.get(key))
      if (!Number.isSafeInteger(value) || value < 1 || value > 10 * 1024 * 1024 * 1024) {
        setError('大小限制必须是有效的字节数，且不超过 10 GiB')
        return
      }
      payload[key] = value
    }
    for (const key of RATE_KEYS) {
      const value = Number(data.get(key))
      if (!Number.isInteger(value) || value < 1 || value > 10000) {
        setError('频率限制必须是 1 到 10000 之间的整数')
        return
      }
      payload[key] = value
    }
    for (const key of SESSION_KEYS) {
      const value = Number(data.get(key))
      if (!Number.isInteger(value) || value < 60 || value > 31536000) {
        setError('会话过期时间必须是 60 到 31536000 秒之间的整数')
        return
      }
      payload[key] = value
    }
    const smtpPort = Number(data.get('smtpPort'))
    if (!Number.isInteger(smtpPort) || smtpPort < 1 || smtpPort > 65535) {
      setError('SMTP 端口必须是 1 到 65535 之间的整数')
      return
    }
    payload.smtpHost = String(data.get('smtpHost') ?? '').trim()
    payload.smtpPort = smtpPort
    payload.smtpUsername = String(data.get('smtpUsername') ?? '').trim()
    payload.smtpFrom = String(data.get('smtpFrom') ?? '').trim()
    const smtpPassword = String(data.get('smtpPassword') ?? '')
    if (smtpPassword) payload.smtpPassword = smtpPassword
    payload.enableScratchpad = data.has('enableScratchpad')
    payload.trainingEnrolledUsers = data.has('trainingEnrolledUsers')
    payload.limitByUser = data.has('limitByUser')
    payload.smtpSecure = data.has('smtpSecure')
    payload.smtpClearPassword = data.has('smtpClearPassword')
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
