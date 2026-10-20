import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import { getContestProblems, getContestScoreboard, type Contest, type ProblemSummary, type ScoreboardRow } from '../api.ts'

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false })
}

export function ContestDetailPage() {
  const { tid = '' } = useParams()
  const [contest, setContest] = useState<Contest | null>(null)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [scoreboard, setScoreboard] = useState<ScoreboardRow[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([getContestProblems(tid), getContestScoreboard(tid)])
      .then(([data, board]) => {
        setContest(data.contest)
        setProblems(data.problems)
        setScoreboard(board.rows)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
  }, [tid])

  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>
  if (!contest) return <section className="panel"><div className="empty-state">加载中...</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">CONTEST</p>
          <h1 className="panel-title">{contest.title}</h1>
        </div>
        <Link className="button" to="/contest">返回比赛</Link>
      </div>
      <div className="contest-meta">
        <span>赛制 {contest.rule || '-'}</span>
        <span>开始 {formatTime(contest.beginAt)}</span>
        <span>结束 {formatTime(contest.endAt)}</span>
        <span>参赛 {contest.attend}</span>
        <span>题目 {contest.pids?.length ?? 0}</span>
        {contest.lockAt && !contest.unlocked && new Date(contest.lockAt).getTime() <= Date.now() ? <span className="tag">榜单已封存</span> : null}
      </div>
      <div className="panel-body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{contest.content || '暂无比赛说明'}</ReactMarkdown>
      </div>
      <div className="panel-header"><h2 className="panel-title">比赛榜单</h2></div>
      <div className="table-wrap">
        <table className="data-table">
          <thead><tr><th className="col-id">排名</th><th>UID</th><th>过题</th><th>罚时</th>{contest.pids?.map((pid) => <th key={pid}>#{pid}</th>)}</tr></thead>
          <tbody>
            {scoreboard.map((row) => (
              <tr key={row.uid}>
                <td className="mono">{row.rank}</td>
                <td className="mono">{row.uid}</td>
                <td className="mono">{row.solved}</td>
                <td className="mono">{row.penalty}</td>
                {contest.pids?.map((pid) => {
                  const cell = row.problems[String(pid)]
                  return <td key={pid} className={`scoreboard-cell${cell?.solved ? ' solved' : ''}`}>{cell?.solved ? `${cell.attempts}/${cell.time}` : cell?.attempts ? `-${cell.attempts}` : '-'}</td>
                })}
              </tr>
            ))}
          </tbody>
        </table>
        {scoreboard.length === 0 ? <div className="empty-state">暂无榜单数据</div> : null}
      </div>
      <div className="panel-header"><h2 className="panel-title">比赛题目</h2></div>
      <div className="table-wrap">
        <table className="data-table">
          <thead><tr><th className="col-id">编号</th><th>题目</th><th className="col-difficulty">难度</th><th className="col-stat">通过 / 提交</th><th>操作</th></tr></thead>
          <tbody>
            {problems.map((problem) => (
              <tr key={problem.id}>
                <td className="mono">{problem.pid || problem.docId}</td>
                <td><Link className="problem-link" to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}>{problem.title}</Link></td>
                <td className="center">{problem.difficulty || '-'}</td>
                <td className="center mono">{problem.nAccept} / {problem.nSubmit}</td>
                <td><Link className="button" to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}/submit?contest=${encodeURIComponent(contest.docId)}`}>提交</Link></td>
              </tr>
            ))}
          </tbody>
        </table>
        {problems.length === 0 ? <div className="empty-state">这场比赛的题目尚未配置</div> : null}
      </div>
    </section>
  )
}
