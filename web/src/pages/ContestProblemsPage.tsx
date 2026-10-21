// Pre-migration page: the upstream `contest_problemlist.html` template will
// replace this hand-written table in a follow-up issue.
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getContestProblems, type Contest, type ProblemSummary } from '../api.ts'

export function ContestProblemsPage() {
  const { tid = '' } = useParams()
  const [contest, setContest] = useState<Contest | null>(null)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    getContestProblems(tid)
      .then((data) => { setContest(data.contest); setProblems(data.problems) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛题目加载失败'))
  }, [tid])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <div className="row">
      <div className="medium-12 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">{contest.title} - 题目列表</h1>
            <div className="section__tools">
              <Link className="rounded button" to={`/contest/${encodeURIComponent(contest.docId)}`}>比赛详情</Link>
            </div>
          </div>
          <div className="section__body no-padding">
            {problems.length ? (
              <table className="data-table">
                <colgroup><col className="col--status" /><col className="col--problem" /><col className="col--ac-tried" /><col className="col--difficulty" /><col className="col--actions" /></colgroup>
                <thead><tr><th className="col--status">状态</th><th className="col--problem">题目</th><th className="col--ac-tried">通过 / 尝试</th><th className="col--difficulty">难度</th><th className="col--actions">操作</th></tr></thead>
                <tbody>
                  {problems.map((problem, index) => (
                    <tr key={problem.id}>
                      <td className="col--status record-status--border">{String.fromCharCode(65 + index)}</td>
                      <td className="col--problem col--problem-name"><Link to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}><b>{problem.pid}</b>&nbsp;&nbsp;{problem.title}</Link></td>
                      <td className="col--ac-tried">{problem.nAccept} / {problem.nSubmit}</td>
                      <td className="col--difficulty">{problem.difficulty || '(None)'}</td>
                      <td className="col--actions"><Link to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}/submit?contest=${encodeURIComponent(contest.docId)}`}>提交</Link></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : <div className="nothing-placeholder">这场比赛尚未配置题目</div>}
          </div>
        </div>
      </div>
    </div>
  )
}
