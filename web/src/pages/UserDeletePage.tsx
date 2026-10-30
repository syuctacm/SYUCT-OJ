import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { cancelAccountDeletion, getAccountDeletion, requestAccountDeletion } from '../api.ts'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function UserDeletePage() {
  const navigate = useNavigate()
  const [deleteAt, setDeleteAt] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  const load = useCallback(() => {
    setLoading(true)
    setError('')
    getAccountDeletion()
      .then((data) => setDeleteAt(data.deleteAt))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '账号状态加载失败'))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { load() }, [load])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    return () => form?.removeEventListener('submit', onSubmit)
  }, [])

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    if (busy) return
    const password = String(new FormData(event.target as HTMLFormElement).get('password') ?? '')
    setBusy(true)
    setError('')
    try {
      const result = await requestAccountDeletion(password)
      setDeleteAt(result.deleteAt)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '注销请求失败')
    } finally {
      setBusy(false)
    }
  }
  submitRef.current = (event) => void submit(event)

  async function cancel() {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await cancelAccountDeletion()
      setDeleteAt(null)
      navigate('/home/settings?category=account')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '取消失败')
    } finally {
      setBusy(false)
    }
  }

  const handlePendingReady = useCallback((root: HTMLDivElement) => {
    const content = root.querySelector('.immersive--content')
    if (!content) return
    const button = document.createElement('button')
    button.className = 'inverse rounded primary button'
    button.textContent = '取消注销'
    button.addEventListener('click', () => void cancel())
    content.appendChild(button)
    return () => button.remove()
  }, [busy])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {busy ? <div className="row"><div className="columns"><blockquote className="note"><p>处理中...</p></blockquote></div></div> : null}
      {deleteAt ? (
        <HydroTemplate template="user_delete_pending.html" context={{ page_name: 'user_delete' }} onReady={handlePendingReady} />
      ) : (
        <HydroTemplate template="user_delete_confirm.html" context={{ page_name: 'user_delete' }} onReady={handleReady} />
      )}
    </>
  )
}
