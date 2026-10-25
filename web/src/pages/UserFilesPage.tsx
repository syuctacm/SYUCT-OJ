// Page markup is rendered from the pinned Hydro ui-default template
// (home_files.html). Keep this file limited to data mapping and the file
// interactions that partials/files.html expects.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  deleteUserFiles, getUserFiles, uploadUserFiles,
  type ActivityFile,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

const PRIV_USER_PROFILE = 1
const PRIV_CREATE_FILE = 2

export function UserFilesPage() {
  const { user, loading } = useAuth()
  const navigate = useNavigate()
  const [files, setFiles] = useState<ActivityFile[] | null>(null)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')

  useEffect(() => {
    if (!loading && !user) navigate('/login')
  }, [loading, navigate, user])

  useEffect(() => {
    if (!user) return
    setError('')
    getUserFiles()
      .then(setFiles)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '文件加载失败'))
  }, [user])

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
            setFiles(await uploadUserFiles(chosen))
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
        if (!(await confirmDialog(`确认删除选中的 ${names.length} 个文件？`))) return
        setActionError('')
        try {
          await deleteUserFiles(names)
          setFiles(await getUserFiles())
        } catch (err) {
          setActionError(err instanceof Error ? err.message : '文件删除失败')
        }
      })
    })

    root.querySelectorAll<HTMLElement>('.files').forEach((zone) => {
      listen(zone, 'dragover', (event) => event.preventDefault())
      listen(zone, 'drop', (event) => {
        event.preventDefault()
        const dropped = Array.from(event.dataTransfer?.files ?? [])
        if (!dropped.length) return
        setActionError('')
        uploadUserFiles(dropped)
          .then(setFiles)
          .catch((err: unknown) => setActionError(err instanceof Error ? err.message : '文件上传失败'))
      })
    })

    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    switch (name) {
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? user?.id ?? ''))}`
      case 'home_settings': return `/home/settings?category=${encodeURIComponent(String(args.category ?? 'account'))}`
      case 'home_files': return '/file'
      case 'home_messages':
      case 'home_security':
        return '#'
      case 'homepage': return '/'
      default: return '#'
    }
  }, [user?.id])

  const context = useMemo(() => ({
    page_name: 'home_files',
    files: (files ?? []).map((file) => ({ name: file.name, size: file.size })),
    handler: {
      user: {
        _id: user?.id ?? 0,
        uname: user?.username ?? '',
        displayName: user?.displayName ?? '',
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPriv: (privilege: number) => Boolean(user) && (privilege === PRIV_USER_PROFILE || privilege === PRIV_CREATE_FILE),
        hasPerm: () => false,
        own: () => true,
      },
      args: { domainId: 'system' },
      request: { query: {}, path: '/file', headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    },
    perm: {},
    PRIV: { PRIV_USER_PROFILE, PRIV_CREATE_FILE },
    avatarUrl,
    UiContext: {},
    UserContext: user,
  }), [files, user])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!user) return <div className="row"><div className="columns"><div className="nothing-placeholder">需要登录</div></div></div>
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!files) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="home_files.html"
        context={context}
        url={buildUrl}
        urlForFile={(name) => `/api/v1/users/${user.id}/files/${encodeURIComponent(name)}`}
        onReady={handleReady}
      />
    </>
  )
}
