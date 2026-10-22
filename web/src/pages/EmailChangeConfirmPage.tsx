import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { confirmEmailChange } from '../api.ts'
import { useAuth } from '../auth.tsx'

export function EmailChangeConfirmPage() {
  const { code = '' } = useParams()
  const { user, loading, refresh } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const started = useRef(false)

  useEffect(() => {
    if (!loading && !user) navigate('/login')
  }, [loading, navigate, user])

  useEffect(() => {
    if (!user || !code || started.current) return
    started.current = true
    confirmEmailChange(code)
      .then(async (result) => {
        await refresh()
        setEmail(result.email)
        setDone(true)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '邮箱确认失败'))
  }, [code, refresh, user])

  if (loading || !user) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <div className="row"><div className="columns">
      <div className="section">
        <div className="section__header"><h1 className="section__title">修改邮箱</h1></div>
        <div className="section__body typo">
          {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
          {done ? <blockquote className="note"><p>邮箱已更新为 {email}。</p></blockquote> : null}
          {!done && !error ? <p>正在确认邮箱变更...</p> : null}
          <button type="button" className="rounded primary button" onClick={() => navigate('/home/security')}>返回安全设置</button>
        </div>
      </div>
    </div></div>
  )
}
