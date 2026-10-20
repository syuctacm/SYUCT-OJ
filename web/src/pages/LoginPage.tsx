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
    return () => form?.removeEventListener('submit', onSubmit)
  }, [])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const username = String(data.get('uname') ?? '').trim()
    const password = String(data.get('password') ?? '')
    setSubmitting(true)
    setError('')
    try {
      await login(username, password)
      navigate('/')
    } catch (err) {
      const message = err instanceof Error ? err.message : ''
      setError(message === 'invalid credentials' ? '用户名或密码错误' : message || '登录失败')
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
