import { useEffect, useState, type FormEvent } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { createDiscussion, getDiscussionNodes, type DiscussionNode } from '../api.ts'
import { useAuth } from '../auth.tsx'

export function DiscussionCreatePage() {
  const { user, loading: authLoading } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [nodes, setNodes] = useState<DiscussionNode[]>([])
  const [nodeId, setNodeId] = useState(searchParams.get('node') ?? '')
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [pin, setPin] = useState(false)
  const [highlight, setHighlight] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    getDiscussionNodes()
      .then((items) => {
        setNodes(items)
        setNodeId((current) => current || items[0]?.docId || '')
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论节点加载失败'))
  }, [])

  const isAdmin = ((user?.priv ?? 0) & 1) !== 0
  const currentNode = nodes.find((node) => node.docId === nodeId)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      const discussion = await createDiscussion({ nodeId, title, content, pin, highlight })
      navigate(`/discuss/${discussion.docId}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : '发帖失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (authLoading) return <div className="row"><div className="columns"><div className="nothing-placeholder">正在确认登录状态...</div></div></div>
  if (!user) return <Navigate to="/login" replace />

  return (
    <>
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">{currentNode ? `在 ${currentNode.title || currentNode.docId} 中创建讨论` : '创建讨论'}</h1>
          </div>
          <div className="section__body">
            <form onSubmit={handleSubmit}>
              <div className="row">
                <div className="medium-6 columns form__item">
                  <label>
                    讨论节点
                    <div className="select-container">
                      <select name="nodeId" className="select" value={nodeId} onChange={(event) => setNodeId(event.target.value)} required>
                        {nodes.map((node) => (
                          <option value={node.docId} key={node.docId}>{node.category} / {node.title || node.docId}</option>
                        ))}
                      </select>
                    </div>
                  </label>
                </div>
              </div>
              <div className="row">
                <div className="medium-8 columns">
                  <label>
                    标题
                    <input name="title" placeholder="标题" value={title} onChange={(event) => setTitle(event.target.value)} className="textbox" autoFocus required maxLength={200} />
                  </label>
                </div>
                <div className="medium-2 columns">
                  <label>
                    <br />
                    <label className="checkbox">
                      <input type="checkbox" name="highlight" className="checkbox" checked={highlight} disabled={!isAdmin} onChange={(event) => setHighlight(event.target.checked)} />
                      高亮
                    </label>
                  </label>
                </div>
                <div className="medium-2 columns">
                  <label>
                    <br />
                    <label className="checkbox">
                      <input type="checkbox" name="pin" className="checkbox" checked={pin} disabled={!isAdmin} onChange={(event) => setPin(event.target.checked)} />
                      置顶
                    </label>
                  </label>
                </div>
              </div>
              <div className="row">
                <div className="medium-12 columns form__item end" data-hotkey="ctrl+enter:submit">
                  <label>
                    内容 (必填)
                    <div className="textarea-container">
                      <textarea
                        name="content"
                        data-markdown=""
                        className="textbox auto-resize"
                        style={{ height: 500 }}
                        value={content}
                        onChange={(event) => setContent(event.target.value)}
                        onKeyDown={(event) => {
                          if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                            event.preventDefault()
                            event.currentTarget.form?.requestSubmit()
                          }
                        }}
                        required
                      />
                    </div>
                  </label>
                </div>
              </div>
              {error ? <p className="form-error">{error}</p> : null}
              <div className="row"><div className="columns">
                <button type="submit" className="rounded primary button" disabled={submitting || nodes.length === 0}>
                  {submitting ? '发布中...' : '创建 (Ctrl+Enter)'}
                </button>
                <button type="button" className="rounded button" onClick={() => navigate(-1)}>取消</button>
              </div></div>
            </form>
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__body typo">
            <dl>
              <dt>行内代码</dt>
              <dd><pre>`std::cout()`</pre></dd>
              <dt>代码块</dt>
              <dd><pre>{'```c++\nint main() {\n  return 0;\n}\n```'}</pre></dd>
              <dt>行内公式</dt>
              <dd><pre>$E=mc^2$</pre></dd>
              <dt>公式块</dt>
              <dd><pre>$$E=mc^2$$</pre></dd>
              <dt>链接</dt>
              <dd><pre>[文本](https://...)</pre></dd>
              <dt>高亮</dt>
              <dd><pre>==文本==</pre></dd>
              <dt>图片</dt>
              <dd><pre>![描述](https://...)</pre></dd>
              <dt>指定尺寸图片</dt>
              <dd><pre>![描述](url =100x100)</pre></dd>
              <dt>脚注</dt>
              <dd><pre>{'Here is a ref[^1].\n\n[^1]: And the footnote.'}</pre></dd>
              <dt>视频</dt>
              <dd><pre>@[video](https://.../a.mp4)</pre></dd>
              <dt>提及</dt>
              <dd><pre>@[](/user/uid)</pre></dd>
            </dl>
            <h2>了解更多</h2>
            <ul>
              <li><a href="http://commonmark.org/help/" target="_blank" rel="noreferrer"><span className="icon icon-link--external" /> CommonMark 语法</a></li>
              <li><a href="http://www.onemathematicalcat.org/MathJaxDocumentation/TeXSyntax.htm" target="_blank" rel="noreferrer"><span className="icon icon-link--external" /> LaTeX 表达式</a></li>
            </ul>
          </div>
        </div>
      </div>
    </>
  )
}
