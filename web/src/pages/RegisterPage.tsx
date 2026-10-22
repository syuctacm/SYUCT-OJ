// Page markup is rendered from the pinned ui-default template
// (user_register.html); this file only wires the email form.
import { useCallback, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { requestRegistration } from '../api.ts'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function RegisterPage() {
  const navigate = useNavigate()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  async function handleSubmit(event: SubmitEvent) {
    const form = event.target as HTMLFormElement
    if (!(form instanceof HTMLFormElement)) return
    event.preventDefault()
    if (busy) return
    const mail = String(new FormData(form).get('mail') ?? '').trim()
    if (!mail) {
      setError('请输入邮箱地址')
      return
    }
    setBusy(true)
    setError('')
    try {
      const result = await requestRegistration(mail)
      navigate(`/register/${encodeURIComponent(result.code)}`)
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

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="user_register.html"
        context={{
          page_name: 'user_register',
          captcha: '',
          handler: { user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false } },
        }}
        onReady={handleReady}
      />
    </>
  )
}
