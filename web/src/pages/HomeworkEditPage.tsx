import { useEffect, useState, type FormEvent } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  createHomework,
  getHomeworkEditor,
  updateHomework,
  type ContestEditInput,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { FormAutocomplete, FormText, FormTextarea, MarkdownHint } from '../components/LegacyForm.tsx'
import { addDays, dateParts, parseDateTime } from '../legacy-date.ts'

const defaultPenaltyRules = `1: 0.9
3: 0.8
12: 0.75
9999: 0.5`

type HomeworkDraft = {
  title: string
  beginDate: string
  beginTime: string
  penaltyDate: string
  penaltyTime: string
  extensionDays: string
  assign: string
  maintainer: string
  penaltyRules: string
  pids: string
  content: string
  langs: string
}

function defaultDraft(): HomeworkDraft {
  const begin = new Date()
  begin.setDate(begin.getDate() + 1)
  begin.setHours(0, 0, 0, 0)
  const penalty = addDays(begin, 7)
  penalty.setHours(23, 59, 0, 0)
  const beginParts = dateParts(begin)
  const penaltyParts = dateParts(penalty)
  return {
    title: '',
    beginDate: beginParts.date,
    beginTime: beginParts.time,
    penaltyDate: penaltyParts.date,
    penaltyTime: penaltyParts.time,
    extensionDays: '1',
    assign: '',
    maintainer: '',
    penaltyRules: defaultPenaltyRules,
    pids: '',
    content: '',
    langs: '',
  }
}

function splitList(value: string) {
  return value.replace(/，/g, ',').split(',').map((item) => item.trim()).filter(Boolean)
}

function rulesToText(rules?: Record<string, number>) {
  if (!rules || !Object.keys(rules).length) return defaultPenaltyRules
  return Object.entries(rules)
    .sort(([left], [right]) => Number(left) - Number(right))
    .map(([hours, coefficient]) => `${hours}: ${coefficient}`)
    .join('\n')
}

function parsePenaltyRules(value: string): Record<string, number> | null {
  const rules: Record<string, number> = {}
  for (const rawLine of value.split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue
    const match = /^([^:]+):\s*(-?\d+(?:\.\d+)?)$/.exec(line)
    if (!match) return null
    const hours = Number(match[1])
    const coefficient = Number(match[2])
    if (!Number.isFinite(hours) || hours < 0 || !Number.isFinite(coefficient) || coefficient < 0) return null
    rules[String(hours)] = coefficient
  }
  return Object.keys(rules).length ? rules : null
}

