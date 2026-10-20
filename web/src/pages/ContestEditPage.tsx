import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  createContest,
  getContestEditor,
  updateContest,
  type ContestEditInput,
} from '../api.ts'
import { useAuth } from '../auth.tsx'

const ruleNames: Record<string, string> = {
  acm: 'XCPC / ACM',
  oi: 'OI',
  ioi: 'IOI',
  strictioi: 'Strict IOI',
  ledo: 'Ledo',
}

function toLocalInput(value: string | Date) {
  const date = typeof value === 'string' ? new Date(value) : value
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

function nextHour() {
  const date = new Date()
  date.setMinutes(0, 0, 0)
  date.setHours(date.getHours() + 1)
  return date
}

export function ContestEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { tid = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [rule, setRule] = useState('acm')
  const [beginAt, setBeginAt] = useState(toLocalInput(nextHour()))
  const [duration, setDuration] = useState(3)
  const [pids, setPids] = useState('')
  const [rated, setRated] = useState(true)
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    getContestEditor(tid)
      .then((item) => {
        setTitle(item.title)
        setContent(item.content || '')
        setRule(item.rule)
        setBeginAt(toLocalInput(item.beginAt))
        setDuration(item.duration || Math.max(1, Math.round((new Date(item.endAt).getTime() - new Date(item.beginAt).getTime()) / 3600000)))
        setPids((item.pids ?? []).join(', '))
        setRated(Boolean(item.rated))
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
      .finally(() => setLoading(false))
  }, [mode, tid])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const begin = new Date(beginAt)
    if (Number.isNaN(begin.getTime()) || duration <= 0) {
      setError('开始时间或时长不正确')
      return
    }
    const end = new Date(begin.getTime() + duration * 3600000)
    const problemIDs = pids.split(/[,，]/).map((value) => Number.parseInt(value.trim(), 10)).filter((value) => value > 0)
    if (!problemIDs.length) {
      setError('至少需要一道题目')
      return
    }
    const input: ContestEditInput = {
      title: title.trim(),
      content,
      beginAt: begin.toISOString(),
      endAt: end.toISOString(),
      rule,
      pids: problemIDs,
      duration,
      rated,
      unlocked: false,
    }
    setSaving(true)
    setError('')
    try {
      const item = mode === 'create' ? await createContest(input) : await updateContest(tid, input)
      navigate(`/contest/${item.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '比赛保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (user && ((user.priv ?? 0) & 1) === 0) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">需要管理员权限</div></div></div>
  }
  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (error && mode === 'edit' && !title) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <form method="post" onSubmit={handleSubmit}>
            <div className="section__header"><h2 className="section__title">基本信息</h2></div>
            <div className="section__body">
              <div className="row">
                <div className="medium-3 columns form__item">
                  <label>
                    赛制
                    <div className="select-container">
                      <select className="select" value={rule} onChange={(event) => setRule(event.target.value)}>
                        {Object.entries(ruleNames).map(([key, label]) => <option value={key} key={key}>{label}</option>)}
                      </select>
                    </div>
                  </label>
                </div>
                <div className="medium-9 columns form__item">
                  <label>
                    标题
                    <input className="textbox" value={title} onChange={(event) => setTitle(event.target.value)} placeholder="比赛标题" autoFocus={mode === 'create'} required />
                  </label>
                </div>
              </div>
              <div className="row">
                <div className="medium-4 columns form__item">
                  <label>
                    开始时间
                    <input className="textbox" type="datetime-local" value={beginAt} onChange={(event) => setBeginAt(event.target.value)} required />
                  </label>
                </div>
                <div className="medium-3 columns form__item">
                  <label>
                    时长（小时）
                    <input className="textbox" type="number" min="1" max="720" value={duration} onChange={(event) => setDuration(Number(event.target.value))} required />
                  </label>
                </div>
                <div className="medium-5 columns form__item">
                  <label className="checkbox">
                    <input type="checkbox" className="checkbox" checked={rated} onChange={(event) => setRated(event.target.checked)} />
                    Rated
                  </label>
                </div>
              </div>
              <div className="form__item">
                <label>
                  题目 docId
                  <input className="textbox" value={pids} onChange={(event) => setPids(event.target.value)} placeholder="例如：9001, 9002" required />
                </label>
                <p className="help-text">按顺序填写题目的 docId，用逗号分隔。</p>
              </div>
              <div className="form__item">
                <label>
                  比赛说明 Markdown
                  <textarea className="textbox auto-resize" value={content} onChange={(event) => setContent(event.target.value)} style={{ minHeight: 360 }} />
                </label>
              </div>
              {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
            </div>
            <div className="section__body">
              <button type="submit" className="rounded primary button" disabled={saving}>
                {saving ? '保存中...' : mode === 'create' ? '创建' : '更新'}
              </button>
              <Link className="rounded button" to={mode === 'create' ? '/contest' : `/contest/${encodeURIComponent(tid)}`}>取消</Link>
            </div>
          </form>
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__header"><h1 className="section__title">Markdown 提示</h1></div>
          <div className="section__body typo">
            <dl className="large horizontal">
              <dt>标题</dt><dd><code>## 标题</code></dd>
              <dt>列表</dt><dd><code>- 项目</code></dd>
              <dt>代码</dt><dd><code>```cpp</code></dd>
              <dt>链接</dt><dd><code>[文字](url)</code></dd>
            </dl>
          </div>
        </div>
      </div>
    </div>
  )
}
