import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Link, useParams } from 'react-router-dom'
import {
  enrollTraining,
  getTraining,
  type TrainingDetail,
  type TrainingSection,
} from '../api.ts'
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
    getTraining(tid)
      .then(setTraining)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
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

  if (error) return <section className="section"><div className="section__body error-state">{error}</div></section>
  if (!training) return <section className="section"><div className="section__body">加载中...</div></section>

  const sectionByID = new Map(training.sections.map((section) => [section.id, section]))
  const overallProgress = progressPercent(training.doneProblems, training.totalProblems)

  return (
    <section className="section">
      <div className="section__header clearfix">
        <div className="float-left">
          <h1 className="section__title">{training.title}</h1>
          <p className="supplementary">
            {training.sections.length} 个章节 · {training.totalProblems} 道题 · {training.attend} 人报名
          </p>
        </div>
        <div className="float-right training-header-actions">
          {training.pin ? <span className="tag">置顶</span> : null}
          {authLoading ? null : user ? (
            training.enrolled ? <span className="training-enrolled-badge">已报名</span> : (
              <button className="rounded primary button compact" type="button" disabled={enrolling} onClick={handleEnroll}>
                {enrolling ? '报名中...' : '报名训练'}
              </button>
            )
          ) : <Link className="rounded button compact" to="/login">登录后报名</Link>}
        </div>
      </div>

      <div className="section__body">
        <div className="training-overview">
          <span>完成进度</span>
          <strong>{training.doneProblems} / {training.totalProblems}</strong>
          <div className="training-progress-track"><div style={{ width: `${overallProgress}%` }} /></div>
          {training.completed ? <span className="training-enrolled-badge">训练已完成</span> : null}
        </div>
        {enrollError ? <p className="form-error">{enrollError}</p> : null}
        {!user && !authLoading ? (
          <blockquote className="note">登录并报名后即可从训练计划进入题目。</blockquote>
        ) : !training.enrolled ? (
          <blockquote className="note">报名后开始训练，系统会根据提交记录自动更新章节进度。</blockquote>
        ) : null}
      </div>

      <div className="section__body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{training.content || '暂无训练说明'}</ReactMarkdown>
      </div>

      <div className="training__list">
        {training.sections.map((section) => {
          const locked = !training.enrolled || section.status === 'invalid'
          return (
            <article className={`training-section training__section ${section.status}`} key={section.id}>
              <div className="section__header clearfix">
                <div className="float-left">
                  <h2 className="section__title">Section {section.id}. {section.title}</h2>
                </div>
                <div className="float-right">
                  <span className="training-section-progress">{section.progress}%</span>
                  <h2 className={`training-section-status--text ${section.status}`} style={{ display: 'inline' }}>
                    <span className={`training-section-status--icon ${section.status}`} />
                    {statusText[section.status]}
                  </h2>
                </div>
              </div>
              {section.status === 'invalid' ? (
                <div className="section__body training-requirement">
                  完成以下前置章节后解锁：
                  {' '}
                  {section.requireNids.map((nid) => `第 ${nid} 节 ${sectionByID.get(nid)?.title ?? ''}`).join('、')}
                </div>
              ) : null}
              {section.content ? (
                <div className="section__body problem-content">
                  <ReactMarkdown remarkPlugins={[remarkGfm]}>{section.content}</ReactMarkdown>
                </div>
              ) : null}
              <div className="section__table-container training__problems">
                <table className="data-table">
                  <thead><tr><th>题目</th><th className="col-stat">通过 / 提交</th><th className="col-stat">状态</th></tr></thead>
                  <tbody>
                    {section.problems.map((problem) => (
                      <tr key={problem.docId}>
                        <td>
                          {problem.missing || locked ? (
                            <span className={problem.missing ? 'training-problem-disabled' : ''}>{problem.pid || problem.docId} {problem.title}</span>
                          ) : (
                            <Link className="problem-link" to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}>
                              {problem.pid} {problem.title}
                            </Link>
                          )}
                        </td>
                        <td className="mono col-stat">{problem.missing ? '-' : `${problem.nAccept} / ${problem.nSubmit}`}</td>
                        <td className="col-stat">
                          {problem.solved ? <span className="record-status-1">已通过</span>
                            : problem.tried ? <span className="record-status-20">尝试过</span> : '-'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </article>
          )
        })}
      </div>
    </section>
  )
}
