import { useEffect, useState, type FormEvent } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  createContest,
  getContestEditor,
  updateContest,
  type ContestEditInput,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import {
  FormCheckbox,
  FormSelect,
  FormText,
  FormTextarea,
  MarkdownHint,
} from '../components/LegacyForm.tsx'
import { dateParts, formatEndTime, parseDateTime } from '../legacy-date.ts'

const ruleNames: Record<string, string> = {
  acm: 'XCPC / ACM',
  oi: 'OI',
  ioi: 'IOI',
  strictioi: 'Strict IOI',
  ledo: 'Ledo',
}

type ContestDraft = {
  title: string
  content: string
  rule: string
  beginDate: string
  beginTime: string
  duration: string
  pids: string
  rated: boolean
  maintainer: string
  permission: string
  assign: string
  invitationCode: string
  langs: string
  autoHide: boolean
  allowViewCode: boolean
  allowPrint: boolean
  allowTeam: boolean
  keepScoreboardHidden: boolean
  lock: string
}

function nextHour() {
  const date = new Date()
  date.setMinutes(0, 0, 0)
  date.setHours(date.getHours() + 1)
  return date
}

function defaultDraft(): ContestDraft {
  const begin = dateParts(nextHour())
  return {
    title: '',
    content: '',
    rule: 'acm',
    beginDate: begin.date,
    beginTime: begin.time,
    duration: '3',
    pids: '',
    rated: true,
    maintainer: '',
    permission: 'public',
    assign: '',
    invitationCode: '',
    langs: '',
    autoHide: true,
    allowViewCode: true,
    allowPrint: false,
    allowTeam: false,
    keepScoreboardHidden: false,
    lock: '',
  }
}

function splitList(value: string) {
  return value.replace(/，/g, ',').split(',').map((item) => item.trim()).filter(Boolean)
}

