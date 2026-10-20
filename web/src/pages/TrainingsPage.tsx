import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getTrainings, type Training } from '../api.ts'
import { useAuth } from '../auth.tsx'

export function TrainingsPage() {
  const { user } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const [input, setInput] = useState(query)
  const [items, setItems] = useState<Training[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setInput(query)
  }, [query])

  useEffect(() => {
    setLoading(true)
    setError('')
    getTrainings(query)
      .then(setItems)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
      .finally(() => setLoading(false))
  }, [query])

  function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const params = new URLSearchParams()
    if (input.trim()) params.set('q', input.trim())
    setSearchParams(params)
  }

  return (
    <div className="row">
      <div className="large-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">全部训练计划</h1>
            <div className="section__tools">
              <form method="get" className="filter-form" name="filter-form" onSubmit={handleSearch}>
                <input type="text" className="inline compact textbox" name="q" value={input} onChange={(event) => setInput(event.target.value)} placeholder="搜索训练..." />
                <button type="submit" className="compact button">搜索</button>
              </form>
            </div>
          </div>
          {loading ? <div className="nothing-placeholder">加载中...</div> : null}
          {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
          {!loading && !error && items.length === 0 ? <div className="nothing-placeholder"><div className="nothing-icon" />暂无训练计划</div> : null}
          {!loading && !error && items.length > 0 ? (
            <ol className="section__list all primary training__list">
              {items.map((training) => (
                <li className="section__list__item training__item" key={training.id}>
                  <div className="media">
                    <div className="media__left middle">
                      <div className="training__participants numbox">
                        <div className="numbox__num large">{training.attend}</div>
                        <div className="numbox__text">报名</div>
                      </div>
                    </div>
                    <div className="media__body middle">
                      <h1 className="training__title"><Link to={`/training/${training.docId}`}>{training.title}</Link></h1>
                      <div className="training__intro typo"><p>{training.content || ''}</p></div>
                      <ul className="supplementary list training__progress">
                        <li><span className="icon icon-flag text-blue" /> {training.sectionCount} 个章节, {training.problemCount} 道题</li>
                        {training.pin ? <li><span className="icon icon-star" /> 置顶</li> : null}
                      </ul>
                    </div>
                  </div>
                </li>
              ))}
            </ol>
          ) : null}
        </div>
      </div>
      <div className="large-3 columns">
        {user && ((user.priv ?? 0) & 1) !== 0 ? (
          <div className="section side">
            <div className="section__header"><h1 className="section__title">创建训练计划</h1></div>
            <ol className="menu">
              <li className="menu__item">
                <Link to="/training/new" className="menu__link"><span className="icon icon-add" /> 新建训练计划</Link>
              </li>
            </ol>
            <div className="section__body">
              <div className="typo supplementary"><p>你可以创建自己的训练计划并分享给其他人。</p></div>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  )
}
