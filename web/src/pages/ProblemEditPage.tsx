// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and event adapters; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import yaml from 'js-yaml'
import {
  createProblem,
  getProblemAssets,
  getProblemEditor,
  updateProblem,
  uploadProblemAssets,
  type ProblemAsset,
  type ProblemDetail,
  type ProblemEditInput,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'
import { renderHydroTemplate } from '../upstream/render.ts'

const defaultConfig = `type: default
time: 1000ms
memory: 256m
`

function parseConfig(raw: string) {
  try {
    const value = yaml.load(raw) as Record<string, unknown> | null
    if (value && typeof value === 'object') return value
  } catch {
    // The editor will surface the server-side validation error on save.
  }
  return {}
}

function configNumbers(raw: string) {
  const config = parseConfig(raw)
  const time = String(config.time ?? '1000').replace(/ms$/i, '')
  const memory = String(config.memory ?? '256').replace(/[a-z]+$/i, '')
  return {
    timeMin: time,
    timeMax: time,
    memoryMin: memory,
    memoryMax: memory,
  }
}

function splitTags(value: string) {
  return value.replace(/，/g, ',').split(',').map((tag) => tag.trim()).filter(Boolean)
}

export function ProblemEditPage({ mode }: { mode: 'create' | 'edit' }) {
  const { pid = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [config, setConfig] = useState(defaultConfig)
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const files = useRef<ProblemAsset[]>([])
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    if (mode !== 'edit') return
    setLoading(true)
    Promise.all([getProblemEditor(pid), getProblemAssets(pid)])
      .then(([editor, assets]) => {
        files.current = assets.additional
        setProblem(editor.problem)
        setConfig(editor.config)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
      .finally(() => setLoading(false))
  }, [mode, pid])

  const refreshFiles = useCallback((root: HTMLElement, nextFiles: ProblemAsset[]) => {
    files.current = nextFiles
    const current = root.querySelector('[data-type="additional_file"] .files')
    if (!current) return
    const rendered = renderHydroTemplate('partials/problem_files.html', {
      context: {
        pdoc: problem,
        additional_file: nextFiles,
        filetype: 'additional_file',
        sidebar: true,
        can_edit: false,
        handler: { user: { own: () => true, hasPriv: () => Boolean(user), hasPerm: () => true } },
      },
      url: (name, args = {}) => name === 'problem_file_download'
        ? `/api/v1/problems/${encodeURIComponent(pid)}/assets/download?name=${encodeURIComponent(String(args.filename ?? ''))}&type=additional`
        : '/',
    })
    const holder = document.createElement('div')
    holder.innerHTML = rendered
    current.replaceWith(holder.firstElementChild!)
  }, [pid, problem, user])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    // The Go API does not expose problem deletion yet; do not leave a dead control.
    root.querySelector('button[value="delete"]')?.remove()
    const stopHydration = hydrateHydroEditForm(root, {
      uploadProblem: mode === 'edit' ? async (selected) => {
        const result = await uploadProblemAssets(pid, 'additional', selected)
        return result.additional
      } : undefined,
      onProblemUploaded: (uploaded) => refreshFiles(root, uploaded as ProblemAsset[]),
    })
    const upload = root.querySelector<HTMLButtonElement>('[name="upload_file"]')
    const onClick = () => {
      if (mode !== 'edit') return
      const picker = document.createElement('input')
      picker.type = 'file'
      picker.multiple = true
      picker.onchange = async () => {
        if (!picker.files?.length) return
        try {
          const result = await uploadProblemAssets(pid, 'additional', Array.from(picker.files))
          refreshFiles(root, result.additional)
        } catch (err) {
          window.alert(err instanceof Error ? err.message : '文件上传失败')
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
  }, [mode, pid, refreshFiles])

  const context = useMemo(() => {
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const configData = configNumbers(config)
    const pdoc = problem ? {
      ...problem,
      tag: problem.tags ?? [],
      config: configData,
      docId: problem.docId,
    } : null
    return {
      page_name: mode === 'edit' ? 'problem_edit' : 'problem_create',
      pdoc,
      statementLangs: {},
      additional_file: files.current,
      udict: {},
      owner_udoc: problem?.owner ? {
        _id: problem.owner,
        uname: `UID ${problem.owner}`,
        avatar: '',
        hasPriv: () => false,
        hasPerm: () => false,
      } : null,
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => Boolean(user) && [1, 2, 3, 4].includes(permission),
          hasPriv: () => Boolean(user),
          own: () => Boolean(user && problem && (problem.owner === user.id || isAdmin)),
        },
        ctx: {
          setting: {
            get: (key: string) => key === 'problem.categories' ? '{}' : key === 'ui-default.enableScratchpad' ? '' : '',
          },
        },
      },
      perm: {
        PERM_EDIT_PROBLEM: 1,
        PERM_EDIT_PROBLEM_SELF: 2,
        PERM_SUBMIT_PROBLEM: 3,
        PERM_READ_PROBLEM_DATA: 4,
        PERM_VIEW_DISCUSSION: 5,
        PERM_CREATE_DISCUSSION: 6,
        PERM_REJUDGE_PROBLEM: 7,
        PERM_VIEW_PROBLEM_SOLUTION: 8,
      },
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_READ_PROBLEM_DATA: 2, PRIV_REGISTER_USER: 3 },
      STATUS: { STATUS_ACCEPTED: 1 },
      psdoc: { status: 1 },
      tdocs: [],
      ctdocs: [],
      htdocs: [],
      discussionCount: 0,
      solutionCount: 0,
      UiContext: { cdn_prefix: '/' },
      avatarUrl: (value?: string) => value || '/logo.png',
      model: {
        builtin: { LEVELS: {} },
        system: { get: (key: string) => key === 'server.login' ? true : false },
        setting: { SETTINGS_BY_KEY: {} },
      },
      lib: { difficulty: () => 0 },
      utils: { getAlphabeticId: () => '' },
      UserContext: user,
    }
  }, [config, files.current, mode, problem, user])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const input: ProblemEditInput = {
      pid: String(data.get('pid') ?? '').trim(),
      title: String(data.get('title') ?? '').trim(),
      content: String(data.get('content') ?? ''),
      tags: splitTags(String(data.get('tag') ?? '')),
      difficulty: Number(data.get('difficulty') || 0),
      hidden: data.get('hidden') !== null,
      config,
    }
    setSaving(true)
    setError('')
    try {
      const result = mode === 'create' ? await createProblem(input) : await updateProblem(pid, input)
      const target = `/p/${encodeURIComponent(result.problem.pid || String(result.problem.docId))}`
      navigate(mode === 'create' ? `${target}/files` : target, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '题目保存失败')
    } finally {
      setSaving(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  if (mode === 'create' && user && !isAdmin) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">没有创建题目的权限</div></div></div>
  }
  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }
  if (mode === 'edit' && (!problem || !context)) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error || '题目不存在'}</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {saving ? <div className="row"><div className="columns"><blockquote className="note"><p>保存中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_edit.html"
        context={context}
        url={(name, args = {}) => {
          const id = encodeURIComponent(String(args.pid ?? problem?.pid ?? problem?.docId ?? ''))
          const query = (args.query ?? {}) as Record<string, unknown>
          const routes: Record<string, string> = {
            problem_main: query.q ? `/p?q=${encodeURIComponent(String(query.q))}` : '/p',
            problem_detail: `/p/${id}`,
            problem_submit: `/p/${id}/submit`,
            problem_files: `/p/${id}/files`,
            problem_config: `/p/${id}/config`,
            problem_edit: `/p/${id}/edit`,
            problem_statistics: `/p/${id}`,
            record_main: `/record${query.pid ? `?pid=${encodeURIComponent(String(query.pid))}` : ''}`,
            user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
            discussion_node: '/discuss',
            problem_file_download: `/api/v1/problems/${encodeURIComponent(String(args.pid ?? problem?.docId ?? ''))}/assets/download?name=${encodeURIComponent(String(args.filename ?? ''))}&type=${encodeURIComponent(String(query.type ?? 'additional'))}`,
          }
          return routes[name] ?? '/'
        }}
        onReady={handleReady}
      />
    </>
  )
}
