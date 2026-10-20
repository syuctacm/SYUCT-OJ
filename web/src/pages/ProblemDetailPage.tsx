import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import { getProblem, type ProblemDetail } from '../api.ts'
import { ProblemSidebar } from '../components/ProblemSidebar.tsx'
import { formatMemoryLimit, formatTimeLimit } from '../problem-format.ts'

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

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!problem) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">#{problem.pid || problem.docId}. {problem.title}</h1>
          </div>
          <div className="section__body">
            <ul className="problem__tags">
              <span className="problem__tag-item">ID: {problem.docId}</span>
              {problem.problemType ? <span className="problem__tag-item icon icon-book">{problem.problemType}</span> : null}
              {problem.timeLimit ? <span className="problem__tag-item icon icon-stopwatch">{formatTimeLimit(problem.timeLimit)}</span> : null}
              {problem.memoryLimit ? <span className="problem__tag-item icon icon-comparison">{formatMemoryLimit(problem.memoryLimit)}</span> : null}
              <span className="problem__tag-item interactive">尝试: {problem.nSubmit}</span>
              <span className="problem__tag-item interactive">通过: {problem.nAccept}</span>
              <span className="problem__tag-item">难度: {problem.difficulty || '(None)'}</span>
              {problem.owner ? <span className="problem__tag-item">上传者: <Link to={`/user/${problem.owner}`}>UID {problem.owner}</Link></span> : null}
              {problem.tags?.map((tag) => <span className="problem__tag-item tags" key={tag}>{tag}</span>)}
            </ul>
          </div>
        </div>
        <div className="section">
          <div className="problem-content-container">
            <div className="problem-content">
              <div className="section__header non-scratchpad--hide"><h1 className="section__title">{problem.title}</h1></div>
              <div className="section__body typo richmedia">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{problem.content || '暂无题目内容'}</ReactMarkdown>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <ProblemSidebar problem={problem} pageName="problem_detail" />
      </div>
    </div>
  )
}
