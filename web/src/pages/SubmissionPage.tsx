import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getSubmission, type Submission } from '../api.ts'
import { statusClass, statusLabel } from '../status.ts'

const languageNames: Record<string, string> = {
  cpp: 'C++17',
  python3: 'Python 3',
}

function judgeText(value: unknown) {
  if (typeof value === 'string') return value
  if (value && typeof value === 'object' && 'message' in value) return String((value as { message: unknown }).message ?? '')
  return JSON.stringify(value)
}

function formatMemory(kib: number) {
  return kib >= 1024 ? `${(kib / 1024).toFixed(1)}MiB` : `${kib}KiB`
}

export function SubmissionPage() {
  const { rid = '' } = useParams()
  const [submission, setSubmission] = useState<Submission | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    let timer: number | undefined
    const load = async () => {
      try {
        const next = await getSubmission(rid)
        if (cancelled) return
        setSubmission(next)
        setError('')
        if ([0, 20, 21, 22].includes(next.status)) timer = window.setTimeout(load, 1000)
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : '提交加载失败')
      }
    }
    load()
    return () => {
      cancelled = true
      if (timer) window.clearTimeout(timer)
    }
  }, [rid])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!submission) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  const className = statusClass(submission.status)
  const cases = submission.testCases ?? []
  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section visible" id="status" data-status={submission.status}>
          <div className="section__header">
            <h1 className="section__title">
              <span className={`icon record-status--icon ${className}`} />
              <span>{submission.score} </span>
              <span className={`record-status--text ${className}`}>{statusLabel(submission.status)}</span>
            </h1>
          </div>
          {submission.compilerTexts?.length ? (
            <div className="section__body"><pre className="compiler-text">{submission.compilerTexts.join('\n')}</pre></div>
          ) : null}
          {submission.judgeTexts?.length ? (
            <div className="section__body"><pre className="judge-text">{submission.judgeTexts.map(judgeText).join('\n')}</pre></div>
          ) : null}
          {cases.length ? (
            <div className="section__body no-padding">
              <table className="data-table record_detail__table">
                <colgroup><col className="col--case" /><col className="col--status" /><col className="col--time" /><col className="col--memory" /></colgroup>
                <thead><tr><th className="col--case record-status--border">#</th><th className="col--status">状态</th><th className="col--time">时间</th><th className="col--memory">内存</th></tr></thead>
                <tbody>
                  {cases.map((item, index) => {
                    const status = Number(item.status ?? 0)
                    const itemClass = statusClass(status)
                    return (
                      <tr className="case" key={String(item.id ?? index)}>
                        <td className={`col--case record-status--border ${itemClass}`}>#{String(item.id ?? index + 1)}</td>
                        <td className="col--status">
                          <span className={`icon record-status--icon ${itemClass}`} />
                          <span className={`record-status--text ${itemClass}`}>{statusLabel(status)}</span>
                          {item.message ? <span className="message">{judgeText(item.message)}</span> : null}
                        </td>
                        <td className="col--time">{item.time ? `${String(item.time)}ms` : '-'}</td>
                        <td className="col--memory">{item.memory ? formatMemory(Number(item.memory)) : '-'}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          ) : null}
        </div>
        {submission.code ? (
          <div className="section">
            <div className="section__header"><h1 className="section__title">代码</h1></div>
            <div className="section__body"><pre className="line-numbers"><code className={`language-${submission.lang}`}>{submission.code}</code></pre></div>
          </div>
        ) : null}
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__header"><h1 className="section__title">信息</h1></div>
          <div className="section__body typo no-media">
            <dl className="large horizontal">
              <dt>提交者</dt>
              <dd>{submission.submitter ? <Link to={`/user/${submission.submitter.id}`}>{submission.submitter.displayName || submission.submitter.username}</Link> : `UID ${submission.uid}`}</dd>
              {submission.problem ? (
                <>
                  <dt>题目</dt>
                  <dd><Link to={`/p/${encodeURIComponent(submission.problem.pid || String(submission.problem.docId))}`}>{submission.problem.pid} {submission.problem.title}</Link></dd>
                </>
              ) : null}
              <dt>语言</dt><dd>{languageNames[submission.lang] || submission.lang}</dd>
              {submission.code ? <><dt>代码长度</dt><dd>{new Blob([submission.code]).size} B</dd></> : null}
              <dt>提交时间</dt><dd>{new Date(submission.createdAt).toLocaleString('zh-CN', { hour12: false })}</dd>
              {submission.judgeAt ? <><dt>评测时间</dt><dd>{new Date(submission.judgeAt).toLocaleString('zh-CN', { hour12: false })}</dd></> : null}
            </dl>
            <dl className="large horizontal" id="summary">
              <dt>分数</dt><dd>{submission.score}</dd>
              {submission.time ? <><dt>总时间</dt><dd>{[3, 4, 5].includes(submission.status) ? '≥' : ''}{submission.time}ms</dd></> : null}
              {submission.memory ? <><dt>峰值内存</dt><dd>{[3, 4, 5].includes(submission.status) ? '≥' : ''}{formatMemory(submission.memory)}</dd></> : null}
            </dl>
          </div>
        </div>
      </div>
    </div>
  )
}
