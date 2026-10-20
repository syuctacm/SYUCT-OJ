import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth.tsx'

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    const root = document.documentElement
    root.classList.remove('layout--basic')
    root.classList.add('layout--immersive')
    return () => {
      root.classList.remove('layout--immersive')
      root.classList.add('layout--basic')
    }
  }, [])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await login(username, password)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="row"><div className="columns">
      <div className="immersive--content immersive--center">
        <h1>登录</h1>
        <form onSubmit={handleSubmit}>
          <div className="row"><div className="columns">
            <label className="inverse material textbox">
              用户名
              <input name="uname" type="text" value={username} onChange={(event) => setUsername(event.target.value)} autoFocus autoComplete="username" />
            </label>
          </div></div>
          <div className="row"><div className="columns">
            <label className="inverse material textbox">
              密码
              <input name="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" />
            </label>
          </div></div>
          <div className="row"><div className="columns">
            <label className="inverse material checkbox" style={{ color: '#fff' }}>
              <input type="checkbox" name="rememberme" className="checkbox" /> 记住我
            </label>
          </div></div>
          {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
          <div className="row"><div className="columns">
            <div className="text-center">
              <input type="submit" value={submitting ? '登录中...' : '登录'} name="login_submit" className="inverse expanded rounded primary button" disabled={submitting} />
            </div>
          </div></div>
        </form>
      </div>
    </div></div>
  )
}
