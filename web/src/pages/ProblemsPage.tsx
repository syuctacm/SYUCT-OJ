import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getProblems, type ProblemPage, type ProblemSummary } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { Pager } from '../components/Pager.tsx'
import { statusLabel } from '../status.ts'

function problemStatusClass(status: number) {
  if (status === 1) return 'pass'
  if ([0, 20, 21, 22].includes(status)) return status === 0 ? 'pending' : 'progress'
  return 'fail'
}

function problemLink(problem: ProblemSummary) {
  return `/p/${encodeURIComponent(problem.pid || String(problem.docId))}`
}

export function ProblemsPage() {
  const { user } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const tag = searchParams.get('tag') ?? ''
  const sort = searchParams.get('sort') === 'recent' ? 'recent' : 'default'
  const pageNumber = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [input, setInput] = useState(query)
  const [page, setPage] = useState<ProblemPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showTags, setShowTags] = useState(false)

  useEffect(() => {
    setInput(query)
  }, [query])

  useEffect(() => {
    setLoading(true)
    setError('')
    getProblems(query, pageNumber, sort, tag)
      .then(setPage)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
      .finally(() => setLoading(false))
  }, [query, pageNumber, sort, tag])

  function updateParams(values: { q?: string; tag?: string; sort?: string }) {
    const params = new URLSearchParams()
    if (values.q) params.set('q', values.q)
    if (values.tag) params.set('tag', values.tag)
    if (values.sort && values.sort !== 'default') params.set('sort', values.sort)
    setSearchParams(params)
  }

  function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    updateParams({ q: input.trim(), tag, sort })
  }

  const tagCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const problem of page?.items ?? []) {
      for (const value of problem.tags ?? []) counts.set(value, (counts.get(value) ?? 0) + 1)
    }
    return Array.from(counts.entries()).sort((a, b) => b[1] - a[1])
  }, [page])

  const luckyProblem = page?.items.length ? page.items[Math.floor(Math.random() * page.items.length)] : null

  return (
    <>
      <div className="medium-9 columns">
        <div className="section display-mode">
          <div className="section__body no-padding">
            <form method="get" id="searchForm" onSubmit={handleSearch}>
              <div className="search">
                <input name="q" type="text" value={input} onChange={(event) => setInput(event.target.value)} placeholder="搜索" />
                <div data-fragment-id="problem_stat"><p>{page?.total ?? 0} 道题</p></div>
                <select name="sort" className="search-sort" value={sort} onChange={(event) => updateParams({ q: input.trim(), tag, sort: event.target.value })}>
                  <option value="default">默认排序</option>
                  <option value="recent">最近添加</option>
                </select>
                <button type="submit"><span className="icon icon-search" /></button>
              </div>
            </form>
          </div>
          <div className="section__body no-padding">
            {loading ? <div className="nothing-placeholder">加载中...</div> : null}
            {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
            {!loading && !error && page && page.items.length === 0 ? (
              <div className="nothing-placeholder"><div className="nothing-icon" />没有找到题目</div>
            ) : null}
            {!loading && !error && page && page.items.length > 0 ? (
              <div data-fragment-id="problem_list">
                <Pager page={page.page} total={page.total} limit={page.limit} query={query} sort={sort} tag={tag} position="top" />
                <table className={`data-table${showTags ? '' : ' hide-problem-tag'}`}>
                  <colgroup>
                    <col className="col--checkbox display-mode-hide" />
                    <col className="col--pid display-mode-hide" />
                    {user ? <col className="col--status edit-mode-hide" /> : null}
                    <col className="col--name" />
                    <col className="col--ac-tried" />
                    <col className="col--difficulty" />
                  </colgroup>
                  <thead>
                    <tr>
                      <th className="col--checkbox display-mode-hide"><label className="compact checkbox"><input type="checkbox" name="select_all" disabled /></label></th>
                      <th className="col--pid display-mode-hide">ID</th>
                      {user ? <th className="col--status record-status--border edit-mode-hide">状态</th> : null}
                      <th className="col--name">
                        题目
                        <span className="nojs--hide toggle-tag tag--hide" onClick={() => setShowTags(true)}>显示标签</span>
                        <span className="nojs--hide toggle-tag notag--hide" onClick={() => setShowTags(false)}>隐藏标签</span>
                      </th>
                      <th className="col--ac-tried">通过 / 尝试</th>
                      <th className="col--difficulty">难度</th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.items.map((problem) => {
                      const statusClass = problem.userStatus ? problemStatusClass(problem.userStatus.status) : ''
                      return (
                        <tr data-pid={problem.docId} key={problem.id}>
                          <td className="col--checkbox display-mode-hide"><label className="compact checkbox"><input type="checkbox" disabled /></label></td>
                          <td className="col--pid display-mode-hide">{problem.docId}</td>
                          {user ? (
                            <td className={`col--status record-status--border edit-mode-hide ${statusClass}`}>
                              {problem.userStatus ? (
                                <div className="col--status__text">
                                  <span className={`icon record-status--icon ${statusClass}`} />
                                  <Link className={`record-status--text ${statusClass}`} to={`/record/${problem.userStatus.rid}`}>{statusLabel(problem.userStatus.status)}</Link>
                                </div>
                              ) : null}
                            </td>
                          ) : null}
                          <td className="col--name col--problem-name">
                            <Link to={problemLink(problem)}><b>{problem.pid || problem.docId}</b>&nbsp;&nbsp;{problem.title}</Link>
                            {(problem.tags?.length ?? 0) > 0 ? (
                              <ul className="problem__tags">
                                {problem.tags?.map((value) => (
                                  <li className="problem__tag" key={value}>
                                    <Link className="problem__tag-link" to={`/p?tag=${encodeURIComponent(value)}`}>{value}</Link>
                                  </li>
                                ))}
                              </ul>
                            ) : null}
                          </td>
                          <td className="col--ac-tried">{problem.nAccept} / {problem.nSubmit}</td>
                          <td className="col--difficulty">{problem.difficulty || '(None)'}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
                <Pager page={page.page} total={page.total} limit={page.limit} query={query} sort={sort} tag={tag} position="bottom" />
              </div>
            ) : null}
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__header edit-mode-hide"><h1 className="section__title">标签统计</h1></div>
          <div className="section__body">
            <ul className="group-list">
              {tagCounts.map(([value, count]) => (
                <li className="group-list__item" key={value}>
                  <Link className="typo-a" to={`/p?tag=${encodeURIComponent(value)}`}>{value}</Link>
                  {' '}
                  <span className="supplementary">{count} 题</span>
                </li>
              ))}
            </ul>
          </div>
          <div className="section__body edit-mode-hide">
            {luckyProblem ? (
              <p>
                <Link to={problemLink(luckyProblem)} className="expanded button" target="_blank">
                  <span className="icon icon-feeling-lucky" /> Lucky
                </Link>
              </p>
            ) : null}
            <div className="typo supplementary"><p>根据当前筛选随机选择一道题。</p></div>
          </div>
        </div>
      </div>
    </>
  )
}
