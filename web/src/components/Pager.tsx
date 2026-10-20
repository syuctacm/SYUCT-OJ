import { Link } from 'react-router-dom'

type PagerProps = {
  page: number
  total: number
  limit: number
  query: string
  sort: string
  tag: string
  position: 'top' | 'bottom'
}

function pageItems(page: number, totalPages: number): Array<number | 'ellipsis'> {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, index) => index + 1)
  const pages = new Set([1, totalPages, page - 1, page, page + 1])
  const valid = Array.from(pages).filter((value) => value >= 1 && value <= totalPages).sort((a, b) => a - b)
  const result: Array<number | 'ellipsis'> = []
  for (const value of valid) {
    const previous = result[result.length - 1]
    if (typeof previous === 'number' && value - previous > 1) result.push('ellipsis')
    result.push(value)
  }
  return result
}

export function Pager({ page, total, limit, query, sort, tag, position }: PagerProps) {
  const totalPages = Math.ceil(total / limit)
  if (totalPages <= 0) return null
  const href = (target: number) => {
    const params = new URLSearchParams({ page: String(target) })
    if (query) params.set('q', query)
    if (sort && sort !== 'default') params.set('sort', sort)
    if (tag) params.set('tag', tag)
    return `/p?${params.toString()}`
  }
  const items = pageItems(page, totalPages)

  return (
    <ul className={`pager pager-${position}`}>
      {page > 1 ? <li><Link className="pager__item first link" to={href(1)}>首页</Link></li> : null}
      {page > 1 ? <li><Link className="pager__item previous link" to={href(page - 1)}>上一页</Link></li> : null}
      {items.map((item, index) => item === 'ellipsis'
        ? <li key={`ellipsis-${index}`}><span className="pager__item ellipsis">...</span></li>
        : <li key={item}>
            {item === page
              ? <span className="pager__item current">{item}</span>
              : <Link className="pager__item page link" to={href(item)}>{item}</Link>}
          </li>)}
      {page < totalPages ? <li><Link className="pager__item next link" to={href(page + 1)}>下一页</Link></li> : null}
      {page < totalPages ? <li><Link className="pager__item last link" to={href(totalPages)}>末页</Link></li> : null}
    </ul>
  )
}