export function HomeworkEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { tid = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const { user } = useAuth()
  const draft = (location.state as { draft?: Partial<HomeworkDraft> } | null)?.draft
  const [form, setForm] = useState<HomeworkDraft>(() => ({ ...defaultDraft(), ...draft }))
  const [loading, setLoading] = useState(mode === 'edit')
  const [ownerID, setOwnerID] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    getHomeworkEditor(tid)
      .then((item) => {
        setOwnerID(item.owner)
        const begin = dateParts(item.beginAt)
        const penalty = dateParts(item.penaltySince || item.endAt)
        const extensionDays = item.penaltySince
          ? Math.max(0, (new Date(item.endAt).getTime() - new Date(item.penaltySince).getTime()) / 86400000)
          : 0
        setForm({
          title: item.title,
          beginDate: begin.date,
          beginTime: begin.time,
          penaltyDate: penalty.date,
          penaltyTime: penalty.time,
          extensionDays: String(Math.round(extensionDays * 100) / 100),
          assign: (item.assign ?? []).join(', '),
          maintainer: (item.maintainer ?? []).join(', '),
          penaltyRules: rulesToText(item.penaltyRules),
          pids: (item.pids ?? []).join(','),
          content: item.content || '',
          langs: (item.langs ?? []).join(', '),
        })
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '作业加载失败'))
      .finally(() => setLoading(false))
  }, [mode, tid])

  function updateField<K extends keyof HomeworkDraft>(key: K, value: HomeworkDraft[K]) {
    setForm((current) => ({ ...current, [key]: value }))
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const begin = parseDateTime(form.beginDate, form.beginTime)
    const penalty = parseDateTime(form.penaltyDate, form.penaltyTime)
    const extensionDays = Number(form.extensionDays)
    if (!begin || !penalty || penalty <= begin) {
      setError('截止时间必须晚于开始时间')
      return
    }
    if (!Number.isFinite(extensionDays) || extensionDays < 0) {
      setError('延期天数不正确')
      return
    }
    const problemIDs = splitList(form.pids).map((value) => Number.parseInt(value, 10)).filter((value) => value > 0)
    if (!problemIDs.length) {
      setError('至少需要一道题目')
      return
    }
    const penaltyRules = parsePenaltyRules(form.penaltyRules)
    if (!penaltyRules) {
      setError('延期递交扣分规则格式不正确')
      return
    }
    const end = addDays(penalty, extensionDays)
    const input: ContestEditInput = {
      title: form.title.trim(),
      content: form.content,
      beginAt: begin.toISOString(),
      endAt: end.toISOString(),
      rule: 'homework',
      pids: problemIDs,
      duration: Math.max(1, Math.round((end.getTime() - begin.getTime()) / 3600000)),
      rated: false,
      unlocked: false,
      penaltySince: penalty.toISOString(),
      penaltyRules,
      maintainer: splitList(form.maintainer).map((value) => Number.parseInt(value, 10)).filter((value) => value > 0),
      assign: splitList(form.assign),
      langs: splitList(form.langs),
    }
    setSaving(true)
    setError('')
    try {
      const item = mode === 'create' ? await createHomework(input) : await updateHomework(tid, input)
      navigate(`/homework/${item.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '作业保存失败')
    } finally {
      setSaving(false)
    }
  }

  function cloneHomework() {
    navigate('/homework/new', { state: { draft: form } })
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (error && mode === 'edit' && !form.title) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (user && !isAdmin && (mode === 'create' || ownerID !== user.id)) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有编辑这个作业的权限</div></div></div>
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__body">
            <form method="post" onSubmit={handleSubmit}>
              {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
              <FormText
                columns={12}
                label="标题"
                name="title"
                placeholder="标题"
                value={form.title}
                autoFocus={mode === 'create'}
                onChange={(event) => updateField('title', event.target.value)}
              />
              <div className="row">
                <FormText
                  columns={3}
                  row={false}
                  label="开始日期"
                  name="beginAtDate"
                  placeholder="YYYY-mm-dd"
                  value={form.beginDate}
                  date
                  onChange={(event) => updateField('beginDate', event.target.value)}
                  onValueChange={(value) => updateField('beginDate', value)}
                />
                <FormText
                  columns={2}
                  row={false}
                  label="开始时间"
                  name="beginAtTime"
                  placeholder="HH:MM"
                  value={form.beginTime}
                  time
                  onChange={(event) => updateField('beginTime', event.target.value)}
                  onValueChange={(value) => updateField('beginTime', value)}
                />
                <FormText
                  columns={3}
                  row={false}
                  label="结束日期"
                  name="penaltySinceDate"
                  placeholder="YYYY-mm-dd"
                  value={form.penaltyDate}
                  date
                  onChange={(event) => updateField('penaltyDate', event.target.value)}
                  onValueChange={(value) => updateField('penaltyDate', value)}
                />
                <FormText
                  columns={2}
                  row={false}
                  label="结束时间"
                  name="penaltySinceTime"
                  placeholder="HH:MM"
                  value={form.penaltyTime}
                  time
                  onChange={(event) => updateField('penaltyTime', event.target.value)}
                  onValueChange={(value) => updateField('penaltyTime', value)}
                />
                <FormText
                  columns={2}
                  row={false}
                  lastColumn
                  label="最长延期 (日)"
                  name="extensionDays"
                  value={form.extensionDays}
                  onChange={(event) => updateField('extensionDays', event.target.value)}
                />
              </div>
              <div className="row">
                <FormAutocomplete
                  columns={6}
                  row={false}
                  kind="assign"
                  label="分配给"
                  name="assign"
                  placeholder="分组 / 用户 ID"
                  value={form.assign}
                  onChange={(value) => updateField('assign', value)}
                />
                <FormAutocomplete
                  columns={6}
                  row={false}
                  lastColumn
                  kind="users"
                  label="作业管理员"
                  name="maintainer"
                  helpText="允许管理此作业"
                  value={form.maintainer}
                  onChange={(value) => updateField('maintainer', value)}
                />
              </div>
              <div className="row">
                <div className="columns">
                  <label>
                    延期递交扣分规则
                    <textarea
                      name="penaltyRules"
                      className="monospace textbox"
                      data-yaml=""
                      value={form.penaltyRules}
                      onChange={(event) => updateField('penaltyRules', event.target.value)}
                    />
                  </label>
                </div>
              </div>
              <FormAutocomplete
                kind="problems"
                label="题目"
                name="pids"
                value={form.pids}
                placeholder="用逗号分隔"
                onChange={(value) => updateField('pids', value)}
              />
              <FormTextarea
                label="内容"
                name="content"
                value={form.content}
                markdown
                onValueChange={(value) => updateField('content', value)}
              />
              <FormAutocomplete
                columns={12}
                kind="langs"
                label="提交语言限制"
                name="langs"
                value={form.langs}
                placeholder="用逗号分隔"
                onChange={(value) => updateField('langs', value)}
              />
              <div className="row">
                <div className="columns">
                  <button name="operation" value="update" type="submit" className="rounded primary button" disabled={saving}>
                    {saving ? '保存中...' : mode === 'create' ? '创建' : '更新'}
                  </button>
                  {mode === 'edit' ? (
                    <button name="operation" value="update" type="button" className="rounded button" onClick={cloneHomework}>
                      复制
                    </button>
                  ) : null}
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
