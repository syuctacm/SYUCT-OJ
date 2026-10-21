// Page markup is rendered from the pinned Hydro ui-default template
// (problem_import_fps.html); this file only wires the upload form.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { importFPSProblems } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function ProblemImportFpsPage() {
  const { user, loading } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    if (!loading && !user) navigate('/login')
  }, [loading, navigate, user])

  async function handleSubmit(event: SubmitEvent) {
    const form = event.target as HTMLFormElement
    if (!(form instanceof HTMLFormElement)) return
    event.preventDefault()
    if (busy) return
    const input = form.querySelector<HTMLInputElement>('input[type="file"]')
    const file = input?.files?.[0]
    if (!file) {
      setError('请选择 FPS 文件')
      return
    }
    setBusy(true)
    setError('')
    try {
      await importFPSProblems(file)
      navigate('/p')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '导入失败')
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

  if (loading || !user) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_import_fps.html"
        context={{ page_name: 'problem_import_fps' }}
        onReady={handleReady}
      />
    </>
  )
}
