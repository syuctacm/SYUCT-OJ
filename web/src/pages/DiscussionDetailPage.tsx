// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping and the reply/lock interactions.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  createDiscussionReply, deleteDiscussionReply, getDiscussion, getDiscussionNode, getDiscussionNodes, getDiscussionReplies,
  getUsersByIDs, setDiscussionLock, setDiscussionStar, updateDiscussionReply,
  type Discussion, type DiscussionNode, type DiscussionReply, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { mountDiscussionReactions } from '../components/DiscussionReaction.tsx'
import { mountDiscussionHistory } from '../components/DiscussionHistory.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

const DIS_PERMS = {
  PERM_EDIT_DISCUSSION: 1,
  PERM_EDIT_DISCUSSION_SELF: 2,
  PERM_LOCK_DISCUSSION: 3,
  PERM_REPLY_DISCUSSION: 4,
  PERM_DELETE_DISCUSSION_REPLY_SELF_DISCUSSION: 5,
  PERM_DELETE_DISCUSSION_REPLY: 6,
  PERM_EDIT_DISCUSSION_REPLY_SELF: 7,
  PERM_DELETE_DISCUSSION_REPLY_SELF: 8,
  PERM_ADD_REACTION: 9,
} as const

export function DiscussionDetailPage() {
  const { did = '' } = useParams()
  const { user } = useAuth()
  const navigate = useNavigate()
  const [discussion, setDiscussion] = useState<Discussion | null>(null)
  const [replies, setReplies] = useState<DiscussionReply[]>([])
  const [nodes, setNodes] = useState<DiscussionNode[]>([])
  const [parentNode, setParentNode] = useState<DiscussionNode | null>(null)
  const [parentTypeName, setParentTypeName] = useState('node')
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const clickRef = useRef<(event: Event) => void>(() => {})
  const inlineEditorCleanups = useRef(new Map<HTMLFormElement, () => void>())

  const load = useCallback(async () => {
    const [ddoc, drdocs, vnodes] = await Promise.all([
      getDiscussion(did),
      getDiscussionReplies(did),
      getDiscussionNodes(),
    ])
    setDiscussion(ddoc)
    setReplies(drdocs)
    setNodes(vnodes)
  }, [did])

  useEffect(() => {
    setDiscussion(null)
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
  }, [load])

  useEffect(() => {
    const ids = new Set<number>()
    if (discussion) ids.add(discussion.owner)
    for (const reply of replies) {
      ids.add(reply.owner)
      for (const tail of reply.replies ?? []) ids.add(tail.owner)
    }
    getUsersByIDs([...ids].filter((id) => id > 0)).then(setUsers).catch(() => setUsers([]))
  }, [discussion, replies])

  useEffect(() => {
    if (!discussion) return
    if (discussion.parentType === 20) {
      setParentTypeName('node')
      setParentNode(nodes.find((node) => node.docId === String(discussion.parentId)) ?? null)
      return
    }
    const name = String(discussion.parentId ?? '')
    if (discussion.parentType === 10) {
      setParentTypeName('problem')
      getDiscussionNode('problem', name).then(setParentNode).catch(() => setParentNode(null))
    } else if (discussion.parentType === 40) {
      setParentTypeName('training')
      getDiscussionNode('training', name).then(setParentNode).catch(() => setParentNode(null))
    } else if (discussion.parentType === 30) {
      getDiscussionNode('contest', name)
        .then((node) => {
          setParentTypeName('contest')
          setParentNode(node)
        })
        .catch(() => getDiscussionNode('homework', name)
          .then((node) => {
            setParentTypeName('homework')
            setParentNode(node)
          })
          .catch(() => setParentNode(null)))
    }
  }, [discussion, nodes])

  const context = useMemo(() => {
    if (!discussion) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const isOwner = Boolean(user && user.id === discussion.owner)
    const node = parentNode ?? (discussion.parentType === 20
      ? nodes.find((item) => item.docId === String(discussion.parentId)) ?? null
      : null)
    const ddoc = {
      ...discussion,
      _id: discussion.docId,
      docId: discussion.docId,
      react: discussion.react ?? {},
    }
    const drdocs = replies.map((reply) => ({
      ...reply,
      _id: reply.id,
      docId: reply.docId || reply.id,
      parentId: reply.parentId || did,
      react: reply.react ?? {},
      reply: (reply.replies ?? []).map((tail) => ({ ...tail, _id: tail.id, react: {} })),
    }))
    const udict = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    }]))
    const handler = {
      user: {
        _id: user?.id ?? 0,
        uname: user?.username ?? '',
        avatar: user?.avatar ?? '',
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (permission === DIS_PERMS.PERM_EDIT_DISCUSSION) return isAdmin
          if (permission === DIS_PERMS.PERM_EDIT_DISCUSSION_SELF) return Boolean(user)
          if (permission === DIS_PERMS.PERM_LOCK_DISCUSSION) return isAdmin
          if (permission === DIS_PERMS.PERM_REPLY_DISCUSSION) return Boolean(user)
          if (permission === DIS_PERMS.PERM_DELETE_DISCUSSION_REPLY_SELF_DISCUSSION) return isOwner || isAdmin
          if (permission === DIS_PERMS.PERM_DELETE_DISCUSSION_REPLY) return isAdmin
          if (permission === DIS_PERMS.PERM_EDIT_DISCUSSION_REPLY_SELF) return Boolean(user)
          if (permission === DIS_PERMS.PERM_DELETE_DISCUSSION_REPLY_SELF) return Boolean(user)
          if (permission === DIS_PERMS.PERM_ADD_REACTION) return Boolean(user)
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
      },
      args: { domainId: 'system' },
      request: {
        query: {},
        path: `/discuss/${discussion.docId}`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    return {
      page_name: 'discussion_detail',
      ddoc,
      dsdoc: { star: Boolean(discussion.starred) },
      drdocs,
      drcount: discussion.nReply,
      udict,
      vnode: node ? {
        _id: node.docId,
        id: node.docId,
        docId: node.docId,
        type: 20,
        title: node.title || node.docId,
      } : { type: discussion.parentType, id: discussion.parentId },
      reactions: discussion.reactions ?? {},
      handler,
      perm: DIS_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: {
        document: { TYPE_PROBLEM: 10, TYPE_CONTEST: 30, TYPE_DISCUSSION_NODE: 20, TYPE_TRAINING: 40 },
        discussion: { typeDisplay: { 10: 'problem', 20: parentTypeName, 30: parentTypeName, 40: 'training' } },
      },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [did, discussion, nodes, parentNode, parentTypeName, replies, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    switch (name) {
      case 'discussion_main': return '/discuss'
      case 'discussion_detail': return `/discuss/${encodeURIComponent(String(args.did ?? did))}`
      case 'discussion_edit': return `/discuss/${encodeURIComponent(String(args.did ?? did))}/edit`
      case 'discussion_node':
        return `/discuss/${encodeURIComponent(parentTypeName)}/${encodeURIComponent(String(args.name ?? parentNode?.docId ?? ''))}`
      case 'discussion_raw': return `/api/v1/discussions/${encodeURIComponent(String(args.did ?? did))}/raw`
      case 'discussion_reply_raw':
        return `/api/v1/discussions/${encodeURIComponent(String(args.did ?? did))}/raw?docId=${encodeURIComponent(String(args.drid ?? ''))}`
      case 'discussion_tail_reply_raw':
        return `/api/v1/discussions/${encodeURIComponent(String(args.did ?? did))}/raw?docId=${encodeURIComponent(String(args.drrid ?? ''))}`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'wiki_help':
      case 'wiki':
        return '/discuss'
      // Raw history and reaction endpoints are handled by the mounted
      // DiscussionHistory and DiscussionReaction islands.
      default: return '/'
    }
  }, [did, parentNode?.docId, parentTypeName])

  const run = useCallback(async (action: () => Promise<void>) => {
    setActionError('')
    try {
      await action()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '操作失败')
    }
  }, [])

  const findReplyContent = useCallback((drid: string, drrid: string) => {
    if (drrid) {
      for (const reply of replies) {
        for (const tail of reply.replies ?? []) {
          if (tail.id === drrid) return tail.content
        }
      }
    }
    return replies.find((reply) => reply.id === drid)?.content ?? ''
  }, [replies])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = [hydrateHydroEditForm(root, { uploadToUserFiles: true })]
    cleanups.push(mountDiscussionHistory(root))
    cleanups.push(mountDiscussionReactions(root, {
      did,
      canReact: Boolean(user),
      onError: setActionError,
      onRequireLogin: () => navigate('/login'),
    }))
    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }
    const onClick = (event: Event) => clickRef.current(event)
    root.addEventListener('click', onClick)
    cleanups.push(() => root.removeEventListener('click', onClick))
    return () => {
      cleanups.forEach((cleanup) => cleanup())
      for (const cleanup of inlineEditorCleanups.current.values()) cleanup()
      inlineEditorCleanups.current.clear()
    }
  }, [did, navigate, user])

  // Inline boxes created for reply/edit follow the same markup the upstream JS
  // builds at runtime.
  function closeInlineForm(form: HTMLFormElement) {
    inlineEditorCleanups.current.get(form)?.()
    inlineEditorCleanups.current.delete(form)
    form.remove()
  }

  function buildInlineForm(kind: 'reply' | 'edit', drid: string, drrid: string, initial = '') {
    const form = document.createElement('form')
    form.className = 'dczcomments__box'
    form.method = 'post'
    form.dataset.hotkey = 'ctrl+enter:submit,esc:vjCommentBoxCancel'
    form.addEventListener('vjCommentBoxCancel', () => closeInlineForm(form))
    form.innerHTML = `
      <div><textarea class="textbox" name="content" data-markdown required></textarea></div>
      <div>
        <input type="submit" class="rounded primary button" value="${kind === 'reply' ? '回复 (Ctrl+Enter)' : '更新 (Ctrl+Enter)'}">
        <input type="button" class="rounded button" name="cancel" value="取消 (Esc)">
      </div>
      <input type="hidden" name="operation" value="${kind === 'reply' ? 'tail_reply' : 'edit_reply'}">
      <input type="hidden" name="drid" value="${drid}">
      <input type="hidden" name="drrid" value="${drrid}">
    `
    const textarea = form.querySelector('textarea') as HTMLTextAreaElement
    textarea.value = initial
    form.addEventListener('submit', (event) => {
      event.preventDefault()
      const content = textarea.value.trim()
      if (!content) return
      void run(async () => {
        if (kind === 'reply') await createDiscussionReply(did, content, drid)
        else await updateDiscussionReply(did, drrid || drid, content)
        await load()
        closeInlineForm(form)
      })
    })
    form.querySelector<HTMLInputElement>('input[name="cancel"]')?.addEventListener('click', () => {
      closeInlineForm(form)
    })
    inlineEditorCleanups.current.set(form, hydrateHydroEditForm(form, { uploadToUserFiles: true }))
    return form
  }

  async function handleClick(event: Event) {
    const target = (event.target as HTMLElement).closest<HTMLElement>('[data-op]')
    if (!target) return
    event.preventDefault()
    const operation = target.dataset.op
    const data = target.dataset.form ? JSON.parse(target.dataset.form) as Record<string, string> : {}
    if (operation === 'reply') {
      const item = target.closest('.dczcomments__item')
      const holder = item?.querySelector('.commentbox-reply-target')
      if (!holder) return
      const existing = holder.querySelector<HTMLFormElement>('form.dczcomments__box')
      if (existing) {
        closeInlineForm(existing)
        return
      }
      // The upstream reply action pre-fills the author of the reply being
      // answered. The link lives in the nearest .media__body for both a
      // top-level reply and a nested tail reply.
      const profile = target.closest('.media__body')?.querySelector<HTMLAnchorElement>('.user-profile-name')
      const profileUID = profile?.getAttribute('href')?.match(/\/user\/(\d+)/)?.[1]
      const initial = profileUID ? `@[](/user/${profileUID}) ` : ''
      const box = document.createElement('li')
      box.className = 'dczcomments__reply'
      box.appendChild(buildInlineForm('reply', String(data.drid ?? ''), '', initial))
      holder.appendChild(box)
    } else if (operation === 'edit') {
      const mediaBody = target.closest('.media__body')
      const content = mediaBody?.querySelector('.typo.richmedia')
      if (!mediaBody || !content) return
      const existing = mediaBody.querySelector<HTMLFormElement>('form.dczcomments__box')
      if (existing) {
        closeInlineForm(existing)
        return
      }
      const drid = String(data.drid ?? '')
      const drrid = String(data.drrid ?? '')
      mediaBody.insertBefore(buildInlineForm('edit', drid, drrid, findReplyContent(drid, drrid)), content)
    } else if (operation === 'delete') {
      const drid = String(data.drid ?? '')
      const drrid = String(data.drrid ?? '')
      if (!(await confirmDialog('确定删除这条回复吗？'))) return
      void run(async () => {
        await deleteDiscussionReply(did, drrid || drid)
        await load()
      })
    }
  }
  clickRef.current = handleClick

  async function handleSubmit(event: SubmitEvent) {
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const operation = String(data.get('operation') ?? '')
    if (operation === 'reply') {
      event.preventDefault()
      const content = String(data.get('content') ?? '').trim()
      if (!content) return
      await run(async () => {
        await createDiscussionReply(did, content)
        await load()
      })
    } else if (operation === 'set_lock') {
      event.preventDefault()
      const lock = Boolean(data.get('lock'))
      await run(async () => {
        await setDiscussionLock(did, lock)
        await load()
      })
    } else if (operation === 'star' || operation === 'unstar') {
      event.preventDefault()
      await run(async () => {
        await setDiscussionStar(did, operation === 'star')
        await load()
      })
    } else if (operation === 'reaction') {
      // Reactions are not ported yet.
      event.preventDefault()
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!discussion) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="discussion_detail.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
