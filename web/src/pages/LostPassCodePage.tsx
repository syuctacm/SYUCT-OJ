// Page markup comes from the pinned Hydro ui-default reset-with-code template.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getLostPass, resetLostPass } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function LostPassCodePage() {
  const { code = '' } = useParams()
  const navigate = useNavigate()
  const { refresh } = useAuth()
  const [uname, setUname] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    setError('')
    getLostPass(code)
      .then((data) => setUname(data.username))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '重置链接无效或已过期'))
  }, [code])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    return () => form?.removeEventListener('submit', onSubmit)
  }, [])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    if (busy) return
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const password = String(data.get('password') ?? '')
    const verifyPassword = String(data.get('verifyPassword') ?? '')
    setBusy(true)
    setError('')
    try {
      await resetLostPass(code, password, verifyPassword)
      await refresh()
      navigate('/login')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '重置密码失败')
    } finally {
      setBusy(false)
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (!uname) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '加载中...'}</div></div></div>
  }
  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {busy ? <div className="row"><div className="columns"><blockquote className="note"><p>重置中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="user_lostpass_with_code.html"
        context={{
          page_name: 'user_lostpass_with_code',
          uname,
          handler: { user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false } },
        }}
        onReady={handleReady}
      />
    </>
  )
}
