// Page markup is rendered from the pinned Hydro ui-default templates
// (homework_files.html and training_files.html). Keep this file limited to data
// mapping and the file interactions those templates expect; do not re-create
// the DOM in JSX.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  attendHomework, deleteActivityFiles, getActivityFiles, getHomework, getTraining, getUsersByIDs,
  uploadActivityFiles,
  type ActivityFile, type ActivityFileResource, type Contest, type TrainingDetail, type UserChoice,
} from '../api.ts'
import { avatarUrl } from '../upstream/avatar.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'

type Kind = 'homework' | 'training'

type PageData = {
  docId: string
  owner: number
  tdoc: Record<string, unknown>
  tsdoc: Record<string, unknown>
}

const config: Record<Kind, { template: string; resource: ActivityFileResource; pageName: string }> = {
  homework: { template: 'homework_files.html', resource: 'homeworks', pageName: 'homework_files' },
  training: { template: 'training_files.html', resource: 'trainings', pageName: 'training_files' },
}

function toUdoc(item: UserChoice) {
  return { _id: item.id, uname: item.username, displayName: item.displayName ?? '', avatar: item.avatar ?? '' }
}

function homeworkPageData(homework: Contest): PageData {
  return {
    docId: homework.docId,
    owner: homework.owner,
    tdoc: {
      ...homework,
      _id: homework.docId,
      beginAt: new Date(homework.beginAt),
      endAt: new Date(homework.endAt),
      penaltySince: new Date(homework.penaltySince ?? homework.endAt),
      lockAt: homework.lockAt ? new Date(homework.lockAt) : null,
      pids: homework.pids ?? [],
      domainId: 'system',
    },
    tsdoc: { attend: Boolean(homework.attended) },
  }
}

function trainingPageData(training: TrainingDetail): PageData {
  const dag = training.sections.map((section) => ({
    _id: section.id,
    title: section.title,
    content: section.content ?? '',
    requireNids: section.requireNids,
    pids: section.problems.map((problem) => problem.docId),
  }))
  return {
    docId: training.docId,
    owner: training.owner,
    tdoc: { ...training, _id: training.id, dag },
    tsdoc: { enroll: training.enrolled, done: training.completed },
  }
}

