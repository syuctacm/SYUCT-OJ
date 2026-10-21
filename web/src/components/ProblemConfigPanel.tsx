// The judge config editor is mounted into the upstream `#ProblemConfig`
// container. The upstream implementation is the Monaco based
// `components/problemconfig` bundle (Monaco + Redux + Mantine form), which this
// rewrite has not ported yet; this panel keeps the same mount point and stores
// the YAML through the existing problem API.
import { useEffect, useState } from 'react'
import { getProblemEditor, updateProblem, type ProblemDetail, type ProblemEditInput } from '../api.ts'

export function ProblemConfigPanel({ pid }: { pid: string }) {
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [config, setConfig] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    setLoading(true)
    getProblemEditor(pid)
      .then((data) => {
        setProblem(data.problem)
        setConfig(data.config)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '判题配置加载失败'))
      .finally(() => setLoading(false))
  }, [pid])

  async function save() {
    if (!problem) return
    setSaving(true)
    setError('')
    setSaved(false)
    const input: ProblemEditInput = {
      pid: problem.pid,
      title: problem.title,
      content: problem.content || '',
      tags: problem.tags ?? [],
      difficulty: problem.difficulty ?? 0,
      hidden: Boolean(problem.hidden),
      config,
    }
    try {
      await updateProblem(pid, input)
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : '判题配置保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="loader-container"><div className="loader" /></div>
  }
  return (
    <div>
      <textarea
        className="textbox auto-resize"
        value={config}
        onChange={(event) => {
          setConfig(event.target.value)
          setSaved(false)
        }}
        spellCheck={false}
        style={{ minHeight: 520, width: '100%', fontFamily: 'var(--code-font-family)' }}
      />
      {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
      {saved ? <blockquote className="note"><p>配置已保存。</p></blockquote> : null}
      <div className="row"><div className="columns">
        <button type="button" className="rounded primary button" disabled={saving} onClick={() => void save()}>
          {saving ? '保存中...' : '保存'}
        </button>
      </div></div>
    </div>
  )
}
