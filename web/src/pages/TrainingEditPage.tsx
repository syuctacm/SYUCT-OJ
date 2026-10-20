import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  createTraining,
  getTrainingEditor,
  updateTraining,
  type TrainingEditInput,
  type TrainingEditNode,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { FormText, FormTextarea, MarkdownHint } from '../components/LegacyForm.tsx'

const defaultDAG = `[
  {
    "_id": 1,
    "title": "第一节",
    "requireNids": [],
    "pids": []
  }
]`

type TrainingDraft = {
  title: string
  content: string
  description: string
  pin: string
  dag: string
}

function defaultDraft(): TrainingDraft {
  return {
    title: '',
    content: '',
    description: '',
    pin: '0',
    dag: defaultDAG,
  }
}

export function TrainingEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { tid = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [form, setForm] = useState<TrainingDraft>(defaultDraft)
  const [ownerID, setOwnerID] = useState<number | null>(null)
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    getTrainingEditor(tid)
      .then((item) => {
        setOwnerID(item.owner)
        const dag = (item.dag ?? []).map((node) => ({
          _id: node.id,
          title: node.title,
          ...(node.content ? { content: node.content } : {}),
          requireNids: node.requireNids,
          pids: node.pids,
        }))
        setForm({
          title: item.title,
          content: item.content || '',
          description: item.description || '',
          pin: item.pin ? '1' : '0',
          dag: JSON.stringify(dag, null, 2),
        })
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
      .finally(() => setLoading(false))
  }, [mode, tid])

  function updateField<K extends keyof TrainingDraft>(key: K, value: TrainingDraft[K]) {
    setForm((current) => ({ ...current, [key]: value }))
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let dag: TrainingEditNode[]
    try {
      const parsed = JSON.parse(form.dag) as unknown
      if (!Array.isArray(parsed)) throw new Error('训练计划必须是 JSON 数组')
      dag = parsed as TrainingEditNode[]
    } catch (err) {
      setError(err instanceof Error ? err.message : '训练计划 JSON 格式不正确')
      return
    }
    const pin = Number(form.pin)
    const input: TrainingEditInput = {
      title: form.title.trim(),
      content: form.content,
      description: form.description,
      pin: Number.isFinite(pin) && pin > 0,
      dag,
    }
    setSaving(true)
    setError('')
    try {
      const item = mode === 'create' ? await createTraining(input) : await updateTraining(tid, input)
      navigate(`/training/${item.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '训练保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (error && mode === 'edit' && !form.title) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (user && !isAdmin && (mode === 'create' || ownerID !== user.id)) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有编辑这个训练的权限</div></div></div>
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__body">
            <form method="post" onSubmit={handleSubmit}>
              {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
              <div className="row">
                <FormText
                  columns={9}
                  row={false}
                  label="标题"
                  name="title"
                  placeholder="标题"
                  value={form.title}
                  autoFocus={mode === 'create'}
                  onChange={(event) => updateField('title', event.target.value)}
                />
                <FormText
                  columns={3}
                  row={false}
                  lastColumn
                  label="置顶"
                  name="pin"
                  placeholder="0 / 1"
                  value={form.pin}
                  onChange={(event) => updateField('pin', event.target.value)}
                />
              </div>
              <FormTextarea
                label="介绍"
                name="content"
                value={form.content}
                helpText="介绍不能超过 500 个字符，会显示在训练列表。"
                onChange={(event) => updateField('content', event.target.value)}
              />
              <FormTextarea
                label="说明"
                name="description"
                value={form.description}
                markdown
                onValueChange={(value) => updateField('description', value)}
              />
              <div className="row">
                <div className="columns">
                  <label>
                    训练计划
                    <textarea
                      data-json=""
                      name="dag"
                      className="textbox"
                      style={{ height: 500 }}
                      value={form.dag}
                      onChange={(event) => updateField('dag', event.target.value)}
                    />
                    <p className="help-text">JSON 数组；pids 可填题目 docId 或 PID，例如 P1001。</p>
                  </label>
                </div>
              </div>
              <div className="row">
                <div className="columns">
                  <button type="submit" className="rounded primary button" disabled={saving}>
                    {saving ? '保存中...' : mode === 'create' ? '创建' : '更新'}
                  </button>
                  <button type="button" className="rounded button" onClick={() => window.history.go(-1)}>
                    取消
                  </button>
                </div>
              </div>
            </form>
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <MarkdownHint />
      </div>
    </div>
  )
}
