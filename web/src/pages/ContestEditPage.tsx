// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and event adapters; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  createContest,
  getActivityFiles,
  getContestEditor,
  updateContest,
  uploadActivityFiles,
  type ActivityFile,
  type Contest,
  type ContestEditInput,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { alertDialog } from '../components/dialogs.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'
import { legacyMoment } from '../upstream/moment.ts'
import { renderHydroTemplate } from '../upstream/render.ts'
import { currentTimeZone, parseZonedDateTime, zonedDateParts } from '../upstream/timezone.ts'

const rules = {
  acm: 'XCPC / ACM',
  codeforces: 'Codeforces',
  oi: 'OI',
  ioi: 'IOI',
  strictioi: 'Strict IOI',
  ledo: 'Ledo',
}

function nextSlot(timeZone = currentTimeZone()) {
  const now = zonedDateParts(new Date(), timeZone)
  const rounded = Math.ceil((now.hour * 60 + now.minute + 1) / 15) * 15
  const dayShift = Math.floor(rounded / (24 * 60))
  const minuteOfDay = rounded % (24 * 60)
  const day = new Date(Date.UTC(now.year, now.month - 1, now.day + dayShift))
  const pad = (value: number) => String(value).padStart(2, '0')
  const date = `${day.getUTCFullYear()}-${pad(day.getUTCMonth() + 1)}-${pad(day.getUTCDate())}`
  const time = `${pad(Math.floor(minuteOfDay / 60))}:${pad(minuteOfDay % 60)}`
  return parseZonedDateTime(date, time, timeZone)
}

function emptyContest(owner = 0, timeZone = currentTimeZone()): Contest {
  const begin = nextSlot(timeZone)
  return {
    id: '',
    docId: '',
    title: '',
    content: '',
    beginAt: begin.toISOString(),
    endAt: new Date(begin.getTime() + 2 * 3600000).toISOString(),
    rule: 'acm',
    owner,
    attend: 0,
    pids: [],
    duration: 2,
    rated: true,
    maintainer: [],
    assign: [],
    langs: [],
    autoHide: true,
    allowViewCode: true,
    allowPrint: false,
    allowTeam: false,
    keepScoreboardHidden: false,
    invitationCode: '',
  }
}

function splitList(value: string) {
  return value.replace(/，/g, ',').split(',').map((item) => item.trim()).filter(Boolean)
}

function listOfNumbers(value: string) {
  return splitList(value).map((item) => Number.parseInt(item, 10)).filter((item) => item > 0)
}

