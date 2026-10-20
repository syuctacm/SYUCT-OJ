import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  createProblem,
  getProblemEditor,
  updateProblem,
  type ProblemDetail,
  type ProblemEditInput,
} from '../api.ts'
import { ProblemSidebar } from '../components/ProblemSidebar.tsx'

const defaultContent = `## 题目描述

在这里填写题目描述。

## 输入格式

在这里填写输入格式。

## 输出格式

在这里填写输出格式。

## 样例输入

\`\`\`
\`\`\`

## 样例输出

\`\`\`
\`\`\`
`

const defaultConfig = `type: default
time: 1000ms
memory: 256m
`

export function ProblemEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { pid = '' } = useParams()
  const navigate = useNavigate()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [form, setForm] = useState<ProblemEditInput>({
    pid: '',
    title: '',
    content: defaultContent,
    tags: [],
    difficulty: 0,
    hidden: false,
    config: defaultConfig,
  })
  const [tagText, setTagText] = useState('')
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    getProblemEditor(pid)
      .then((data) => {
        setProblem(data.problem)
        setForm({
          pid: data.problem.pid,
          title: data.problem.title,
          content: data.problem.content || '',
          tags: data.problem.tags ?? [],
          difficulty: data.problem.difficulty ?? 0,
          hidden: Boolean(data.problem.hidden),
          config: data.config,
        })
        setTagText((data.problem.tags ?? []).join(', '))
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
      .finally(() => setLoading(false))
  }, [mode, pid])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaving(true)
    setError('')
    const input: ProblemEditInput = {
      ...form,
      tags: tagText.split(/[,，]/).map((value) => value.trim()).filter(Boolean),
    }
    try {
      const data = mode === 'create'
        ? await createProblem(input)
        : await updateProblem(pid, input)
      if (mode === 'create') navigate(`/p/${encodeURIComponent(data.problem.pid)}/files`, { replace: true })
      else navigate(`/p/${encodeURIComponent(data.problem.pid)}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '题目保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (error && mode === 'edit' && !problem) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__body">
            <form method="post" onSubmit={handleSubmit}>
              <div className="row">
                <div className="medium-3 columns">
                  <label>
                    PID
                    <br />
                    <input
                      name="pid"
                      placeholder="P1000"
                      value={form.pid}
                      onChange={(event) => setForm((current) => ({ ...current, pid: event.target.value }))}
                      className="textbox"
                      required
                    />
                  </label>
                </div>
                <div className="medium-7 columns">
                  <label>
                    标题
                    <input
                      name="title"
                      placeholder="题目标题"
                      value={form.title}
                      onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))}
                      className="textbox"
                      autoFocus={mode === 'create'}
                      required
                    />
                  </label>
                </div>
                <div className="medium-2 columns">
                  <label>
                    <br />
                    <label className="checkbox">
                      <input
                        type="checkbox"
                        name="hidden"
                        checked={form.hidden}
                        onChange={(event) => setForm((current) => ({ ...current, hidden: event.target.checked }))}
                        className="checkbox"
                      />
                      隐藏
                    </label>
                  </label>
                </div>
              </div>
              <div className="row">
                <div className="medium-9 columns form__item">
                  <label>
                    标签
                    <input
                      name="tag"
                      value={tagText}
                      onChange={(event) => setTagText(event.target.value)}
                      className="textbox"
                      placeholder="数学, 图论"
                    />
                  </label>
                  <p className="help-text">使用逗号分隔。</p>
                </div>
                <div className="medium-3 columns form__item">
                  <label>
                    难度
                    <input
                      name="difficulty"
                      type="number"
                      min="0"
                      max="10"
                      value={form.difficulty}
                      onChange={(event) => setForm((current) => ({ ...current, difficulty: Number(event.target.value) }))}
                      className="textbox"
                    />
                  </label>
                  <p className="help-text">范围 0-10。</p>
                </div>
              </div>
              <div className="form__item">
                <label>
                  题面 Markdown
                  <textarea
                    name="content"
                    value={form.content}
                    onChange={(event) => setForm((current) => ({ ...current, content: event.target.value }))}
                    className="textbox auto-resize"
                    style={{ minHeight: 500 }}
                  />
                </label>
              </div>
              {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
              <div className="row"><div className="columns">
                <button type="submit" className="rounded primary button" disabled={saving}>
                  {saving ? '保存中...' : mode === 'create' ? '创建' : '更新'}
                </button>
                <Link className="rounded button" to={mode === 'create' ? '/p' : `/p/${encodeURIComponent(problem?.pid || pid)}`}>取消</Link>
              </div></div>
            </form>
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        {mode === 'edit' && problem ? (
          <ProblemSidebar problem={problem} pageName="problem_edit" />
        ) : (
          <div className="section side">
            <div className="section__header"><h1 className="section__title">Markdown 提示</h1></div>
            <div className="section__body typo">
              <dl className="large horizontal">
                <dt>标题</dt><dd><code>## 标题</code></dd>
                <dt>代码</dt><dd><code>```cpp</code></dd>
                <dt>公式</dt><dd><code>$x^2$</code></dd>
                <dt>链接</dt><dd><code>[文字](url)</code></dd>
              </dl>
              <p><a href="https://commonmark.org/help/" target="_blank" rel="noreferrer"><span className="icon icon-link--external" /> CommonMark 语法</a></p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
