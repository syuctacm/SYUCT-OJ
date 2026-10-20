import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { getProblemTags, getTagRanking, type ProblemTagStat, type TagRanking } from '../api.ts'

export function TagRankingPage() {
  const { tag = '' } = useParams()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const page = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [input, setInput] = useState(query)
  const [ranking, setRanking] = useState<TagRanking | null>(null)
  const [allTags, setAllTags] = useState<ProblemTagStat[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    setInput(query)
  }, [query])

  useEffect(() => {
    Promise.all([getTagRanking(tag, page, query), getProblemTags()])
      .then(([data, tags]) => { setRanking(data); setAllTags(tags) })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '标签榜单加载失败'))
  }, [tag, page, query])

  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const params = new URLSearchParams()
    if (input.trim()) params.set('q', input.trim())
    setSearchParams(params)
  }

  const algorithmTags = allTags.filter((item) => !item.isSource)
  const sourceTags = allTags.filter((item) => item.isSource)
  const totalPages = ranking ? Math.max(1, Math.ceil(ranking.matchedCount / ranking.limit)) : 1
  const pageLink = (value: number) => {
    const params = new URLSearchParams({ page: String(value) })
    if (query) params.set('q', query)
    return `/p/tag/${encodeURIComponent(tag)}?${params.toString()}`
  }

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!ranking) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <div className="row"><div className="columns">
      <div className="section">
        <div className="section__header">
          <h1 className="section__title">{tag}</h1>
          <div className="section__tools" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <select className="select filter" value="" onChange={(event) => {
              if (event.target.value) navigate(`/p/tag/${encodeURIComponent(event.target.value)}`)
            }}>
              <option value="" disabled>切换标签...</option>
              <optgroup label="算法知识点">
                {algorithmTags.map((item) => <option value={item.tag} key={item.tag}>{item.tag} ({item.count} 题)</option>)}
              </optgroup>
              <optgroup label="题目来源">
                {sourceTags.map((item) => <option value={item.tag} key={item.tag}>{item.tag} ({item.count} 题)</option>)}
              </optgroup>
            </select>
            <Link to="/p" className="button"><span className="icon icon-arrow-left" /> 返回题库</Link>
          </div>
        </div>
        <div className="section__body" style={{ paddingTop: 14, paddingBottom: 14, borderBottom: '1px solid #e4e4e4' }}>
          <form method="get" onSubmit={search}>
            <div className="row">
              <div className="medium-5 columns">
                <div className="search">
                  <input name="q" type="text" value={input} onChange={(event) => setInput(event.target.value)} placeholder="搜索账号 / 用户名 / UID" />
                  <button type="submit"><span className="icon icon-search" /></button>
                </div>
              </div>
              <div className="medium-7 columns text-right" style={{ lineHeight: '32px' }}>
                {query ? <span className="supplementary">匹配到 {ranking.matchedCount} 个账号（全站上榜 {ranking.totalUsers} 人）</span> : <span className="supplementary">题库收录 {ranking.problemCount} 题 · 共 {ranking.totalUsers} 人有过题记录</span>}
              </div>
            </div>
          </form>
        </div>
        <div className="section__body no-padding">
          {ranking.items.length === 0 ? <div className="nothing-placeholder"><div className="nothing-icon" />暂无用户在该标签下有过题记录</div> : (
            <>
              <table className="data-table">
                <colgroup><col className="col--rank" /><col className="col--user" /><col className="col--ac" /><col className="col--tried" /><col className="col--rate" /></colgroup>
                <thead><tr><th className="col--rank">排名</th><th className="col--user">用户</th><th className="col--ac center">通过</th><th className="col--tried center">尝试</th><th className="col--rate center">通过率</th></tr></thead>
                <tbody>
                  {ranking.items.map((item) => {
                    const rate = item.submissions > 0 ? ((item.solved / item.submissions) * 100).toFixed(1) : '0.0'
                    return (
                      <tr key={item.uid}>
                        <td className="col--rank">{item.rank <= 3 ? <strong>{item.rank}</strong> : item.rank}</td>
                        <td className="col--user"><Link to={`/user/${item.uid}`}>{item.displayName || item.username || `UID ${item.uid}`}</Link></td>
                        <td className="col--ac center">{item.solved}</td>
                        <td className="col--tried center">{item.submissions}</td>
                        <td className="col--rate center">{rate}%</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
              <div style={{ padding: 12, textAlign: 'center' }}>
                <ul className="pager">
                  {page > 1 ? <li><Link className="pager__item previous link" to={pageLink(page - 1)}>上一页</Link></li> : null}
                  <li><span className="pager__item current">{page} / {totalPages}</span></li>
                  {page < totalPages ? <li><Link className="pager__item next link" to={pageLink(page + 1)}>下一页</Link></li> : null}
                </ul>
                <div className="supplementary">共 {ranking.matchedCount} 名用户，每页 {ranking.limit} 人</div>
              </div>
            </>
          )}
        </div>
      </div>
    </div></div>
  )
}
