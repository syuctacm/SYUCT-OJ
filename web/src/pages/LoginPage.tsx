import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth.tsx'

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await login(username, password)
      navigate('/p')
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-card section">
      <div className="section__header">
        <h1 className="section__title">登录</h1>
      </div>
      <form className="section__body form-stack" onSubmit={handleSubmit}>
        <label className="field">
          <span>用户名或邮箱</span>
          <input className="textbox" value={username} onChange={(event) => setUsername(event.target.value)} autoFocus />
        </label>
        <label className="field">
          <span>密码</span>
          <input className="textbox" type="password" value={password} onChange={(event) => setPassword(event.target.value)} />
        </label>
        {error ? <p className="form-error">{error}</p> : null}
        <button className="rounded primary button" type="submit" disabled={submitting}>
          {submitting ? '登录中...' : '登录'}
        </button>
        <p className="form-hint"><Link to="/p">先看看题库</Link></p>
      </form>
    </div>
  )
}
