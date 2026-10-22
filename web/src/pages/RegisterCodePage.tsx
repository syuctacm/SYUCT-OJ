// Page markup is rendered from the pinned ui-default template
// (user_register_with_code.html); this file loads the token and wires the form.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { completeRegistration, getRegistration } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function RegisterCodePage() {
  const { code = '' } = useParams()
  const navigate = useNavigate()
  const { refresh } = useAuth()
  const [mail, setMail] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    setError('')
    getRegistration(code)
      .then((data) => setMail(data.mail))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '注册链接无效或已过期'))
  }, [code])

  async function handleSubmit(event: SubmitEvent) {
    const form = event.target as HTMLFormElement
    if (!(form instanceof HTMLFormElement)) return
    event.preventDefault()
    if (busy) return
    const data = new FormData(form)
    setBusy(true)
    setError('')
    try {
      await completeRegistration(
        code,
        String(data.get('uname') ?? '').trim(),
        String(data.get('password') ?? ''),
        String(data.get('verifyPassword') ?? ''),
      )
      await refresh()
      navigate('/home/settings?category=preference')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '注册失败')
    } finally {
      setBusy(false)
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    return () => form?.removeEventListener('submit', onSubmit)
  }, [])

  if (!mail) {
    return (
      <div className="row"><div className="columns">
        <div className="nothing-placeholder">{error || '加载中...'}</div>
      </div></div>
    )
  }
  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="user_register_with_code.html"
        context={{
          page_name: 'user_register_with_code',
          mail,
          username: '',
          lockUsername: false,
          handler: { user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false } },
        }}
        onReady={handleReady}
      />
    </>
  )
}
