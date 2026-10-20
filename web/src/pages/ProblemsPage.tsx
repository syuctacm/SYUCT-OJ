import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { getProblems, type ProblemPage } from '../api.ts'

export function ProblemsPage() {
  const [query, setQuery] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')
  const [page, setPage] = useState<ProblemPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    setError('')
    getProblems(submittedQuery)
      .then(setPage)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
      .finally(() => setLoading(false))
  }, [submittedQuery])

  function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmittedQuery(query)
  }

  return (
    <section className="panel">
      <div className="panel-header problem-list-header">
        <div>
          <h1 className="panel-title">题库</h1>
          <p className="panel-subtitle">SYUCT ACM 训练题库</p>
        </div>
        <form className="search" onSubmit={handleSearch}>
          <input className="input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索题号或标题" />
          <button className="button" type="submit">搜索</button>
        </form>
      </div>
      {loading ? <div className="empty-state">加载中...</div> : null}
      {error ? <div className="empty-state error-state">{error}</div> : null}
      {page && !loading ? (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th className="col-id">编号</th>
                <th>题目</th>
                <th className="col-difficulty">难度</th>
                <th className="col-stat">通过 / 提交</th>
                <th>标签</th>
              </tr>
            </thead>
            <tbody>
              {page.items.map((problem) => (
                <tr key={problem.id}>
                  <td className="mono">{problem.pid || problem.docId}</td>
                  <td><Link className="problem-link" to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}>{problem.title}</Link></td>
                  <td className="center">{problem.difficulty || '-'}</td>
                  <td className="center mono">{problem.nAccept} / {problem.nSubmit}</td>
                  <td>{problem.tags?.map((tag) => <span className="tag" key={tag}>{tag}</span>)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {page.items.length === 0 ? <div className="empty-state">没有找到题目</div> : null}
        </div>
      ) : null}
      <div className="panel-footer">共 {page?.total ?? 0} 道题</div>
    </section>
  )
}
