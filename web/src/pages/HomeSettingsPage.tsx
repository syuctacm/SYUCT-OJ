// Page markup is rendered from the pinned Hydro ui-default template
// (home_settings.html). The setting schema is the account subset of
// packages/hydrooj/src/model/setting.ts.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { updateUserSettings, uploadUserAvatar, type UserSettingsInput } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'

const FLAG_HIDDEN = 1
const FLAG_DISABLED = 2
const FLAG_SECRET = 4

// packages/common/status.ts
const GENDER_RANGE: Record<number, string> = { 0: 'Boy ♂', 1: 'Girl ♀', 2: 'Other' }

type SettingSchema = {
  family: string
  key: string
  type: string
  name: string
  desc?: string | null
  value: unknown
  range?: Record<number, string>
  flag?: number
}

const ACCOUNT_SETTINGS: SettingSchema[] = [
  {
    family: 'setting_info', key: 'avatar', type: 'text', name: 'Avatar', flag: 0, value: '',
    desc: 'Allow using gravatar:email qq:id github:name url:link format.',
  },
  { family: 'setting_info', key: 'qq', type: 'text', name: 'QQ', flag: 0, value: null },
  { family: 'setting_info', key: 'gender', type: 'select', name: 'Gender', flag: 0, value: 2, range: GENDER_RANGE },
  { family: 'setting_info', key: 'bio', type: 'markdown', name: 'Bio', flag: 0, value: null },
  { family: 'setting_info', key: 'school', type: 'text', name: 'School', flag: 0, value: '' },
  { family: 'setting_info', key: 'studentId', type: 'text', name: 'Student ID', flag: 0, value: '' },
  { family: 'setting_info', key: 'phone', type: 'text', name: 'Phone', flag: FLAG_DISABLED, value: null },
  {
    family: 'setting_customize', key: 'backgroundImage', type: 'text', name: 'Profile Background Image',
    flag: 0, value: '/components/profile/backgrounds/1.jpg',
    desc: 'Choose the background image in your profile page.',
  },
]

const EDITABLE_KEYS = ['avatar', 'qq', 'bio', 'school', 'studentId', 'backgroundImage'] as const

export function HomeSettingsPage() {
  const { user, loading, refresh } = useAuth()
  const navigate = useNavigate()
  const [saved, setSaved] = useState('')
  const [error, setError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    if (!loading && !user) navigate('/login')
  }, [loading, navigate, user])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = [hydrateHydroEditForm(root)]
    const avatarButton = root.querySelector<HTMLButtonElement>('button.change-avatar')
    const onAvatarClick = () => {
      const picker = document.createElement('input')
      picker.type = 'file'
      picker.accept = '.jpg,.jpeg,.png,image/jpeg,image/png'
      picker.onchange = async () => {
        const file = picker.files?.[0]
        if (!file) return
        setSaved('')
        setError('')
        try {
          await uploadUserAvatar(file)
          await refresh()
          setSaved('头像已更新')
        } catch (err) {
          setError(err instanceof Error ? err.message : '头像上传失败')
        }
      }
      picker.click()
    }
    avatarButton?.addEventListener('click', onAvatarClick)
    cleanups.push(() => avatarButton?.removeEventListener('click', onAvatarClick))
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [refresh])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const data = new FormData(event.target as HTMLFormElement)
    const payload: UserSettingsInput = {}
    for (const key of EDITABLE_KEYS) {
      if (data.has(key)) payload[key] = String(data.get(key) ?? '')
    }
    if (data.has('gender')) payload.gender = Number(data.get('gender'))
    setSaved('')
    setError('')
    try {
      await updateUserSettings(payload)
      setSaved('已保存')
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败')
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'homepage': return '/'
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? user?.id ?? ''))}`
      case 'home_settings': {
        const category = String(args.category ?? 'account')
        return `/home/settings?category=${encodeURIComponent(category)}`
      }
      case 'home_messages': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/home/messages${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'home_files': return '/file'
      // Domain and security pages are not ported yet.
      case 'home_domain':
      case 'home_security':
        return '/home/security'
      default: return '#'
    }
  }, [user?.id])

  const context = useMemo(() => ({
    page_name: 'home_account',
    category: 'account',
    current: { ...(user ?? {}) },
    settings: ACCOUNT_SETTINGS,
    model: { setting: { FLAG_HIDDEN, FLAG_DISABLED, FLAG_SECRET } },
    handler: {
      user: {
        _id: user?.id ?? 0,
        uname: user?.username ?? '',
        displayName: user?.displayName ?? '',
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPriv: (privilege: number) => Boolean(user) && (privilege === 1 || privilege === 2),
        hasPerm: () => false,
        own: () => true,
      },
      args: { domainId: 'system' },
      request: {
        query: { category: 'account' },
        path: '/home/settings',
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    },
    perm: {},
    PRIV: { PRIV_USER_PROFILE: 1, PRIV_CREATE_FILE: 2, PRIV_EDIT_SYSTEM: 3 },
    avatarUrl,
    UiContext: {},
    UserContext: user,
  }), [user])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!user) return <div className="row"><div className="columns"><div className="nothing-placeholder">需要登录</div></div></div>
  return (
    <>
      {saved ? <div className="row"><div className="columns"><blockquote className="note"><p>{saved}</p></blockquote></div></div> : null}
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="home_settings.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
