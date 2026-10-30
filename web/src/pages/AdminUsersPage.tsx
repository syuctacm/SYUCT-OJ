// Page markup is rendered from the pinned Hydro ui-default template
// (manage_user_priv.html). Keep this file limited to data mapping and the
// privilege dialog that the template expects.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { getAdminUsers, setAdminUserBan, updateAdminUserPriv, type AdminUserPage } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { Dialog } from '../components/Dialog.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { UserSelectAutoComplete } from '../components/autocomplete/UserSelectAutoComplete.tsx'
import { PRIV_DEFAULT, PRIV_OPTIONS } from '../upstream/priv.ts'
import { avatarUrl } from '../upstream/avatar.ts'

export function AdminUsersPage() {
  const { user } = useAuth()
  const [data, setData] = useState<AdminUserPage | null>(null)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [editing, setEditing] = useState<{ uid: number; priv: number } | null>(null)
  const [dialogBody, setDialogBody] = useState('')
  const [selecting, setSelecting] = useState(false)
  const [pickedUID, setPickedUID] = useState(0)
  const bodyRef = useRef<HTMLDivElement>(null)
  const clickRef = useRef<(event: Event) => void>(() => {})

  const load = useCallback(async () => {
    setData(await getAdminUsers('', 1000))
  }, [])

  useEffect(() => {
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '用户加载失败'))
  }, [load])

  const handleReady = useCallback((root: HTMLDivElement) => {
    // Changing the system default privilege needs a settings page that this
    // rewrite does not have yet; keep the informational row but drop its link.
    root.querySelector('tr[data-uid="default"] [name="set_priv"]')?.remove()
    for (const item of data?.items ?? []) {
      if (item.id === 1 || item.priv & 1) continue
      const row = root.querySelector<HTMLElement>(`tr[data-uid="${item.id}"]`)
      const actions = row?.querySelector<HTMLElement>('.col--actions')
      if (!actions || actions.querySelector('[name="toggle_ban"]')) continue
      const link = document.createElement('a')
      link.setAttribute('name', 'toggle_ban')
      link.dataset.uid = String(item.id)
      link.dataset.banned = item.banned ? 'true' : 'false'
      link.textContent = item.banned ? '解封' : '封禁'
      link.style.marginLeft = '0.75rem'
      actions.appendChild(link)
    }
    const onClick = (event: Event) => clickRef.current(event)
    root.addEventListener('click', onClick)
    return () => root.removeEventListener('click', onClick)
  }, [data])

  function openPrivDialog(uid: number, priv: number) {
    const body = document.querySelector('.dialog__body--set-priv > div')
    if (!body) return
    setDialogBody(body.outerHTML)
    setEditing({ uid, priv })
  }

  function handleClick(event: Event) {
    const target = (event.target as HTMLElement).closest<HTMLElement>('[name="set_priv"], [name="select_user"], [name="toggle_ban"]')
    if (!target) return
    if (target.getAttribute('name') === 'select_user') {
      event.preventDefault()
      setPickedUID(0)
      setSelecting(true)
      return
    }
    event.preventDefault()
    if (target.getAttribute('name') === 'toggle_ban') {
      const uid = Number(target.dataset.uid ?? 0)
      const banned = target.dataset.banned === 'true'
      const item = data?.items.find((entry) => entry.id === uid)
      void confirmDialog(`${banned ? '解封' : '封禁'}用户 ${item?.username ?? uid}？`).then((ok) => {
        if (!ok) return
        setActionError('')
        setAdminUserBan(uid, !banned)
          .then(load)
          .catch((err: unknown) => setActionError(err instanceof Error ? err.message : '用户状态更新失败'))
      })
      return
    }
    openPrivDialog(Number(target.dataset.uid ?? 0), Number(target.dataset.priv ?? 0))
  }
  clickRef.current = handleClick

  useEffect(() => {
    if (!editing) return
    bodyRef.current?.querySelectorAll<HTMLInputElement>('input.priv').forEach((input) => {
      input.checked = (Number(input.value) & editing.priv) !== 0
    })
  }, [editing, dialogBody])

  async function submitPriv() {
    if (!editing) return
    let priv = 0
    bodyRef.current?.querySelectorAll<HTMLInputElement>('input.priv').forEach((input) => {
      if (input.checked) priv |= Number(input.value)
    })
    setActionError('')
    try {
      await updateAdminUserPriv(editing.uid, priv)
      setEditing(null)
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '权限更新失败')
    }
  }

  function confirmSelectUser() {
    const target = data?.items.find((item) => item.id === pickedUID)
    setSelecting(false)
    if (!target) {
      setActionError('未在已加载的用户列表中找到该用户。')
      return
    }
    openPrivDialog(target.id, target.priv)
  }

  const context = useMemo(() => ({
    page_name: 'manage_user_priv',
    udocs: (data?.items ?? []).map((item) => ({
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
      priv: item.priv,
    })),
    defaultPriv: PRIV_DEFAULT,
    Priv: PRIV_OPTIONS,
    handler: {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        hasPerm: () => false,
        own: () => false,
      },
      args: { domainId: 'system' },
      request: { query: {}, path: '/admin/users', headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    },
    avatarUrl,
    UiContext: {},
    UserContext: user,
  }), [data, user])

  const buildUrl = useCallback((name: string) => {
    if (name === 'manage_dashboard') return '/admin'
    if (name === 'manage_user_priv') return '/admin/users'
    if (name === 'manage_user_import') return '/admin/users/import'
    if (name === 'manage_setting') return '/admin/settings'
    if (name === 'manage_audit') return '/admin/audit'
    if (name === 'user_detail') return '/user/'
    return '/'
  }, [])

  if (!user || ((user.priv ?? 0) & 1) === 0) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">需要管理员权限</div></div></div>
  }
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!data) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="manage_user_priv.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
      {editing ? (
        <Dialog
          onOk={() => void submitPriv()}
          onCancel={() => setEditing(null)}
          width="900px"
        >
          <div ref={bodyRef} dangerouslySetInnerHTML={{ __html: dialogBody }} />
        </Dialog>
      ) : null}
      {selecting ? (
        <Dialog
          onOk={confirmSelectUser}
          onCancel={() => setSelecting(false)}
          okLabel="选择"
          cancelLabel="取消"
        >
          <h1>选择用户</h1>
          <UserSelectAutoComplete
            selectedKeys={[]}
            multi={false}
            placeholder="输入用户名搜索"
            onChange={(value) => setPickedUID(Number(value) || 0)}
          />
        </Dialog>
      ) : null}
    </>
  )
}
