import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { updateUserSettings } from '../api.ts'
import { useAuth } from '../auth.tsx'

export function SetThemePage() {
  const { theme = 'light' } = useParams()
  const { user, loading, refresh } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [error, setError] = useState('')
  const started = useRef('')

  useEffect(() => {
    if (loading) return
    if (!user) {
      navigate('/login', { replace: true })
      return
    }
    if (theme !== 'light' && theme !== 'dark') {
      navigate('/', { replace: true })
      return
    }
    const key = `${theme}:${location.search}`
    if (started.current === key) return
    started.current = key
    const redirect = new URLSearchParams(location.search).get('redirect')
    const returnTo = redirect?.startsWith('/') && !redirect.startsWith('//') ? redirect : '/'
    updateUserSettings({ theme })
      .then(async () => {
        await refresh()
        if (started.current === key) navigate(returnTo, { replace: true })
      })
      .catch((reason: unknown) => {
        if (started.current === key) setError(reason instanceof Error ? reason.message : '主题切换失败')
      })
  }, [loading, location.search, navigate, refresh, theme, user])

  return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '正在切换主题...'}</div></div></div>
}
