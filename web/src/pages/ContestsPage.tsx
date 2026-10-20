import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getContests, type Contest } from '../api.ts'

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

function ImportantContest({ contest, view, title }: { contest: Contest; view: 'live' | 'ready'; title: string }) {
  return (
    <div className={`section immersive--content section--contest ${view}`}>
      <div className="media">
        <div className="media__body middle">
          <div className="section__body">
            <div><h2 className="status_title">{title}</h2></div>
            <h1>{contest.title}</h1>
            <ul className="info">
              <li><span className="icon icon-award" /> 赛制: {ruleNames[contest.rule] || contest.rule}</li>
              <li><span className="icon icon-calendar" /> 开始: {formatDate(contest.beginAt)}</li>
              <li><span className="icon icon-schedule--fill" /> 时长: {durationOf(contest)} 小时</li>
              <li><span className="icon icon-user--multiple" /> 参赛: {contest.attend}</li>
            </ul>
          </div>
        </div>
        <div className="media__right middle">
          <div className="section__body">
            <Link to={`/contest/${contest.docId}`} className="detail-button">查看详情</Link>
          </div>
        </div>
      </div>
    </div>
  )
}

export function ContestsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const rule = searchParams.get('rule') ?? ''
  const [input, setInput] = useState(query)
  const [items, setItems] = useState<Contest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setInput(query)
  }, [query])

  useEffect(() => {
    setLoading(true)
    setError('')
    getContests(query, rule)
      .then(setItems)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
      .finally(() => setLoading(false))
  }, [query, rule])

  function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const params = new URLSearchParams()
    if (input.trim()) params.set('q', input.trim())
    if (rule) params.set('rule', rule)
    setSearchParams(params)
  }

  const now = Date.now()
  const important = items.filter((contest) => new Date(contest.beginAt).getTime() <= now && new Date(contest.endAt).getTime() >= now)
  const upcoming = items.filter((contest) => new Date(contest.beginAt).getTime() > now)

  return (
    <div className="row">
      <div className="medium-9 columns">
        {important.map((contest) => <ImportantContest key={`live-${contest.id}`} contest={contest} view="live" title="Live..." />)}
        {upcoming.map((contest) => <ImportantContest key={`ready-${contest.id}`} contest={contest} view="ready" title="Ready (☆▽☆)" />)}
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">全部比赛</h1>
            <div className="section__tools">
              <form method="get" name="filter-form" onSubmit={handleSearch}>
                <input type="text" className="inline compact textbox" name="q" value={input} onChange={(event) => setInput(event.target.value)} placeholder="搜索比赛..." />
                <select className="inline compact select" name="rule" value={rule} onChange={(event) => {
                  const params = new URLSearchParams()
                  if (input.trim()) params.set('q', input.trim())
                  if (event.target.value) params.set('rule', event.target.value)
                  setSearchParams(params)
                }}>
                  <option value="">全部</option>
                  {Object.entries(ruleNames).map(([key, label]) => <option value={key} key={key}>{label}</option>)}
                </select>
              </form>
            </div>
          </div>
          <div className="section__body no-padding">
            {loading ? <div className="nothing-placeholder">加载中...</div> : null}
            {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
            {!loading && !error && items.length === 0 ? <div className="nothing-placeholder"><div className="nothing-icon" />暂无比赛</div> : null}
            {!loading && !error && items.length > 0 ? (
              <ol className="section__list contest__list">
                {items.map((contest) => {
                  const begin = new Date(contest.beginAt)
                  return (
                    <li className={`section__list__item contest__item contest-type--${contest.rule}`} key={contest.id}>
                      <div className="media">
                        <div className="media__left medium">
                          <div className="contest__date numbox">
                            <div className="numbox__num large">{begin.getDate()}</div>
                            <div className="numbox__text">{begin.getFullYear()}-{begin.getMonth() + 1}</div>
                          </div>
                        </div>
                        <div className="media__body medium">
                          <h1 className="contest__title"><Link to={`/contest/${contest.docId}`}>{contest.title}</Link></h1>
                          <ul className="supplementary list">
                            <li><Link to={`?rule=${encodeURIComponent(contest.rule)}`} className="contest-type-tag"><span className="icon icon-award" />{ruleNames[contest.rule] || contest.rule}</Link></li>
                            {contest.rated ? <li className="contest-tag-rated"><span className="icon icon-star" />Rated</li> : null}
                            <li><span className="icon icon-schedule--fill text-blue" /> {durationOf(contest)} 小时</li>
                            <li><span className="icon icon-user--multiple" /> {contest.attend}</li>
                          </ul>
                        </div>
                      </div>
                    </li>
                  )
                })}
              </ol>
            ) : null}
          </div>
        </div>
      </div>
      <div className="medium-3 columns" />
    </div>
  )
}
