import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { getDiscussionNodes, getDiscussions, type Discussion, type DiscussionNode } from '../api.ts'
import { useAuth } from '../auth.tsx'

export function DiscussionsPage() {
  const { user } = useAuth()
  const [searchParams] = useSearchParams()
  const selectedNode = searchParams.get('node') ?? ''
  const [items, setItems] = useState<Discussion[]>([])
  const [nodes, setNodes] = useState<DiscussionNode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([getDiscussions(), getDiscussionNodes()])
      .then(([nextItems, nextNodes]) => {
        setItems(nextItems)
        setNodes(nextNodes)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
      .finally(() => setLoading(false))
  }, [])

  const filteredItems = selectedNode
    ? items.filter((item) => item.parentType === 20 && item.parentId === selectedNode)
    : items
  const currentTitle = nodes.find((node) => node.docId === selectedNode)?.title || '讨论'
  const groupedNodes = useMemo(() => {
    const groups = new Map<string, DiscussionNode[]>()
    for (const node of nodes) {
      const values = groups.get(node.category) ?? []
      values.push(node)
      groups.set(node.category, values)
    }
    return Array.from(groups.entries())
  }, [nodes])

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">{currentTitle}</h1>
          </div>
          {loading ? <div className="nothing-placeholder">加载中...</div> : null}
          {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
          {!loading && !error && filteredItems.length === 0 ? (
            <div className="nothing-placeholder">
              <div className="nothing-icon" />
              暂无讨论
            </div>
          ) : null}
          {!loading && !error && filteredItems.length > 0 ? (
            <ol className="section__list discussion__list">
              {filteredItems.map((item) => (
                <li className={`section__list__item discussion__item${item.highlight ? ' highlight' : ''}`} key={item.id}>
                  <div className="media">
                    <div className="media__left">
                      <div className="discussion__replies numbox">
                        <div className="numbox__num large">{item.nReply}</div>
                        <div className="numbox__text">回复</div>
                      </div>
                    </div>
                    <div className="media__body top">
                      <h1 className="discussion__title">
                        <Link to={`/discuss/${item.docId}`}>{item.title}</Link>
                      </h1>
                      <ul className="supplementary list">
                        <li>
                          <Link className="discussion-node-tag" to={`/discuss?node=${encodeURIComponent(String(item.parentId ?? ''))}`}>
                            <span className="v-center icon icon-flag" />
                            {item.parentTitle || String(item.parentId || '讨论')}
                          </Link>
                        </li>
                        <li>{item.views} 浏览</li>
                        <li>
                          <Link to={`/user/${item.owner}`}>UID {item.owner}</Link>
                          {' @ '}
                          {new Date(item.updateAt).toLocaleString('zh-CN', { hour12: false })}
                        </li>
                      </ul>
                    </div>
                  </div>
                </li>
              ))}
            </ol>
          ) : null}
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__header">
            <h1 className="section__title">{selectedNode ? currentTitle : '创建讨论'}</h1>
          </div>
          <div className="section__body">
            {user ? (
              <p><Link to={selectedNode ? `/discuss/new?node=${encodeURIComponent(selectedNode)}` : '/discuss/new'} className="expanded primary button">创建讨论</Link></p>
            ) : (
              <p><Link to="/login" className="expanded primary button">登录后创建讨论</Link></p>
            )}
          </div>
        </div>
        <div className="section side">
          <div className="section__header">
            <h1 className="section__title">讨论节点</h1>
          </div>
          <div className="section__body">
            <ul className="group-list">
              {groupedNodes.map(([category, values]) => (
                <li className="group-list__item" key={category}>
                  <h2 className="section__title">{category}</h2>
                  <ol className="chip-list">
                    {values.map((node) => (
                      <li className="chip-list__item" key={node.docId}>
                        <Link className="typo-a" to={`/discuss?node=${encodeURIComponent(node.docId)}`}>{node.title || node.docId}</Link>
                      </li>
                    ))}
                  </ol>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </div>
  )
}
