import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getHomeworks, type Contest } from '../api.ts'

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

export function HomeworksPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
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
    getHomeworks(query)
      .then(setItems)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
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
      <div className="medium-12 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">全部作业</h1>
            <div className="section__tools">
              <form method="get" className="filter-form" name="filter-form" onSubmit={handleSearch}>
                <input type="text" className="inline compact textbox" name="q" value={input} onChange={(event) => setInput(event.target.value)} placeholder="搜索作业..." />
                <button type="submit" className="compact button">搜索</button>
              </form>
            </div>
          </div>
          <div className="section__body no-padding">
            {loading ? <div className="nothing-placeholder">加载中...</div> : null}
            {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
            {!loading && !error && items.length === 0 ? <div className="nothing-placeholder"><div className="nothing-icon" />暂无作业</div> : null}
            {!loading && !error && items.length > 0 ? (
              <ol className="section__list homework__list">
                {items.map((homework) => {
                  const deadline = homework.penaltySince || homework.endAt
                  const date = new Date(homework.penaltySince || homework.beginAt)
                  return (
                    <li className="section__list__item homework__item" key={homework.id}>
                      <div className="media">
                        <div className="media__left medium">
                          <div className="homework__date numbox">
                            <div className="numbox__num large">{date.getDate()}</div>
                            <div className="numbox__text">{date.getFullYear()}-{date.getMonth() + 1}</div>
                          </div>
                        </div>
                        <div className="media__body medium">
                          <h1 className="homework__title"><Link to={`/homework/${homework.docId}`}>{homework.title}</Link></h1>
                          <ul className="supplementary list">
                            <li>状态: {homeworkStatus(homework)}</li>
                            <li>开始时间: {formatDate(homework.beginAt)}</li>
                            <li>截止时间: {formatDate(deadline)}</li>
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
    </div>
  )
}
