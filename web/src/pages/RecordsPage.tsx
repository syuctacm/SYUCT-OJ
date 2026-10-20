import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getSubmissions, type SubmissionPage } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { RecordStatusCell } from '../components/RecordStatus.tsx'
import { statusLabels } from '../status.ts'

const languageNames: Record<string, string> = {
  cpp: 'C++17',
  python3: 'Python 3',
}

function formatMemory(kib: number) {
  if (!kib) return '-'
  return kib >= 1024 ? `${(kib / 1024).toFixed(1)}MiB` : `${kib}KiB`
}

function formatTime(ms: number) {
  return ms ? `${Math.round(ms)}ms` : '-'
}

export function RecordsPage() {
  const { user, loading: authLoading } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const filters = {
    uidOrName: searchParams.get('uidOrName') ?? '',
    pid: searchParams.get('pid') ?? '',
    tid: searchParams.get('tid') ?? '',
    lang: searchParams.get('lang') ?? '',
    status: searchParams.get('status') ?? '',
    page: Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1),
  }
  const [uidOrName, setUidOrName] = useState(filters.uidOrName)
  const [pid, setPid] = useState(filters.pid)
  const [tid, setTid] = useState(filters.tid)
  const [page, setPage] = useState<SubmissionPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setUidOrName(filters.uidOrName)
    setPid(filters.pid)
  }, [filters.uidOrName, filters.pid])

  useEffect(() => {
    if (authLoading) return
    if (!user) {
      setLoading(false)
      return
    }
    setLoading(true)
    setError('')
    getSubmissions(filters)
      .then(setPage)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '提交记录加载失败'))
      .finally(() => setLoading(false))
  }, [authLoading, user, filters.uidOrName, filters.pid, filters.tid, filters.lang, filters.status, filters.page])

  function handleFilter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const params = new URLSearchParams()
    if (uidOrName.trim()) params.set('uidOrName', uidOrName.trim())
    if (pid.trim()) params.set('pid', pid.trim())
    if (tid.trim()) params.set('tid', tid.trim())
    if (filters.lang) params.set('lang', filters.lang)
    if (filters.status) params.set('status', filters.status)
    setSearchParams(params)
  }

  function updateSelect(name: string, value: string) {
    const params = new URLSearchParams(searchParams)
    if (value) params.set(name, value)
    else params.delete(name)
    params.delete('page')
    setSearchParams(params)
  }

  if (authLoading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!user) return <div className="row"><div className="columns"><div className="nothing-placeholder">请先 <Link to="/login">登录</Link> 查看评测记录。</div></div></div>

  const nextHref = (() => {
    const params = new URLSearchParams(searchParams)
    params.set('page', String(filters.page + 1))
    return `/record?${params.toString()}`
  })()

  return (
    <div className="row">
      <div className="medium-12 columns">
        <form method="get" onSubmit={handleFilter}>
          <div className="section">
            <div className="section__header">
              <h1 className="section__title">筛选</h1>
              <div className="section__tools">
                <button type="submit" className="primary button"><span className="icon icon-filter" /> 筛选</button>
                <Link to="/record" className="button">重置</Link>
              </div>
            </div>
            <div className="section__body">
              <div className="row">
                <div className="medium-2 columns">
                  <label className="filter-user">
                    用户名 / UID
                    <input name="uidOrName" type="text" className="textbox" value={uidOrName} onChange={(event) => setUidOrName(event.target.value)} autoComplete="off" />
                  </label>
                </div>
                <div className="medium-2 columns">
                  <label>题目<input name="pid" type="text" className="textbox" value={pid} onChange={(event) => setPid(event.target.value)} /></label>
                </div>
                <div className="medium-2 columns">
                  <label>比赛<input name="tid" type="text" className="textbox" value={tid} onChange={(event) => setTid(event.target.value)} /></label>
                </div>
                <div className="medium-3 columns">
                  <label>语言</label>
                  <select className="compact select" name="lang" value={filters.lang} onChange={(event) => updateSelect('lang', event.target.value)}>
                    <option value="">所有语言</option>
                    <option value="cpp">C++17</option>
                    <option value="python3">Python 3</option>
                  </select>
                </div>
                <div className="medium-3 columns">
                  <label>状态</label>
                  <select className="compact select" name="status" value={filters.status} onChange={(event) => updateSelect('status', event.target.value)}>
                    <option value="">所有提交</option>
                    {Object.entries(statusLabels).map(([value, label]) => <option value={value} key={value}>{label}</option>)}
                  </select>
                </div>
              </div>
            </div>
          </div>
        </form>
        <div className="section">
          <div className="section__body no-padding">
            {loading ? <div className="nothing-placeholder">加载中...</div> : null}
            {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
            {!loading && !error && page?.items.length === 0 ? <div className="nothing-placeholder"><div className="nothing-icon" />没有匹配的提交记录</div> : null}
            {!loading && !error && page && page.items.length > 0 ? (
              <>
                <table className="data-table record_main__table">
                  <colgroup>
                    <col className="col--status" /><col className="col--problem" /><col className="col--submit-by" />
                    <col className="col--time" /><col className="col--memory" /><col className="col--lang" /><col className="col--submit-at" />
                  </colgroup>
                  <thead>
                    <tr>
                      <th className="col--status record-status--border">状态</th>
                      <th className="col--problem">题目</th>
                      <th className="col--submit-by">提交者</th>
                      <th className="col--time">时间</th>
                      <th className="col--memory">内存</th>
                      <th className="col--lang">语言</th>
                      <th className="col--submit-at">提交时间</th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.items.map((submission) => (
                      <tr data-rid={submission.id} key={submission.id}>
                        <RecordStatusCell status={submission.status} rid={submission.id} />
                        <td className="col--problem col--problem-name">
                          {submission.problem ? <Link to={`/p/${encodeURIComponent(submission.problem.pid || String(submission.problem.docId))}`}><b>{submission.problem.pid}</b>&nbsp;&nbsp;{submission.problem.title}</Link> : '*'}
                        </td>
                        <td className="col--submit-by">
                          {submission.submitter ? <Link to={`/user/${submission.submitter.id}`}>{submission.submitter.displayName || submission.submitter.username}</Link> : `UID ${submission.uid}`}
                        </td>
                        <td className="col--time">{submission.time ? `${[3, 4, 5].includes(submission.status) ? '≥' : ''}${formatTime(submission.time)}` : '-'}</td>
                        <td className="col--memory">{submission.memory ? `${[3, 4, 5].includes(submission.status) ? '≥' : ''}${formatMemory(submission.memory)}` : '-'}</td>
                        <td className="col--lang">{languageNames[submission.lang] || submission.lang}</td>
                        <td className="col--submit-at">{new Date(submission.createdAt).toLocaleString('zh-CN', { hour12: false })}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {page.page * page.limit < page.total ? (
                  <ul className="pager"><li><Link className="pager__item next link" to={nextHref}>下一页</Link></li></ul>
                ) : null}
              </>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}
