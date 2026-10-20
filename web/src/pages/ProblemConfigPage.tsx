import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  getProblemEditor,
  updateProblem,
  type ProblemDetail,
  type ProblemEditInput,
} from '../api.ts'
import { ProblemSidebar } from '../components/ProblemSidebar.tsx'

export function ProblemConfigPage() {
  const { pid = '' } = useParams()
  const navigate = useNavigate()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [config, setConfig] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

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

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!problem) return
    setSaving(true)
    setError('')
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
      navigate(`/p/${encodeURIComponent(problem.pid)}/files`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '判题配置保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (!problem) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '题目不存在'}</div></div></div>
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">判题配置</h1>
            <div className="section__tools">
              <Link className="rounded button" to={`/p/${encodeURIComponent(problem.pid)}/files`}>测试数据</Link>
            </div>
          </div>
          <div className="section__body">
            <form onSubmit={handleSubmit}>
              <p className="help-text">YAML 格式。修改后会用于下一次提交。</p>
              <textarea
                className="textbox auto-resize"
                value={config}
                onChange={(event) => setConfig(event.target.value)}
                spellCheck={false}
                style={{ minHeight: 520, fontFamily: 'var(--code-font-family)' }}
              />
              {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
              <div className="row"><div className="columns">
                <button type="submit" className="rounded primary button" disabled={saving}>{saving ? '保存中...' : '保存配置'}</button>
                <Link className="rounded button" to={`/p/${encodeURIComponent(problem.pid)}`}>取消</Link>
              </div></div>
            </form>
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <ProblemSidebar problem={problem} pageName="problem_config" />
      </div>
    </div>
  )
}
