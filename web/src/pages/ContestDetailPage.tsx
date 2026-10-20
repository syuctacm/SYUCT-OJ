import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import { getContestProblems, getContestScoreboard, type Contest, type ProblemSummary, type ScoreboardRow } from '../api.ts'
import { useAuth } from '../auth.tsx'

const ruleNames: Record<string, string> = {
  acm: 'ACM',
  oi: 'OI',
  ioi: 'IOI',
  ledo: 'Ledo',
  strictioi: 'Strict IOI',
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function durationOf(contest: Contest) {
  if (contest.duration) return contest.duration
  return Math.max(0, Math.round((new Date(contest.endAt).getTime() - new Date(contest.beginAt).getTime()) / 3600000))
}

function contestStatus(contest: Contest) {
  const now = Date.now()
  if (now < new Date(contest.beginAt).getTime()) return '即将开始'
  if (now > new Date(contest.endAt).getTime()) return '已结束'
  return '进行中'
}

export function ContestDetailPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
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

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  const status = contestStatus(contest)
  const locked = Boolean(contest.lockAt && !contest.unlocked && new Date(contest.lockAt).getTime() <= Date.now())
  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">{contest.title}</h1>
            {user && (user.id === contest.owner || ((user.priv ?? 0) & 1) !== 0) ? (
              <div className="section__tools">
                <Link className="rounded button" to={`/contest/${encodeURIComponent(contest.docId)}/files`}>文件</Link>
                <Link className="rounded button" to={`/contest/${encodeURIComponent(contest.docId)}/edit`}>编辑比赛</Link>
              </div>
            ) : null}
          </div>
          <div className="section__body">
            <span className="problem__tag-item">{locked ? '榜单已封存' : status}</span>
            <span className="problem__tag-item icon icon-award">{ruleNames[contest.rule] || contest.rule}</span>
            <span className="problem__tag-item">开始: {formatDate(contest.beginAt)}</span>
            <span className="problem__tag-item icon icon-schedule">{durationOf(contest)} 小时</span>
            <span className="problem__tag-item">主办: <Link to={`/user/${contest.owner}`}>UID {contest.owner}</Link></span>
            <span className="problem__tag-item icon icon-user--multiple">{contest.attend}</span>
          </div>
        </div>
        <div className="section">
          <div className="section__body typo richmedia">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{contest.content || '暂无比赛说明'}</ReactMarkdown>
            {status === '进行中' ? <blockquote className="warn"><p>比赛结束后将进行代码相似度检查。</p></blockquote> : null}
          </div>
        </div>

        <div className="section" id="contest-problems">
          <div className="section__header"><h1 className="section__title">比赛题目</h1></div>
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

        <div className="section" id="contest-scoreboard">
          <div className="section__header"><h1 className="section__title">榜单</h1></div>
          <div className="section__body no-padding">
            <table className="data-table">
              <thead><tr><th className="col--rank">排名</th><th className="col--user">用户</th><th className="col--solved">过题</th><th className="col--time">罚时</th>{contest.pids?.map((pid) => <th className="col--problem" key={pid}>#{pid}</th>)}</tr></thead>
              <tbody>
                {scoreboard.map((row) => (
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
            {scoreboard.length === 0 ? <div className="nothing-placeholder">暂无榜单数据</div> : null}
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div>
            <ol className="menu">
              <li className="menu__item"><a className="menu__link" href="#contest-problems"><span className="icon icon-unordered_list" /> 题目列表</a></li>
              <li className="menu__item"><a className="menu__link" href="#contest-scoreboard"><span className="icon icon-statistics" /> 榜单</a></li>
              <li className="menu__item"><Link className="menu__link" to={`/record?tid=${encodeURIComponent(contest.docId)}`}><span className="icon icon-flag" /> 全部提交</Link></li>
              <li className="menu__item"><Link className="menu__link" to="/discuss"><span className="icon icon-comment--text" /> 讨论</Link></li>
            </ol>
          </div>
          <div className="section__body typo">
            <dl className="large horizontal">
              <dt>状态</dt><dd>{status}</dd>
              <dt>赛制</dt><dd>{ruleNames[contest.rule] || contest.rule}</dd>
              <dt>题目</dt><dd>{contest.pids?.length ?? 0}</dd>
              <dt>开始</dt><dd>{formatDate(contest.beginAt)}</dd>
              <dt>结束</dt><dd>{formatDate(contest.endAt)}</dd>
              <dt>时长</dt><dd>{durationOf(contest)} 小时</dd>
              <dt>主办</dt><dd><Link to={`/user/${contest.owner}`}>UID {contest.owner}</Link></dd>
              <dt>参赛</dt><dd>{contest.attend}</dd>
            </dl>
          </div>
        </div>
      </div>
    </div>
  )
}
