// Page markup is rendered from the pinned Hydro ui-default template
// (home_security.html). Password, TOTP/Passkey authenticators and active-session
// sections are wired; email and OAuth remain outside the current scope.
import { useCallback, useEffect, useRef, useState } from 'react'
import { startRegistration } from '@simplewebauthn/browser'
import { useNavigate } from 'react-router-dom'
import {
  beginPasskeyRegistration, changePassword, deletePasskey, disableTFA, enableTFA, finishPasskeyRegistration,
  getPasskeys, getSecuritySessions, requestEmailChange, revokeAllSecuritySessions, revokeSecuritySession,
  setupTFA, type Passkey, type SecuritySession, type TFASetup,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { Dialog } from '../components/Dialog.tsx'
import { confirmDialog, promptDialog } from '../components/dialogs.tsx'
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
  const { user, loading, refresh } = useAuth()
  const navigate = useNavigate()
  const [sessions, setSessions] = useState<SecuritySession[]>([])
  const [authenticators, setAuthenticators] = useState<Passkey[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [authChoiceOpen, setAuthChoiceOpen] = useState(false)
  const [authChoice, setAuthChoice] = useState<'tfa' | 'passkey'>('tfa')
  const [passkeyNameOpen, setPasskeyNameOpen] = useState(false)
  const [passkeyName, setPasskeyName] = useState('Passkey')
  const [passkeyBusy, setPasskeyBusy] = useState(false)
  const passkeyBusyRef = useRef(false)
  const [passkeyError, setPasskeyError] = useState('')
  const [tfaSetup, setTfaSetup] = useState<TFASetup | null>(null)
  const [tfaCode, setTfaCode] = useState('')
  const [tfaError, setTfaError] = useState('')
  const [tfaBusy, setTfaBusy] = useState(false)
  const [mailOpen, setMailOpen] = useState(false)
  const [mailCurrent, setMailCurrent] = useState('')
  const [mailAddress, setMailAddress] = useState('')
  const [mailError, setMailError] = useState('')
  const [mailBusy, setMailBusy] = useState(false)
  const [mailLink, setMailLink] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const authSetupRef = useRef<() => void>(() => {})
  const passkeySupported = typeof window !== 'undefined' && window.isSecureContext && 'PublicKeyCredential' in window

  useEffect(() => {
    if (!loading && !user) navigate('/login')
  }, [loading, navigate, user])

  useEffect(() => {
    if (!user) return
    getSecuritySessions()
      .then(setSessions)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '会话加载失败'))
    getPasskeys()
      .then(setAuthenticators)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Passkey 加载失败'))
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
    authenticators: authenticators.map((authenticator) => ({
      name: authenticator.name,
      authenticatorAttachment: authenticator.attachment === 'platform' ? 'platform' : 'cross-platform',
      credentialDeviceType: authenticator.deviceType || 'singleDevice',
      fmt: authenticator.format || 'none',
      regat: new Date(authenticator.createdAt),
      credentialID: {
        buffer: { toString: () => authenticator.id },
      },
    })),
    geoipProvider: null,
    platformIcon,
    handler: {
      user: {
        _id: user?.id ?? 0,
        uname: user?.username ?? '',
        mail: user?.email ?? '',
        tfa: Boolean(user?.tfa),
        avatar: user?.avatar ?? '',
        viewLang: 'zh',
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        hasPerm: () => false,
      },
      request: { path: '/home/security', query: {} },
    },
    PRIV: { PRIV_CREATE_FILE: 1 },
    avatarUrl,
  }), [authenticators, sessions, user])

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
    if (operation === 'disable_tfa') {
      const current = await promptDialog('请输入当前密码以关闭双重验证')
      if (!current) return
      try {
        await disableTFA(current)
        await refresh()
        setNotice('双重验证已关闭')
      } catch (err) {
        setError(err instanceof Error ? err.message : '关闭双重验证失败')
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
      return
    }
    if (operation === 'disable_authn') {
      if (!(await confirmDialog('确定要删除这个 Passkey 吗？'))) return
      try {
        await deletePasskey(String(data.get('id') ?? ''))
        setNotice('Passkey 已删除')
        setReloadKey((key) => key + 1)
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : '删除 Passkey 失败')
      }
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  async function openTfaSetup() {
    setTfaError('')
    setTfaBusy(true)
    try {
      const setup = await setupTFA()
      setTfaCode('')
      setTfaSetup(setup)
    } catch (err) {
      setError(err instanceof Error ? err.message : '双重验证初始化失败')
    } finally {
      setTfaBusy(false)
    }
  }
  function openAuthenticatorChoice() {
    setAuthChoice(user?.tfa ? 'passkey' : 'tfa')
    setAuthChoiceOpen(true)
  }
  authSetupRef.current = openAuthenticatorChoice

  async function confirmTfaSetup() {
    if (!tfaSetup) return
    if (!/^\d{6}$/.test(tfaCode.trim())) {
      setTfaError('请输入 6 位验证码')
      return
    }
    setTfaBusy(true)
    setTfaError('')
    try {
      await enableTFA(tfaSetup.secret, tfaCode.trim())
      setTfaSetup(null)
      await refresh()
      setNotice('双重验证已启用')
    } catch (err) {
      setTfaError(err instanceof Error ? err.message : '双重验证启用失败')
    } finally {
      setTfaBusy(false)
    }
  }

  async function submitEmailChange() {
    setMailError('')
    if (!mailCurrent || !mailAddress.trim()) {
      setMailError('请输入当前密码和新邮箱')
      return
    }
    setMailBusy(true)
    try {
      const result = await requestEmailChange(mailCurrent, mailAddress.trim())
      setMailOpen(false)
      setMailCurrent('')
      setMailAddress('')
      setMailLink(result.link ?? '')
      setNotice(result.sent ? '确认邮件已发送，请查收新邮箱' : '未配置 SMTP，请点击下方确认链接完成修改')
    } catch (err) {
      setMailError(err instanceof Error ? err.message : '修改邮箱失败')
    } finally {
      setMailBusy(false)
    }
  }

  async function registerPasskey() {
    if (passkeyBusyRef.current) return
    if (!passkeyName.trim()) {
      setPasskeyError('请输入 Passkey 名称')
      return
    }
    if (!passkeySupported) {
      setPasskeyError('当前浏览器或访问环境不支持 Passkey')
      return
    }
    passkeyBusyRef.current = true
    setPasskeyBusy(true)
    setPasskeyError('')
    try {
      const begin = await beginPasskeyRegistration()
      const credential = await startRegistration({ optionsJSON: begin.options.publicKey })
      await finishPasskeyRegistration(begin.token, passkeyName.trim(), credential)
      setPasskeyNameOpen(false)
      setPasskeyName('Passkey')
      setNotice('Passkey 已添加')
      setReloadKey((key) => key + 1)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : ''
      if (message.toLowerCase().includes('cancel') || message.toLowerCase().includes('abort')) {
        setPasskeyError('Passkey 注册已取消')
      } else {
        setPasskeyError(message || 'Passkey 注册失败')
      }
    } finally {
      passkeyBusyRef.current = false
      setPasskeyBusy(false)
    }
  }

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const changeMail = root.querySelector<HTMLButtonElement>('[data-operation="change_mail"]')
    const onChangeMail = () => {
      setMailError('')
      setMailLink('')
      setMailOpen(true)
    }
    changeMail?.addEventListener('click', onChangeMail)
    if (changeMail) cleanups.push(() => changeMail.removeEventListener('click', onChangeMail))
    const authEnable = root.querySelector<HTMLButtonElement>('[name="auth_enable"]')
    const onAuthEnable = () => authSetupRef.current()
    authEnable?.addEventListener('click', onAuthEnable)
    if (authEnable) cleanups.push(() => authEnable.removeEventListener('click', onAuthEnable))
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
      {mailLink ? <div className="row"><div className="columns"><blockquote className="note"><p>本地模式确认链接：<a href={mailLink}>{mailLink}</a></p></blockquote></div></div> : null}
      <HydroTemplate template="home_security.html" context={context()} url={buildUrl} onReady={handleReady} />
      {authChoiceOpen ? (
        <Dialog
          okLabel="下一步"
          cancelLabel="取消"
          onOk={() => {
            if (authChoice === 'passkey' && !passkeySupported) {
              setAuthChoiceOpen(false)
              setError('当前浏览器或访问环境不支持 Passkey')
              return
            }
            setAuthChoiceOpen(false)
            if (authChoice === 'tfa' && !user.tfa) {
              void openTfaSetup()
              return
            }
            setPasskeyError('')
            setPasskeyName('Passkey')
            setPasskeyNameOpen(true)
          }}
          onCancel={() => setAuthChoiceOpen(false)}
        >
          <div className="typo">
            <h3>选择认证器类型</h3>
            <label>
              <input
                type="radio"
                name="authenticator-type"
                checked={authChoice === 'tfa'}
                disabled={Boolean(user.tfa)}
                onChange={() => setAuthChoice('tfa')}
              /> 双重验证（TOTP）{user.tfa ? '（已启用）' : ''}
            </label>
            <label>
              <input
                type="radio"
                name="authenticator-type"
                checked={authChoice === 'passkey'}
                disabled={!passkeySupported}
                onChange={() => setAuthChoice('passkey')}
              /> Passkey{passkeySupported ? '' : '（当前环境不支持）'}
            </label>
          </div>
        </Dialog>
      ) : null}
      {passkeyNameOpen ? (
        <Dialog
          okLabel={passkeyBusy ? '注册中...' : '注册'}
          cancelLabel="取消"
          onOk={() => void registerPasskey()}
          onCancel={() => {
            if (!passkeyBusy) setPasskeyNameOpen(false)
          }}
        >
          <div className="typo">
            <p>为这个 Passkey 设置一个便于识别的名称。</p>
            <label>名称
              <div className="textbox-container">
                <input
                  className="textbox"
                  value={passkeyName}
                  maxLength={64}
                  onChange={(event) => setPasskeyName(event.target.value)}
                  autoFocus
                />
              </div>
            </label>
            {passkeyError ? <p className="warn">{passkeyError}</p> : null}
          </div>
        </Dialog>
      ) : null}
      {mailOpen ? (
        <Dialog okLabel="发送确认" cancelLabel="取消" onOk={() => void submitEmailChange()} onCancel={() => setMailOpen(false)}>
          <div className="typo">
            <p>修改邮箱前需要验证当前密码。</p>
            <label>当前密码
              <div className="textbox-container">
                <input className="textbox" type="password" value={mailCurrent} onChange={(event) => setMailCurrent(event.target.value)} autoFocus />
              </div>
            </label>
            <label>新邮箱
              <div className="textbox-container">
                <input className="textbox" type="email" value={mailAddress} onChange={(event) => setMailAddress(event.target.value)} autoComplete="email" />
              </div>
            </label>
            {mailError ? <p className="warn">{mailError}</p> : null}
            {mailBusy ? <p className="supplementary">正在处理...</p> : null}
          </div>
        </Dialog>
      ) : null}
      {tfaSetup ? (
        <Dialog okLabel="启用" cancelLabel="取消" onOk={() => void confirmTfaSetup()} onCancel={() => setTfaSetup(null)}>
          <div className="typo">
            <p>请使用双因素认证应用扫描下面的二维码：</p>
            <div style={{ textAlign: 'center' }}>
              <img src={tfaSetup.qr} width={256} height={256} alt="双重验证二维码" />
              <p className="supplementary">扫描失败时可手动输入密钥：<code>{tfaSetup.secret}</code></p>
            </div>
            <label>6 位验证码
              <div className="textbox-container">
                <input className="textbox" value={tfaCode} onChange={(event) => setTfaCode(event.target.value)} inputMode="numeric" autoComplete="one-time-code" maxLength={6} autoFocus />
              </div>
            </label>
            {tfaError ? <p className="warn">{tfaError}</p> : null}
            {tfaBusy ? <p className="supplementary">正在验证...</p> : null}
          </div>
        </Dialog>
      ) : null}
    </>
  )
}
