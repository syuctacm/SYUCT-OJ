// Pre-migration page: the upstream `contest_scoreboard.html` template will
// replace this hand-written table in a follow-up issue.
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getContestScoreboard, type Contest, type ScoreboardRow } from '../api.ts'

export function ContestScoreboardPage() {
  const { tid = '' } = useParams()
  const [contest, setContest] = useState<Contest | null>(null)
  const [rows, setRows] = useState<ScoreboardRow[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    getContestScoreboard(tid)
      .then((data) => { setContest(data.contest); setRows(data.rows) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '榜单加载失败'))
  }, [tid])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <div className="row">
      <div className="medium-12 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">{contest.title} - 榜单</h1>
            <div className="section__tools">
              <Link className="rounded button" to={`/contest/${encodeURIComponent(contest.docId)}`}>比赛详情</Link>
            </div>
          </div>
          <div className="section__body no-padding">
            <table className="data-table">
              <thead><tr><th className="col--rank">排名</th><th className="col--user">用户</th><th className="col--solved">过题</th><th className="col--time">罚时</th>{contest.pids?.map((pid) => <th className="col--problem" key={pid}>#{pid}</th>)}</tr></thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.uid}>
                    <td className="col--rank">{row.rank}</td>
                    <td className="col--user"><Link to={`/user/${row.uid}`}>UID {row.uid}</Link></td>
                    <td className="col--solved">{row.solved}</td>
                    <td className="col--time">{row.penalty}</td>
                    {contest.pids?.map((pid) => {
                      const cell = row.problems[String(pid)]
                      return <td key={pid} className={`col--problem${cell?.solved ? ' record-status--text pass' : ''}`}>{cell?.solved ? `${cell.attempts}/${cell.time}` : cell?.attempts ? `-${cell.attempts}` : '-'}</td>
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
            {rows.length === 0 ? <div className="nothing-placeholder">暂无榜单数据</div> : null}
          </div>
        </div>
      </div>
    </div>
  )
}
