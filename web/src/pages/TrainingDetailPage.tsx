import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import { enrollTraining, getTraining, type TrainingDetail, type TrainingSection } from '../api.ts'
import { useAuth } from '../auth.tsx'

const statusText: Record<TrainingSection['status'], string> = {
  done: '已完成',
  progress: '进行中',
  open: '可开始',
  invalid: '未解锁',
}

function progressPercent(done: number, total: number) {
  return total > 0 ? Math.round(done * 100 / total) : 0
}

export function TrainingDetailPage() {
  const { tid = '' } = useParams()
  const { user, loading: authLoading } = useAuth()
  const [training, setTraining] = useState<TrainingDetail | null>(null)
  const [error, setError] = useState('')
  const [enrollError, setEnrollError] = useState('')
  const [enrolling, setEnrolling] = useState(false)

  useEffect(() => {
    getTraining(tid).then(setTraining).catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
  }, [tid])

  async function handleEnroll() {
    setEnrolling(true)
    setEnrollError('')
    try {
      await enrollTraining(tid)
      setTraining(await getTraining(tid))
    } catch (err) {
      setEnrollError(err instanceof Error ? err.message : '报名失败')
    } finally {
      setEnrolling(false)
    }
  }

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!training) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  const sectionByID = new Map(training.sections.map((section) => [section.id, section]))
  const progress = progressPercent(training.doneProblems, training.totalProblems)

  return (
    <div className="row">
      <div className="large-9 columns">
        <div className="section">
          <div className="section__header"><h1 className="section__title">{training.title}</h1></div>
          <div className="section__body typo">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{training.content || ''}</ReactMarkdown>
          </div>
          <div className="section__body typo richmedia">
            {!authLoading && !user ? <blockquote className="typo note"><p>登录后参加训练计划。</p></blockquote> : !training.enrolled ? <blockquote className="typo note"><p>报名后开始训练，系统会根据提交记录更新进度。</p></blockquote> : null}
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{training.description || ''}</ReactMarkdown>
          </div>
        </div>
        <div className="section">
          <div data-fragment-id="training_detail">
            {training.sections.map((section) => {
              const locked = !training.enrolled || section.status === 'invalid'
              return (
                <div className={`training__section expanded ${section.status}`} key={section.id}>
                  <div className="section__header clearfix">
                    <div className="float-left">
                      <h1 className="section__title">第 {section.id} 节. {section.title.split('\n')[0]}</h1>
                      {section.title.split('\n')[1] ? <h4>{section.title.split('\n')[1]}</h4> : null}
                    </div>
                    <div className="float-right">
                      <h1 className={`section__title training-section-status--text ${section.status}`}>
                        <span className={`icon training-section-status--icon ${section.status}`} />
                        {statusText[section.status]}
                      </h1>
                    </div>
                  </div>
                  <div className="section__body">
                    <ul className="supplementary dot list"><li>{section.progress}%</li></ul>
                  </div>
                  <div className="training__section__detail">
                    {section.status === 'invalid' ? (
                      <div className="section__body">
                        <blockquote className="typo note">
                          <p>请先完成以下前置章节：</p>
                          <ul>
                            {section.requireNids.map((nid) => <li key={nid}>第 {nid} 节. {sectionByID.get(nid)?.title ?? ''}</li>)}
                          </ul>
                        </blockquote>
                      </div>
                    ) : null}
                    {section.content ? <div className="section__body typo richmedia"><ReactMarkdown remarkPlugins={[remarkGfm]}>{section.content}</ReactMarkdown></div> : null}
                    <div className="section__body no-padding training__problems">
                      <table className="data-table">
                        <colgroup>
                          <col className="col--status" /><col className="col--name" /><col className="col--tried" /><col className="col--ac" /><col className="col--difficulty" />
                        </colgroup>
                        <thead><tr><th className="col--status record-status--border">状态</th><th className="col--name">题目</th><th className="col--tried">尝试</th><th className="col--ac">通过</th><th className="col--difficulty">难度</th></tr></thead>
                        <tbody>
                          {section.problems.map((problem) => (
                            <tr data-pid={problem.docId} key={problem.docId}>
                              <td className="col--status record-status--border">{problem.solved ? '已通过' : problem.tried ? '尝试过' : ''}</td>
                              <td className="col--name col--problem-name">
                                {problem.missing || locked ? <span>{problem.pid || problem.docId} {problem.title}</span> : <Link to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}><b>{problem.pid}</b>&nbsp;&nbsp;{problem.title}</Link>}
                              </td>
                              <td className="col--tried">{problem.nSubmit}</td>
                              <td className="col--ac">{problem.nAccept}</td>
                              <td className="col--difficulty">{problem.difficulty || '(None)'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                </div>
              )
            })}
            <div className="section__body">&nbsp;</div>
          </div>
        </div>
      </div>
      <div className="large-3 columns">
        <div className="section side">
          <div>
            <ol className="menu">
              {!training.enrolled && user && !training.completed ? (
                <li className="menu__item">
                  <form onSubmit={(event) => { event.preventDefault(); handleEnroll() }}>
                    <button className="menu__link" type="submit" disabled={enrolling}>
                      <span className="icon icon-add" /> {enrolling ? '报名中...' : '报名训练'}
                    </button>
                  </form>
                </li>
              ) : null}
              <li className="menu__item"><a className="menu__link" href="#training_detail"><span className="icon icon-unordered_list" /> 训练章节</a></li>
              <li className="menu__item"><Link className="menu__link" to="/discuss"><span className="icon icon-comment--text" /> 讨论</Link></li>
              <li className="menu__separator" />
            </ol>
          </div>
          <div className="section__body typo">
            {enrollError ? <p className="form-error">{enrollError}</p> : null}
            <dl className="large horizontal">
              <dt>状态</dt><dd>{training.completed ? '已完成' : training.enrolled ? '进行中' : '未报名'}</dd>
              <dt>章节</dt><dd>{training.sections.length}</dd>
              <dt>题目</dt><dd>{training.totalProblems}</dd>
              <dt>完成</dt><dd>{progress}%</dd>
              <dt>报名</dt><dd>{training.attend}</dd>
            </dl>
          </div>
        </div>
      </div>
    </div>
  )
}
