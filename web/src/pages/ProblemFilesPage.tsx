// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and event adapters; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteProblemAssets,
  downloadProblemAssetsArchive,
  getProblemAssets,
  renameProblemAssets,
  uploadProblemAssets,
  type ProblemAssetList,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog, createFileDialog, promptDialog } from '../components/dialogs.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

type AssetKind = 'testdata' | 'additional_file'

function configSummary(problem: ProblemAssetList['problem']) {
  return {
    type: problem.problemType || 'default',
    timeMin: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    timeMax: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    memoryMin: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
    memoryMax: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
  }
}

export function ProblemFilesPage() {
  const { pid = '' } = useParams()
  const { user } = useAuth()
  const [data, setData] = useState<ProblemAssetList | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')

  useEffect(() => {
    setError('')
    getProblemAssets(pid)
      .then(setData)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目文件加载失败'))
  }, [pid])

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

    // Testdata generation and selected-file batch tools still need dedicated APIs.
    root.querySelector('.generate')?.closest('.section')?.remove()
    root.querySelectorAll('[name="rename_selected"]')
      .forEach((element) => element.remove())

    const assetType = (kind: AssetKind) => kind === 'testdata' ? 'testdata' : 'additional'

    root.querySelectorAll<HTMLButtonElement>('[name="create_testdata"], [name="create_file"]').forEach((button) => {
      listen(button, 'click', async () => {
        const kind = (button.closest('[data-type]')?.getAttribute('data-type') as AssetKind | null) ?? 'testdata'
        const value = await createFileDialog()
        if (!value) return
        setBusy(`create-${kind}`)
        setError('')
        try {
          const file = new File([value.content], value.name, { type: 'text/plain' })
          setData(await uploadProblemAssets(pid, assetType(kind), [file]))
        } catch (err) {
          setError(err instanceof Error ? err.message : '文件创建失败')
        } finally {
          setBusy('')
        }
      })
    })

    root.querySelectorAll<HTMLElement>('[name="file_rename"]').forEach((button) => {
      listen(button, 'click', async () => {
        const row = button.closest<HTMLElement>('tr[data-filename]')
        const kind = (row?.closest('[data-type]')?.getAttribute('data-type') as AssetKind | null) ?? 'testdata'
        const oldName = row?.getAttribute('data-filename') ?? ''
        if (!oldName) return
        const newName = await promptDialog('输入新文件名', oldName)
        if (!newName || newName === oldName) return
        setBusy(`rename-${kind}`)
        setError('')
        try {
          setData(await renameProblemAssets(pid, assetType(kind), [oldName], [newName]))
        } catch (err) {
          setError(err instanceof Error ? err.message : '文件重命名失败')
        } finally {
          setBusy('')
        }
      })
    })

    root.querySelectorAll<HTMLElement>('[name="file_remove"]').forEach((button) => {
      listen(button, 'click', async () => {
        const row = button.closest<HTMLElement>('tr[data-filename]')
        const kind = (row?.closest('[data-type]')?.getAttribute('data-type') as AssetKind | null) ?? 'testdata'
        const name = row?.getAttribute('data-filename') ?? ''
        if (!name || !(await confirmDialog(`确认删除文件 ${name}？`))) return
        setBusy(`delete-${kind}`)
        setError('')
        try {
          setData(await deleteProblemAssets(pid, assetType(kind), [name]))
        } catch (err) {
          setError(err instanceof Error ? err.message : '文件删除失败')
        } finally {
          setBusy('')
        }
      })
    })

    root.querySelectorAll<HTMLButtonElement>('[name="upload_file"]').forEach((button) => {
      listen(button, 'click', () => {
        const kind = (button.closest('[data-type]')?.getAttribute('data-type') as AssetKind | null) ?? 'testdata'
        const picker = document.createElement('input')
        picker.type = 'file'
        picker.multiple = true
        picker.onchange = async () => {
          if (!picker.files?.length) return
          setBusy(`upload-${kind}`)
          setError('')
          try {
            const result = await uploadProblemAssets(pid, assetType(kind), Array.from(picker.files))
            setData(result)
          } catch (err) {
            setError(err instanceof Error ? err.message : '文件上传失败')
          } finally {
            setBusy('')
          }
        }
        picker.click()
      })
    })

    root.querySelectorAll<HTMLButtonElement>('[name="download_selected"]').forEach((button) => {
      listen(button, 'click', async () => {
        const kind = (button.closest('[data-type]')?.getAttribute('data-type') as AssetKind | null) ?? 'testdata'
        const names = Array.from(root.querySelectorAll<HTMLInputElement>(`input[data-checkbox-group="${kind}"]:checked`))
          .map((checkbox) => checkbox.closest('tr')?.getAttribute('data-filename') ?? '')
          .filter(Boolean)
        if (!names.length) {
          setError('请至少选择一个文件。')
          return
        }
        setBusy(`download-${kind}`)
        setError('')
        try {
          const blob = await downloadProblemAssetsArchive(pid, assetType(kind), names)
          const url = URL.createObjectURL(blob)
          const anchor = document.createElement('a')
          anchor.href = url
          anchor.download = `${pid}-${kind}.zip`
          document.body.append(anchor)
          anchor.click()
          anchor.remove()
          window.setTimeout(() => URL.revokeObjectURL(url), 1000)
        } catch (err) {
          setError(err instanceof Error ? err.message : '文件下载失败')
        } finally {
          setBusy('')
        }
      })
    })

    root.querySelectorAll<HTMLButtonElement>('[name="remove_selected"]').forEach((button) => {
      listen(button, 'click', async () => {
        const kind = (button.closest('[data-type]')?.getAttribute('data-type') as AssetKind | null) ?? 'testdata'
        const names = Array.from(root.querySelectorAll<HTMLInputElement>(`input[data-checkbox-group="${kind}"]:checked`))
          .map((checkbox) => checkbox.closest('tr')?.getAttribute('data-filename') ?? '')
          .filter(Boolean)
        if (!names.length || !(await confirmDialog(`确认删除选中的 ${names.length} 个文件？`))) return
        setBusy(`delete-${kind}`)
        setError('')
        try {
          const result = await deleteProblemAssets(pid, assetType(kind), names)
          setData(result)
        } catch (err) {
          setError(err instanceof Error ? err.message : '文件删除失败')
        } finally {
          setBusy('')
        }
      })
    })

    return () => cleanups.forEach((cleanup) => cleanup())
  }, [pid])

  const context = useMemo(() => {
    if (!data) return null
    const problem = data.problem
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      page_name: 'problem_files',
      pdoc: {
        ...problem,
        tag: problem.tags ?? [],
        config: configSummary(problem),
      },
      testdata: data.testdata,
      additional_file: data.additional,
      reference: false,
      udict: {},
      owner_udoc: problem.owner ? {
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
          hasPerm: (permission: number) => permission === 8 || (Boolean(user) && [1, 2, 3, 4].includes(permission)),
          hasPriv: () => Boolean(user),
          own: () => Boolean(user && (problem.owner === user.id || isAdmin)),
        },
        ctx: { setting: { get: () => '' } },
        request: { headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false },
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
      avatarUrl,
      model: {
        builtin: { LEVELS: {} },
        system: { get: (key: string) => key === 'server.login' },
        setting: { SETTINGS_BY_KEY: {} },
      },
      lib: { difficulty: () => 0 },
      utils: { getAlphabeticId: () => '' },
      UserContext: user,
    }
  }, [data, user])

  if (error && !data) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!data || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {busy ? <div className="row"><div className="columns"><blockquote className="note"><p>正在处理文件...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_files.html"
        context={context}
        url={(name, args = {}) => {
          const id = encodeURIComponent(String(args.pid ?? data.problem.pid ?? data.problem.docId))
          const query = (args.query ?? {}) as Record<string, unknown>
          if (name === 'problem_file_download') {
            const type = query.type === 'additional_file' ? 'additional' : String(query.type ?? 'testdata')
            return `/api/v1/problems/${encodeURIComponent(String(args.pid ?? data.problem.docId))}/assets/download?name=${encodeURIComponent(String(args.filename ?? ''))}&type=${encodeURIComponent(type)}`
          }
          const routes: Record<string, string> = {
            problem_main: query.q ? `/p?q=${encodeURIComponent(String(query.q))}` : '/p',
            problem_detail: `/p/${id}`,
            problem_submit: `/p/${id}/submit`,
            problem_files: `/p/${id}/files`,
            problem_solution: `/p/${id}/solution`,
            problem_config: `/p/${id}/config`,
            problem_edit: `/p/${id}/edit`,
            problem_statistics: `/p/${id}/statistics`,
            record_main: `/record${query.pid ? `?pid=${encodeURIComponent(String(query.pid))}` : ''}`,
            user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
          }
          return routes[name] ?? '/'
        }}
        onReady={handleReady}
      />
    </>
  )
}