export function ContestEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { tid = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const { user } = useAuth()
  const draft = (location.state as { draft?: Partial<ContestDraft> } | null)?.draft
  const [form, setForm] = useState<ContestDraft>(() => ({ ...defaultDraft(), ...draft }))
  const [loading, setLoading] = useState(mode === 'edit')
  const [ownerID, setOwnerID] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    getContestEditor(tid)
      .then((item) => {
        setOwnerID(item.owner)
        const begin = dateParts(item.beginAt)
        setForm({
          title: item.title,
          content: item.content || '',
          rule: item.rule,
          beginDate: begin.date,
          beginTime: begin.time,
          duration: String(item.duration || Math.max(1, Math.round((new Date(item.endAt).getTime() - new Date(item.beginAt).getTime()) / 3600000))),
          pids: (item.pids ?? []).join(','),
          rated: Boolean(item.rated),
          maintainer: (item.maintainer ?? []).join(', '),
          permission: item.assign?.length ? 'assign' : (item.invitationCode ? 'invite' : 'public'),
          assign: (item.assign ?? []).join(', '),
          invitationCode: item.invitationCode || '',
          langs: (item.langs ?? []).join(', '),
          autoHide: item.autoHide ?? true,
          allowViewCode: item.allowViewCode ?? true,
          allowPrint: Boolean(item.allowPrint),
          allowTeam: Boolean(item.allowTeam),
          keepScoreboardHidden: Boolean(item.keepScoreboardHidden),
          lock: item.lockAt
            ? String(Math.max(0, Math.round((new Date(item.endAt).getTime() - new Date(item.lockAt).getTime()) / 60000)))
            : '',
        })
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
      .finally(() => setLoading(false))
  }, [mode, tid])

  function updateField<K extends keyof ContestDraft>(key: K, value: ContestDraft[K]) {
    setForm((current) => ({ ...current, [key]: value }))
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const begin = parseDateTime(form.beginDate, form.beginTime)
    const duration = Number(form.duration)
    if (!begin || !Number.isFinite(duration) || duration <= 0) {
      setError('开始时间或持续时间不正确')
      return
    }
    const problemIDs = splitList(form.pids).map((value) => Number.parseInt(value, 10)).filter((value) => value > 0)
    if (!problemIDs.length) {
      setError('至少需要一道题目')
      return
    }
    const maintainer = splitList(form.maintainer).map((value) => Number.parseInt(value, 10)).filter((value) => value > 0)
    const lockMinutes = form.lock.trim() ? Number(form.lock) : 0
    if (form.lock.trim() && (!Number.isFinite(lockMinutes) || lockMinutes < 0)) {
      setError('封榜时间不正确')
      return
    }
    const end = new Date(begin.getTime() + duration * 3600000)
    const input: ContestEditInput = {
      title: form.title.trim(),
      content: form.content,
      beginAt: begin.toISOString(),
      endAt: end.toISOString(),
      rule: form.rule,
      pids: problemIDs,
      duration,
      rated: form.rated,
      unlocked: false,
      maintainer,
      assign: form.permission === 'assign' ? splitList(form.assign) : [],
      invitationCode: form.permission === 'invite' ? form.invitationCode : '',
      langs: splitList(form.langs),
      autoHide: form.autoHide,
      allowViewCode: form.allowViewCode,
      allowPrint: form.allowPrint,
      allowTeam: form.allowTeam,
      keepScoreboardHidden: form.keepScoreboardHidden,
      ...(lockMinutes > 0 ? { lockAt: new Date(end.getTime() - lockMinutes * 60000).toISOString() } : {}),
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

  function cloneContest() {
    navigate('/contest/new', { state: { draft: form } })
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (error && mode === 'edit' && !form.title) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (user && !isAdmin && (mode === 'create' || ownerID !== user.id)) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有编辑这个比赛的权限</div></div></div>
  }

  const begin = parseDateTime(form.beginDate, form.beginTime)
  const duration = Number(form.duration)
  const endText = begin && Number.isFinite(duration) && duration > 0 ? formatEndTime(begin, duration) : ''

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <form method="post" onSubmit={handleSubmit}>
            <div className="section__header">
              <h2 className="section__title">基础信息</h2>
            </div>
            <div className="section__body">
              {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
              <div className="row">
                <FormSelect
                  columns={3}
                  row={false}
                  label="规则"
                  name="rule"
                  value={form.rule}
                  onChange={(event) => updateField('rule', event.target.value)}
                >
                  {Object.entries(ruleNames).map(([key, label]) => <option value={key} key={key}>{label}</option>)}
                </FormSelect>
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
              </div>
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
                />
                <FormText
                  columns={3}
                  row={false}
                  label="开始时间"
                  name="beginAtTime"
                  placeholder="HH:MM"
                  value={form.beginTime}
                  time
                  onChange={(event) => updateField('beginTime', event.target.value)}
                />
                <FormText
                  columns={2}
                  row={false}
                  label="持续时间 (小时)"
                  name="duration"
                  value={form.duration}
                  onChange={(event) => updateField('duration', event.target.value)}
                />
                <FormText
                  columns={4}
                  row={false}
                  lastColumn
                  label="结束时间"
                  name="endAt"
                  placeholder="YYYY-mm-dd HH:MM"
                  value={endText}
                  disabled
                />
              </div>
              <FormText
                columns={12}
                label="题目"
                name="pids"
                value={form.pids}
                placeholder="用逗号分隔"
                onChange={(event) => updateField('pids', event.target.value)}
              />
              <FormTextarea
                label="说明"
                name="content"
                value={form.content}
                markdown
                onChange={(event) => updateField('content', event.target.value)}
              />
            </div>
            <div style={{ paddingTop: 0 }} className="section__header">
              <h2 className="section__title">权限控制</h2>
            </div>
            <div className="section__body">
              <FormText
                columns={12}
                label="比赛管理员"
                name="maintainer"
                helpText="允许管理此比赛"
                value={form.maintainer}
                onChange={(event) => updateField('maintainer', event.target.value)}
              />
              <div className="row">
                <FormSelect
                  columns={3}
                  row={false}
                  label="权限控制"
                  name="permission"
                  value={form.permission}
                  onChange={(event) => updateField('permission', event.target.value)}
                >
                  <option value="public">公开</option>
                  <option value="invite">需要邀请码</option>
                  <option value="assign">指定用户或分组</option>
                </FormSelect>
                <div style={{ display: form.permission === 'assign' ? undefined : 'none' }} data-perm="assign">
                  <FormText
                    columns={9}
                    row={false}
                    label="分配给"
                    name="assign"
                    placeholder="分组 / 用户 ID"
                    value={form.assign}
                    onChange={(event) => updateField('assign', event.target.value)}
                  />
                </div>
                <div style={{ display: form.permission === 'invite' ? undefined : 'none' }} data-perm="invite">
                  <FormText
                    columns={9}
                    row={false}
                    label="邀请码"
                    name="code"
                    placeholder="（留空表示无）"
                    value={form.invitationCode}
                    onChange={(event) => updateField('invitationCode', event.target.value)}
                  />
                </div>
              </div>
            </div>
            <div style={{ paddingTop: 0 }} className="section__header">
              <h2 className="section__title">比赛设置</h2>
            </div>
            <div className="section__body">
              <FormText
                columns={12}
                label="提交语言限制"
                name="langs"
                value={form.langs}
                placeholder="用逗号分隔"
                onChange={(event) => updateField('langs', event.target.value)}
              />
              <div className="row">
                <FormCheckbox
                  columns={2}
                  row={false}
                  label="计入排名"
                  name="rated"
                  checked={form.rated}
                  placeholder="是否计入排名"
                  onChange={(event) => updateField('rated', event.target.checked)}
                />
                <FormCheckbox
                  columns={3}
                  row={false}
                  label="自动隐藏和显示"
                  name="autoHide"
                  checked={form.autoHide}
                  placeholder="在比赛过程中自动隐藏题目"
                  onChange={(event) => updateField('autoHide', event.target.checked)}
                />
                <FormCheckbox
                  columns={3}
                  row={false}
                  label="允许查看代码"
                  name="allowViewCode"
                  checked={form.allowViewCode}
                  placeholder="允许用户赛后查看其他人的代码"
                  onChange={(event) => updateField('allowViewCode', event.target.checked)}
                />
                <FormCheckbox
                  columns={4}
                  row={false}
                  label="允许打印"
                  name="allowPrint"
                  checked={form.allowPrint}
                  placeholder="允许用户在比赛期间打印"
                  onChange={(event) => updateField('allowPrint', event.target.checked)}
                />
                <FormCheckbox
                  columns={4}
                  row={false}
                  label="允许组队"
                  name="allowTeam"
                  checked={form.allowTeam}
                  placeholder="允许组队参赛"
                  helpText="一旦启用，本次比赛的组队功能将无法关闭。"
                  disabled={form.allowTeam}
                  onChange={(event) => updateField('allowTeam', event.target.checked)}
                />
                <div style={{ display: 'none' }} className="contest-rule-settings contest-rule--oi contest-rule--strictioi">
                  <FormCheckbox
                    columns={4}
                    row={false}
                    label="保持成绩表隐藏"
                    name="keepScoreboardHidden"
                    checked={form.keepScoreboardHidden}
                    placeholder="即使比赛结束后也保持成绩表隐藏"
                    onChange={(event) => updateField('keepScoreboardHidden', event.target.checked)}
                  />
                </div>
                <div style={{ display: 'none' }} className="contest-rule-settings contest-rule--acm contest-rule--ioi">
                  <FormText
                    columns={4}
                    row={false}
                    label="封榜 (分钟)"
                    name="lock"
                    placeholder="（留空表示无）"
                    helpText="在赛事剩余 N 分钟时封榜"
                    value={form.lock}
                    onChange={(event) => updateField('lock', event.target.value)}
                  />
                </div>
                <div style={{ display: 'none' }} className="contest-rule-settings contest-rule--oi contest-rule--ioi contest-rule--ledo contest-rule--strictioi">
                  <FormText
                    columns={4}
                    row={false}
                    label="灵活时长"
                    name="contestDuration"
                    placeholder="（留空表示无）"
                    helpText="限制用户在 N 小时内完成比赛"
                    value=""
                    disabled
                  />
                </div>
              </div>
              <div className="row">
                <div className="columns">
                  <button type="submit" className="rounded primary button" disabled={saving}>
                    {saving ? '保存中...' : mode === 'create' ? '创建' : '更新'}
                  </button>
                  {mode === 'edit' ? (
                    <button type="button" className="rounded button" onClick={cloneContest}>
                      复制
                    </button>
                  ) : null}
                  <button type="button" className="rounded button" onClick={() => window.history.go(-1)}>
                    取消
                  </button>
                </div>
              </div>
            </div>
          </form>
        </div>
      </div>
      <div className="medium-3 columns">
        <MarkdownHint />
      </div>
    </div>
  )
}
