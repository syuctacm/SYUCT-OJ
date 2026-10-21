// Page markup is rendered from the pinned Hydro ui-default template
// (problem_config.html). The `#ProblemConfig` container is filled by
// ProblemConfigPanel; see that file for the remaining editor deviation.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { useParams } from 'react-router-dom'
import {
  getProblemAssets, uploadProblemAssets,
  type ProblemAssetList,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { ProblemConfigPanel } from '../components/ProblemConfigPanel.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

export function ProblemConfigPage() {
  const { pid = '' } = useParams()
  const { user } = useAuth()
  const [data, setData] = useState<ProblemAssetList | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setError('')
    getProblemAssets(pid)
      .then(setData)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目文件加载失败'))
  }, [pid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    // Batch download of the whole testdata set is not ported yet.
    root.querySelector('[name="testdata__download__all"]')?.remove()
    root.querySelectorAll<HTMLElement>('[name="testdata__upload"]').forEach((button) => {
      const onClick = () => {
        const picker = document.createElement('input')
        picker.type = 'file'
        picker.multiple = true
        picker.onchange = async () => {
          const chosen = Array.from(picker.files ?? [])
          if (!chosen.length) return
          setBusy(true)
          setError('')
          try {
            setData(await uploadProblemAssets(pid, 'testdata', chosen))
          } catch (err) {
            setError(err instanceof Error ? err.message : '文件上传失败')
          } finally {
            setBusy(false)
          }
        }
        picker.click()
      }
      button.addEventListener('click', onClick)
      cleanups.push(() => button.removeEventListener('click', onClick))
    })

    const container = root.querySelector<HTMLElement>('#ProblemConfig')
    if (container) {
      container.replaceChildren()
      const host = document.createElement('div')
      container.appendChild(host)
      const root_ = createRoot(host)
      root_.render(<ProblemConfigPanel pid={pid} />)
      cleanups.push(() => {
        host.remove()
        queueMicrotask(() => root_.unmount())
      })
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [pid])

  const context = useMemo(() => {
    if (!data) return null
    const problem = data.problem
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      page_name: 'problem_config',
      pdoc: {
        ...problem,
        tag: problem.tags ?? [],
        config: {
          type: problem.problemType || 'default',
          timeMin: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
          timeMax: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
          memoryMin: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
          memoryMax: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
        },
      },
      testdata: data.testdata,
      additional_file: data.additional,
      reference: false,
      can_edit: true,
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
          hasPerm: (permission: number) => Boolean(user) && [1, 2, 3, 4].includes(permission),
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

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = encodeURIComponent(String(args.pid ?? data?.problem.pid ?? data?.problem.docId ?? pid))
    const query = (args.query ?? {}) as Record<string, unknown>
    if (name === 'problem_file_download') {
      const type = query.type === 'additional_file' ? 'additional' : String(query.type ?? 'testdata')
      return `/api/v1/problems/${encodeURIComponent(String(args.pid ?? data?.problem.docId ?? pid))}/assets/download?name=${encodeURIComponent(String(args.filename ?? ''))}&type=${encodeURIComponent(type)}`
    }
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
    }
    return routes[name] ?? '/'
  }, [data, pid])

  if (error && !data) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!data || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {busy ? <div className="row"><div className="columns"><blockquote className="note"><p>正在上传测试数据...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_config.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