export function ContestEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { tid = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const { user } = useAuth()
  const draft = (location.state as { draft?: Partial<Contest> } | null)?.draft
  const [item, setItem] = useState<Contest | null>(() => (
    mode === 'create' ? { ...emptyContest(user?.id ?? 0, user?.timeZone), ...draft } : null
  ))
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const files = useRef<ActivityFile[]>([])
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    Promise.all([getContestEditor(tid), getActivityFiles('contests', tid)])
      .then(([contest, contestFiles]) => {
        files.current = contestFiles
        setItem(contest)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛加载失败'))
      .finally(() => setLoading(false))
  }, [mode, tid])

  const refreshFiles = useCallback(async (root: HTMLElement, nextFiles?: ActivityFile[]) => {
    if (mode !== 'edit' || !tid) return
    const resolved = nextFiles ?? await getActivityFiles('contests', tid)
    files.current = resolved
    const current = root.querySelector('.files')
    if (!current) return
    const rendered = renderHydroTemplate('partials/files.html', {
      context: { files: resolved, sidebar: true, filetype: 'public' },
      urlForFile: (name) => `/api/v1/contests/${encodeURIComponent(tid)}/files/${encodeURIComponent(name)}`,
    })
    const holder = document.createElement('div')
    holder.innerHTML = rendered
    current.replaceWith(holder.firstElementChild!)
  }, [mode, tid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    const stopHydration = hydrateHydroEditForm(root, {
      resource: mode === 'edit' ? 'contests' : undefined,
      tid: mode === 'edit' ? tid : undefined,
      autocomplete: { problems: 'pids', users: 'maintainer', assign: 'assign', langs: 'langs' },
      contestSwitches: true,
      dateTime: true,
      onUploaded: (uploaded) => void refreshFiles(root, uploaded),
    })
    const upload = root.querySelector<HTMLButtonElement>('[name="upload_file"]')
    const onClick = () => {
      const picker = document.createElement('input')
      picker.type = 'file'
      picker.multiple = true
      picker.onchange = async () => {
        if (!picker.files?.length || !tid) return
        try {
          const uploaded = await uploadActivityFiles('contests', tid, Array.from(picker.files!))
          await refreshFiles(root, uploaded)
        } catch (err) {
          await alertDialog(err instanceof Error ? err.message : '文件上传失败')
        }
      }
      picker.click()
    }
    upload?.addEventListener('click', onClick)
    return () => {
      form?.removeEventListener('submit', onSubmit)
      upload?.removeEventListener('click', onClick)
      stopHydration()
    }
  }, [mode, refreshFiles, tid])

  const context = useMemo(() => {
    if (!item) return null
    const begin = new Date(item.beginAt)
    const end = new Date(item.endAt)
    const duration = Math.max(0, Math.round(((end.getTime() - begin.getTime()) / 3600000) * 100) / 100)
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      page_name: mode === 'edit' ? 'contest_edit' : 'contest_create',
      rules,
      tdoc: mode === 'edit' ? {
        ...item,
        _code: item.invitationCode ?? '',
        assign: item.assign ?? [],
        langs: (item.langs ?? []).join(','),
        maintainer: (item.maintainer ?? []).join(','),
        endAt: end.getTime(),
        lockAt: item.lockAt ? new Date(item.lockAt).getTime() : 0,
        files: files.current,
      } : null,
      beginAt: legacyMoment(begin),
      duration,
      pids: (item.pids ?? []).join(','),
      handler: {
        user: {
          // Delete is hidden until the Go API exposes resource deletion.
          hasPerm: (permission: number) => permission === 1 && isAdmin,
          hasPriv: () => isAdmin,
          own: () => false,
        },
      },
      perm: {
        PERM_EDIT_PROBLEM: 1,
        PERM_EDIT_CONTEST: 2,
      },
      UserContext: user,
    }
  }, [item, mode, user])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const submitter = event.submitter as HTMLButtonElement | null
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const text = (name: string) => String(data.get(name) ?? '')
    const begin = parseZonedDateTime(text('beginAtDate'), text('beginAtTime') || '00:00')
    const duration = Number(text('duration'))
    if (Number.isNaN(begin.getTime()) || !Number.isFinite(duration) || duration <= 0) {
      setError('开始时间或持续时间不正确')
      return
    }
    const end = new Date(begin.getTime() + duration * 3600000)
    const pids = listOfNumbers(text('pids'))
    if (!pids.length) {
      setError('至少需要一道题目')
      return
    }
    const permission = text('permission')
    const lockMinutes = Number(text('lock') || 0)
    if (!Number.isFinite(lockMinutes) || lockMinutes < 0) {
      setError('封榜时间不正确')
      return
    }
    const input: ContestEditInput = {
      title: text('title').trim(),
      content: text('content'),
      beginAt: begin.toISOString(),
      endAt: end.toISOString(),
      rule: text('rule'),
      pids,
      duration,
      rated: data.get('rated') !== null,
      unlocked: false,
      maintainer: listOfNumbers(text('maintainer')),
      assign: permission === 'assign' ? splitList(text('assign')) : [],
      invitationCode: permission === 'invite' ? text('code') : '',
      langs: splitList(text('langs')),
      ...(data.has('autoHide') ? { autoHide: data.get('autoHide') !== null } : {}),
      ...(data.has('allowViewCode') ? { allowViewCode: data.get('allowViewCode') !== null } : {}),
      ...(data.has('allowPrint') ? { allowPrint: data.get('allowPrint') !== null } : {}),
      ...(data.has('allowTeam') ? { allowTeam: data.get('allowTeam') !== null } : {}),
      ...(data.has('keepScoreboardHidden') ? { keepScoreboardHidden: data.get('keepScoreboardHidden') !== null } : {}),
      ...(lockMinutes > 0 ? { lockAt: new Date(end.getTime() - lockMinutes * 60000).toISOString() } : {}),
    }
    if (submitter?.formAction && mode === 'edit' && submitter.formAction.endsWith('/contest/new')) {
      navigate('/contest/new', { state: { draft: input } })
      return
    }
    setSaving(true)
    setError('')
    try {
      const saved = mode === 'create' ? await createContest(input) : await updateContest(tid, input)
      navigate(`/contest/${saved.docId}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '比赛保存失败')
    } finally {
      setSaving(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (mode === 'create' && user && !isAdmin) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有创建比赛的权限</div></div></div>
  }

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (!item || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '比赛不存在'}</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {saving ? <div className="row"><div className="columns"><blockquote className="note"><p>保存中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_edit.html"
        context={context}
        urlForFile={(name) => `/api/v1/contests/${encodeURIComponent(tid)}/files/${encodeURIComponent(name)}`}
        onReady={handleReady}
      />
    </>
  )
}
