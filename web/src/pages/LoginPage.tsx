// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to the login request and error state.
import { useCallback, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [tfaRequired, setTfaRequired] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const context = useMemo(() => ({
    model: { system: { get: (key: string) => key === 'server.login' } },
    loginMethods: [],
    handler: { loginMethods: [] },
  }), [])
  const loginUrl = useCallback((name: string) => (
    name === 'user_lostpass' ? '/login?lostpass=1' : name === 'user_oauth' ? '/login' : '/'
  ), [])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    // The Go backend does not expose password recovery yet; keep the template
    // structure but remove the dead recovery link.
    root.querySelector('a[href="/login?lostpass=1"]')?.closest('.row')?.remove()
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
    return () => form?.removeEventListener('submit', onSubmit)
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
      navigate('/')
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
