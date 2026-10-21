// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping and request handling.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  getDiscussionNodes, getDiscussions, getUsersByIDs,
  type Discussion, type DiscussionNode, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function DiscussionsPage() {
  const { name: nodeName } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const legacyNode = searchParams.get('node') ?? ''
  const { user } = useAuth()
  const isNode = Boolean(nodeName)
  const [items, setItems] = useState<Discussion[]>([])
  const [nodes, setNodes] = useState<DiscussionNode[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (legacyNode && !isNode) {
      navigate(`/discuss/node/${encodeURIComponent(legacyNode)}`, { replace: true })
    }
  }, [isNode, legacyNode, navigate])

  useEffect(() => {
    setLoaded(false)
    setError('')
    Promise.all([
      getDiscussions(isNode ? { parentType: 20, parentId: nodeName } : {}),
      getDiscussionNodes(),
    ])
      .then(([discussions, nextNodes]) => {
        setItems(discussions)
        setNodes(nextNodes)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
      .finally(() => setLoaded(true))
  }, [isNode, nodeName])

  useEffect(() => {
    const ids = [...new Set(items.map((item) => item.owner))].filter((id) => id > 0)
    if (!ids.length) {
      setUsers([])
      return
    }
    getUsersByIDs(ids).then(setUsers).catch(() => setUsers([]))
  }, [items])

  const vnode = useMemo(
    () => (isNode ? nodes.find((node) => node.docId === nodeName) ?? null : null),
    [isNode, nodeName, nodes],
  )

  const context = useMemo(() => {
    const vnodes = nodes.map((node) => ({
      docId: node.docId,
      title: node.title,
      content: node.category,
      pic: node.pic ?? '',
    }))
    const vndict: Record<string, Record<string, { title: string }>> = { '20': {} }
    for (const node of nodes) vndict['20'][node.docId] = { title: node.title || node.docId }
    const ddocs = items.map((item) => ({
      ...item,
      _id: item.docId,
      updateAt: new Date(item.updateAt),
    }))
    const udict = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    }]))
    return {
      page_name: isNode ? 'discussion_node' : 'discussion_main',
      ddocs,
      udict,
      vndict,
      vnodes,
      vnode: vnode ? {
        _id: vnode.docId,
        id: vnode.docId,
        docId: vnode.docId,
        type: 20,
        title: vnode.title,
        pic: vnode.pic ?? '',
      } : {},
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => permission === 1 && Boolean(user),
          hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
          own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
        },
        args: { domainId: 'system' },
        request: {
          query: {},
          path: isNode ? `/discuss/node/${nodeName}` : '/discuss',
          headers: { 'user-agent': navigator.userAgent },
        },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_CREATE_DISCUSSION: 1, PERM_EDIT_DISCUSSION: 2 },
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: {
        document: { TYPE_PROBLEM: 10, TYPE_CONTEST: 30, TYPE_DISCUSSION_NODE: 20, TYPE_TRAINING: 40 },
        discussion: { typeDisplay: { 10: 'problem', 20: 'node', 30: 'contest', 40: 'training' } },
      },
      avatarUrl: (value?: string) => value || '/logo.png',
      UiContext: {},
      UserContext: user,
    }
  }, [isNode, items, nodeName, nodes, user, users, vnode])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    switch (name) {
      case 'discussion_main': return '/discuss'
      case 'discussion_detail': {
        const did = encodeURIComponent(String(args.did ?? ''))
        return `/discuss/${did}${args.anchor ? `#${args.anchor}` : ''}`
      }
      case 'discussion_node': return `/discuss/node/${encodeURIComponent(String(args.name ?? ''))}`
      case 'discussion_create': return `/discuss/node/${encodeURIComponent(String(args.name ?? ''))}/create`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      default: return '/'
    }
  }, [])

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!loaded) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  if (isNode && !vnode) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">讨论节点不存在</div></div></div>
  }
  return (
    <HydroTemplate
      template="discussion_main_or_node.html"
      context={context}
      url={buildUrl}
    />
  )
}
