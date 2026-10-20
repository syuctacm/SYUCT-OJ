import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import { getHomeworkProblems, type Contest, type ProblemSummary } from '../api.ts'

function formatDate(value?: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function homeworkStatus(homework: Contest) {
  const now = Date.now()
  if (now < new Date(homework.beginAt).getTime()) return '未开始'
  if (now > new Date(homework.endAt).getTime()) return '已结束'
  return '进行中'
}

export function HomeworkDetailPage() {
  const { tid = '' } = useParams()
  const [homework, setHomework] = useState<Contest | null>(null)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    getHomeworkProblems(tid)
      .then((data) => { setHomework(data.homework); setProblems(data.problems) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
  }, [tid])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!homework) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header"><h1 className="section__title">作业说明</h1></div>
          <div className="section__body typo richmedia">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{homework.content || '暂无作业说明'}</ReactMarkdown>
          </div>
        </div>
        <div className="section" id="homework-problems">
          <div className="section__header"><h1 className="section__title">题目</h1></div>
          {problems.length ? (
            <div className="section__body no-padding">
              <table className="data-table">
                <colgroup><col className="col--problem" /></colgroup>
                <thead><tr><th className="col--problem">题目</th></tr></thead>
                <tbody>
                  {problems.map((problem) => (
                    <tr key={problem.id}>
                      <td className="col--problem col--problem-name">
                        <Link to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}><b>{problem.pid}</b>&nbsp;&nbsp;{problem.title}</Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : <div className="nothing-placeholder">该作业尚未配置题目</div>}
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__header"><h1 className="section__title">{homework.title}</h1></div>
          <div>
            <ol className="menu">
              <li className="menu__item"><a className="menu__link" href="#homework-problems"><span className="icon icon-homework" /> 题目列表</a></li>
              <li className="menu__item"><Link className="menu__link" to={`/record?tid=${encodeURIComponent(homework.docId)}`}><span className="icon icon-flag" /> 提交记录</Link></li>
              <li className="menu__item"><Link className="menu__link" to="/discuss"><span className="icon icon-comment--text" /> 讨论</Link></li>
              <li className="menu__separator" />
            </ol>
          </div>
          <div className="section__body typo">
            <dl className="large horizontal">
              <dt>状态</dt><dd>{homeworkStatus(homework)}</dd>
              <dt>题目</dt><dd>{problems.length}</dd>
              <dt>开始时间</dt><dd>{formatDate(homework.beginAt)}</dd>
              <dt>截止时间</dt><dd>{formatDate(homework.penaltySince || homework.endAt)}</dd>
            </dl>
          </div>
        </div>
      </div>
    </div>
  )
}
