import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import { getProblem, type ProblemDetail } from '../api.ts'

export function ProblemDetailPage() {
  const { pid = '' } = useParams()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    setProblem(null)
    setError('')
    getProblem(pid)
      .then(setProblem)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
  }, [pid])

  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>
  if (!problem) return <section className="panel"><div className="empty-state">加载中...</div></section>

  return (
    <article className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">PROBLEM {problem.pid || problem.docId}</p>
          <h1 className="panel-title">{problem.title}</h1>
        </div>
        <Link className="button button-primary" to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}/submit`}>提交代码</Link>
      </div>
      <div className="problem-meta">
        <span>通过 {problem.nAccept}</span>
        <span>提交 {problem.nSubmit}</span>
        <span>难度 {problem.difficulty || '-'}</span>
        {problem.tags?.map((tag) => <span className="tag" key={tag}>{tag}</span>)}
      </div>
      <div className="panel-body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{problem.content || '暂无题目内容'}</ReactMarkdown>
      </div>
    </article>
  )
}
