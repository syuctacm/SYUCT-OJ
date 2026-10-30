// Page markup comes from the pinned Hydro ui-default lost-password template.
import { useCallback, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { requestLostPass } from '../api.ts'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function LostPassPage() {
  const navigate = useNavigate()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [sent, setSent] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  const context = useMemo(() => ({
    page_name: 'user_lostpass',
    model: { system: { get: (key: string) => (key === 'smtp.user' ? 'configured' : '') } },
    handler: { user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false } },
  }), [])

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
    const mail = String(new FormData(form).get('mail') ?? '').trim()
    setBusy(true)
    setError('')
    try {
      const result = await requestLostPass(mail)
      if (result.code) {
        navigate(`/lostpass/${encodeURIComponent(result.code)}`)
      } else {
        setSent(true)
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '发送重置邮件失败')
    } finally {
      setBusy(false)
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (sent) {
    return <HydroTemplate template="user_lostpass_mail_sent.html" context={context} />
  }
  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {busy ? <div className="row"><div className="columns"><blockquote className="note"><p>发送中...</p></blockquote></div></div> : null}
      <HydroTemplate template="user_lostpass.html" context={context} onReady={handleReady} />
    </>
  )
}
