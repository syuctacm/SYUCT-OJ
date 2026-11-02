import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  deleteAdminIPLogin, getAdminIPLogins, setAdminIPLogin, type AdminIPLogin,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { ErrorPage } from './ErrorPage.tsx'

const ADMIN_LINKS: Record<string, string> = {
  manage_dashboard: '/admin',
  manage_user_priv: '/admin/users',
  manage_user_import: '/admin/users/import',
  manage_setting: '/admin/settings',
  manage_audit: '/admin/audit',
  manage_discussion_nodes: '/admin/discussion-nodes',
  manage_ip_login: '/admin/ip-login',
}

export function AdminIPLoginPage() {
  const { user, loading } = useAuth()
  const [items, setItems] = useState<AdminIPLogin[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)

  const load = useCallback(async () => {
    if (!isAdmin) return
    setError('')
    try {
      setItems(await getAdminIPLogins())
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'IP 自动登录映射加载失败')
    }
  }, [isAdmin])

  useEffect(() => {
    if (!loading) void load()
  }, [load, loading])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form[data-ip-login-form]')
    if (!form) return
    const cleanups: Array<() => void> = []
    const ipInput = form.querySelector<HTMLInputElement>('[name="ip"]')
    const uidInput = form.querySelector<HTMLInputElement>('[name="uid"]')
    const onSubmit = async (event: SubmitEvent) => {
      event.preventDefault()
      const ip = ipInput?.value.trim() ?? ''
      const uid = Number(uidInput?.value)
      setError('')
      setNotice('')
      if (!ip || !Number.isSafeInteger(uid) || uid <= 0) {
        setError('请输入有效的 IP 地址和用户 UID')
        return
      }
      try {
        await setAdminIPLogin(ip, uid)
        setNotice(`已保存 ${ip} → UID ${uid}`)
        form.reset()
        await load()
      } catch (reason) {
        setError(reason instanceof Error ? reason.message : 'IP 自动登录映射保存失败')
      }
    }
    const onClick = async (event: Event) => {
      const target = event.target
      if (!(target instanceof Element)) return
      const row = target.closest<HTMLTableRowElement>('tr[data-ip]')
      if (!row) return
      const ip = row.dataset.ip ?? ''
      const uid = Number(row.dataset.uid)
      if (target.closest('[data-ip-edit]')) {
        event.preventDefault()
        if (ipInput) ipInput.value = ip
        if (uidInput) uidInput.value = String(uid)
        setNotice('')
        ipInput?.focus()
        return
      }
      if (target.closest('[data-ip-delete]')) {
        event.preventDefault()
        if (!(await confirmDialog(`确定删除 ${ip} 的自动登录映射？`))) return
        setError('')
        setNotice('')
        try {
          await deleteAdminIPLogin(ip)
          setNotice(`已删除 ${ip} 的映射`)
          await load()
        } catch (reason) {
          setError(reason instanceof Error ? reason.message : 'IP 自动登录映射删除失败')
        }
      }
    }
    form.addEventListener('submit', onSubmit)
    root.addEventListener('click', onClick)
    cleanups.push(() => form.removeEventListener('submit', onSubmit))
    cleanups.push(() => root.removeEventListener('click', onClick))
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [load])

  const context = useMemo(() => ({ page_name: 'manage_ip_login', items }), [items])
  const buildUrl = useCallback((name: string) => ADMIN_LINKS[name] ?? '/', [])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!isAdmin) return <ErrorPage status={403} />
  return (
    <>
      {notice ? <div className="row"><div className="columns"><blockquote className="note"><p>{notice}</p></blockquote></div></div> : null}
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate template="manage_ip_login.html" context={context} url={buildUrl} onReady={handleReady} />
    </>
  )
}
