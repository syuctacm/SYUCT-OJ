// Page markup is rendered from the pinned Hydro ui-default template
// (home_security.html). Only the password and active-session sections are
// wired; email, OAuth and authenticator sections are removed below.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  changePassword, getSecuritySessions, revokeAllSecuritySessions, revokeSecuritySession,
  type SecuritySession,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

function parseUserAgent(ua: string) {
  const value = ua || ''
  let os = { name: 'Unknown', version: '' }
  if (/Windows NT 10\.0/.test(value)) os = { name: 'Windows', version: '10' }
  else if (/Windows NT 6\.3/.test(value)) os = { name: 'Windows', version: '8.1' }
  else if (/Windows NT 6\.1/.test(value)) os = { name: 'Windows', version: '7' }
  else if (/Android ([\d.]+)/.test(value)) os = { name: 'Android', version: RegExp.$1 }
  else if (/(iPhone|iPad|iPod).*OS ([\d_]+)/.test(value)) {
    os = { name: 'iOS', version: RegExp.$2.replace(/_/g, '.') }
  } else if (/Mac OS X ([\d_.]+)/.test(value)) {
    os = { name: 'Mac OS', version: RegExp.$1.replace(/_/g, '.') }
  } else if (/Linux/.test(value)) os = { name: 'Linux', version: '' }
  let browser = { name: 'Unknown', version: '' }
  if (/Edg\/([\d.]+)/.test(value)) browser = { name: 'Edge', version: RegExp.$1 }
  else if (/OPR\/([\d.]+)/.test(value)) browser = { name: 'Opera', version: RegExp.$1 }
  else if (/Firefox\/([\d.]+)/.test(value)) browser = { name: 'Firefox', version: RegExp.$1 }
  else if (/Chrome\/([\d.]+)/.test(value)) browser = { name: 'Chrome', version: RegExp.$1 }
  else if (/Version\/([\d.]+).*Safari/.test(value)) browser = { name: 'Safari', version: RegExp.$1 }
  return { os, browser }
}

function platformIcon(platform: string) {
  const key = String(platform || '').toLowerCase()
  return key === 'mac os' ? 'mac' : key
}

export function SecurityPage() {
  const { user, loading } = useAuth()
  const navigate = useNavigate()
  const [sessions, setSessions] = useState<SecuritySession[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    if (!loading && !user) navigate('/login')
  }, [loading, navigate, user])

  useEffect(() => {
    if (!user) return
    getSecuritySessions()
      .then(setSessions)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '会话加载失败'))
  }, [reloadKey, user])

  const context = useCallback(() => ({
    page_name: 'home_security',
    sessions: sessions.map((session) => ({
      _id: session.id,
      isCurrent: session.current,
      updateAt: session.updateAt,
      updateIp: session.ip || '未知',
      updateGeoip: { display: '未知' },
      createHost: '',
      updateUaInfo: parseUserAgent(session.ua),
    })),
    sudoUid: null,
    relations: [],
    loginMethods: [],
    authenticators: [],
    geoipProvider: null,
    platformIcon,
    handler: {
      user: {
        _id: user?.id ?? 0,
        uname: user?.username ?? '',
        mail: user?.email ?? '',
        avatar: user?.avatar ?? '',
        viewLang: 'zh',
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        hasPerm: () => false,
      },
      request: { path: '/home/security', query: {} },
    },
    PRIV: { PRIV_CREATE_FILE: 1 },
    avatarUrl,
  }), [sessions, user])

  async function handleSubmit(event: SubmitEvent) {
    const form = event.target as HTMLFormElement
    if (!(form instanceof HTMLFormElement)) return
    event.preventDefault()
    const data = new FormData(form)
    const operation = String(data.get('operation') ?? '')
    setError('')
    setNotice('')
    if (operation === 'change_password') {
      try {
        await changePassword(
          String(data.get('current') ?? ''),
          String(data.get('password') ?? ''),
          String(data.get('verifyPassword') ?? ''),
        )
        navigate('/login')
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : '修改密码失败')
      }
      return
    }
    if (operation === 'delete_token') {
      try {
        await revokeSecuritySession(String(data.get('tokenDigest') ?? ''))
        setNotice('已退出该会话')
        setReloadKey((key) => key + 1)
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : '退出会话失败')
      }
      return
    }
    if (operation === 'delete_all_tokens') {
      try {
        await revokeAllSecuritySessions()
        navigate('/login')
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : '退出全部会话失败')
      }
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  const handleReady = useCallback((root: HTMLDivElement) => {
    // Email/OAuth/2FA are not part of the rewrite yet; drop their sections so
    // the page does not show dead controls.
    root.querySelector('#linked-accounts')?.closest('.section')?.remove()
    root.querySelector('#authenticator')?.closest('.section')?.remove()
    const cleanups: Array<() => void> = []
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  const buildUrl = useCallback((name: string) => {
    switch (name) {
      case 'homepage': return '/'
      case 'user_detail': return `/user/${encodeURIComponent(String(user?.id ?? ''))}`
      case 'home_messages': return '/home/messages'
      case 'home_settings': return '/home/settings?category=account'
      case 'home_files': return '/file'
      case 'home_security': return '/home/security'
      default: return '#'
    }
  }, [user?.id])

  if (loading || !user) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {notice ? <div className="row"><div className="columns"><blockquote className="note"><p>{notice}</p></blockquote></div></div> : null}
      <HydroTemplate template="home_security.html" context={context()} url={buildUrl} onReady={handleReady} />
    </>
  )
}
