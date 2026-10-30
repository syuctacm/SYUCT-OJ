// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to the login request and error state.
import { useCallback, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { safeLoginRedirect } from '../login-redirect.ts'

export function LoginPage() {
  const { login, passkeyLogin } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const redirect = safeLoginRedirect(searchParams.get('redirect'))
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [tfaRequired, setTfaRequired] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const passkeyRef = useRef<() => void>(() => {})
  const context = useMemo(() => ({
    model: { system: { get: (key: string) => key === 'server.login' } },
    loginMethods: [],
    handler: { loginMethods: [] },
  }), [])
  const loginUrl = useCallback((name: string) => (
    name === 'user_lostpass' ? '/lostpass' : name === 'user_oauth' ? '/login' : '/'
  ), [])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    if (tfaRequired && form) {
      const hidden = form.querySelector<HTMLInputElement>('input[name="tfa"]')
      const submitRow = hidden?.closest('.row')
      if (hidden && submitRow?.parentElement) {
        const row = document.createElement('div')
        row.className = 'row'
        row.innerHTML = '<div class="columns"><label class="inverse material textbox">双重验证码<input name="tfa" type="text" inputmode="numeric" autocomplete="one-time-code" maxlength="6" required></label></div>'
        submitRow.parentElement.insertBefore(row, submitRow)
        hidden.remove()
        row.querySelector<HTMLInputElement>('input')?.focus()
      }
    }
    let passkeyRow: HTMLDivElement | null = null
    let passkeyButton: HTMLButtonElement | null = null
    const onPasskey = () => passkeyRef.current()
    if (form && window.isSecureContext && 'PublicKeyCredential' in window) {
      const submitInput = form.querySelector<HTMLInputElement>('input[name="login_submit"]')
      const submitRow = submitInput?.closest('.row')
      if (submitRow?.parentElement) {
        passkeyRow = document.createElement('div')
        passkeyRow.className = 'row'
        passkeyRow.innerHTML = '<div class="columns"><button type="button" class="expanded rounded button inverse" data-passkey-login>使用 Passkey 登录</button></div>'
        passkeyButton = passkeyRow.querySelector('button')
        passkeyButton?.addEventListener('click', onPasskey)
        submitRow.parentElement.insertBefore(passkeyRow, submitRow.nextSibling)
      }
    }
    return () => {
      form?.removeEventListener('submit', onSubmit)
      passkeyButton?.removeEventListener('click', onPasskey)
      passkeyRow?.remove()
    }
  }, [tfaRequired])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const username = String(data.get('uname') ?? '').trim()
    const password = String(data.get('password') ?? '')
    const tfa = String(data.get('tfa') ?? '').trim()
    setSubmitting(true)
    setError('')
    try {
      await login(username, password, tfa)
      navigate(redirect, { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : ''
      if (message === '2FA required') {
        setTfaRequired(true)
        setError('请输入双重验证码')
      } else if (message === 'invalid 2FA code') {
        setTfaRequired(true)
        setError('双重验证码错误')
      } else {
        setError(message === 'invalid credentials' ? '用户名或密码错误' : message || '登录失败')
      }
    } finally {
      setSubmitting(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  async function handlePasskeyLogin() {
    const form = document.querySelector<HTMLFormElement>('.immersive--content form')
    const username = String(new FormData(form ?? undefined).get('uname') ?? '').trim()
    if (!username) {
      setError('请先填写用户名')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      await passkeyLogin(username)
      navigate(redirect, { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : ''
      if (message === 'passkey login is not available for this account') {
        setError('该账号没有可用的 Passkey')
      } else if (message.toLowerCase().includes('cancel') || message.toLowerCase().includes('abort')) {
        setError('Passkey 登录已取消')
      } else {
        setError(message || 'Passkey 登录失败')
      }
    } finally {
      setSubmitting(false)
    }
  }
  passkeyRef.current = () => void handlePasskeyLogin()

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {submitting ? <div className="row"><div className="columns"><blockquote className="note"><p>登录中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="user_login.html"
        context={context}
        url={loginUrl}
        onReady={handleReady}
      />
    </>
  )
}