export function ActivityFilesPage({ kind }: { kind: Kind }) {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const page = config[kind]
  const [data, setData] = useState<PageData | null>(null)
  const [files, setFiles] = useState<ActivityFile[]>([])
  const [owner, setOwner] = useState<UserChoice | null>(null)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  const load = useCallback(async () => {
    const [next, list] = await Promise.all([
      kind === 'homework' ? getHomework(tid).then(homeworkPageData) : getTraining(tid).then(trainingPageData),
      getActivityFiles(page.resource, tid),
    ])
    setData(next)
    setFiles(list)
    const [udoc] = await getUsersByIDs([next.owner]).catch(() => [])
    setOwner(udoc ?? null)
  }, [kind, page.resource, tid])

  useEffect(() => {
    setData(null)
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '文件加载失败'))
  }, [load])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const listen = <K extends keyof HTMLElementEventMap>(
      element: Element,
      type: K,
      listener: (event: HTMLElementEventMap[K]) => void,
    ) => {
      element.addEventListener(type, listener as EventListener)
      cleanups.push(() => element.removeEventListener(type, listener as EventListener))
    }

    // Mirrors the select-all handling in the upstream files.page.tsx.
    root.querySelectorAll<HTMLInputElement>('input[data-checkbox-toggle]').forEach((input) => {
      listen(input, 'change', () => {
        const group = input.getAttribute('data-checkbox-toggle') ?? 'files'
        root.querySelectorAll<HTMLInputElement>(`input[data-checkbox-group="${group}"]`).forEach((checkbox) => {
          checkbox.checked = input.checked
        })
      })
    })

    root.querySelectorAll<HTMLButtonElement>('[name="upload_file"]').forEach((button) => {
      listen(button, 'click', () => {
        const picker = document.createElement('input')
        picker.type = 'file'
        picker.multiple = true
        picker.onchange = async () => {
          const chosen = Array.from(picker.files ?? [])
          if (!chosen.length) return
          setActionError('')
          try {
            setFiles(await uploadActivityFiles(page.resource, tid, chosen))
          } catch (err) {
            setActionError(err instanceof Error ? err.message : '文件上传失败')
          }
        }
        picker.click()
      })
    })

    root.querySelectorAll<HTMLButtonElement>('[name="remove_selected"]').forEach((button) => {
      listen(button, 'click', async () => {
        const names = Array.from(root.querySelectorAll<HTMLInputElement>('input[data-checkbox-group]:checked'))
          .map((checkbox) => checkbox.closest('tr')?.getAttribute('data-filename') ?? '')
          .filter(Boolean)
        if (!names.length) {
          setActionError('请至少选择一个文件。')
          return
        }
        if (!window.confirm(`确认删除选中的 ${names.length} 个文件？`)) return
        setActionError('')
        try {
          await deleteActivityFiles(page.resource, tid, names)
          setFiles(await getActivityFiles(page.resource, tid))
        } catch (err) {
          setActionError(err instanceof Error ? err.message : '文件删除失败')
        }
      })
    })

    // Drag and drop onto the file table, matching the upstream page script.
    root.querySelectorAll<HTMLElement>('.files').forEach((zone) => {
      listen(zone, 'dragover', (event) => event.preventDefault())
      listen(zone, 'drop', (event) => {
        event.preventDefault()
        const dropped = Array.from(event.dataTransfer?.files ?? [])
        if (!dropped.length) return
        setActionError('')
        uploadActivityFiles(page.resource, tid, dropped)
          .then(setFiles)
          .catch((err: unknown) => setActionError(err instanceof Error ? err.message : '文件上传失败'))
      })
    })

    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }

    return () => cleanups.forEach((cleanup) => cleanup())
  }, [page.resource, tid])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    if (new FormData(form).get('operation') !== 'attend') return
    setActionError('')
    try {
      await attendHomework(tid)
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '认领失败')
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? data?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'homework_detail': return `/homework/${encodeURIComponent(id)}`
      case 'homework_edit': return `/homework/${encodeURIComponent(id)}/edit`
      case 'homework_files': return `/homework/${encodeURIComponent(id)}/file`
      case 'homework_scoreboard': return `/homework/${encodeURIComponent(id)}/scoreboard`
      case 'training_detail': return `/training/${encodeURIComponent(id)}`
      case 'training_edit': return `/training/${encodeURIComponent(id)}/edit`
      case 'training_files': return `/training/${encodeURIComponent(id)}/file`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'wiki':
      case 'wiki_help':
        return '/discuss'
      case 'homework_code':
        return `/api/v1/homeworks/${encodeURIComponent(id)}/code`
      default: return '/'
    }
  }, [data?.docId, tid])

  const context = useMemo(() => {
    if (!data) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const canEdit = Boolean(user && (user.id === data.owner || isAdmin))
    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (permission === CONTEST_PERMS.PERM_ATTEND_HOMEWORK) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_SCOREBOARD) return true
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD) return canEdit
          if (permission === CONTEST_PERMS.PERM_EDIT_HOMEWORK) return canEdit
          if (permission === CONTEST_PERMS.PERM_EDIT_TRAINING) return canEdit
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
      },
      args: { domainId: 'system' },
      request: {
        query: {},
        path: `/${kind}/${data.docId}/file`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    return {
      page_name: page.pageName,
      tdoc: data.tdoc,
      tsdoc: data.tsdoc,
      files: files.map((file) => ({ name: file.name, size: file.size })),
      udoc: owner ? toUdoc(owner) : null,
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [data, files, kind, owner, page.pageName, user])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!data) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template={page.template}
        context={context}
        url={buildUrl}
        urlForFile={(name) => `/api/v1/${page.resource}/${encodeURIComponent(tid)}/files/${encodeURIComponent(name)}`}
        onReady={handleReady}
      />
    </>
  )
}
