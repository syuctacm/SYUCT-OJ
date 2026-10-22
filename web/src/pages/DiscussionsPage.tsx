// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping and request handling.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  getDiscussionNode, getDiscussionNodes, getDiscussions, getUsersByIDs,
  type Discussion, type DiscussionNode, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

const NODE_TYPES: Record<string, number> = {
  problem: 10,
  node: 20,
  contest: 30,
  homework: 30,
  training: 40,
}

export function DiscussionsPage() {
  const { type = '', name = '' } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const legacyNode = searchParams.get('node') ?? ''
  const { user } = useAuth()
  const isNode = Boolean(name)
  const typeName = isNode ? (type || 'node') : ''
  const parentType = NODE_TYPES[typeName] ?? 20
  const page = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const [items, setItems] = useState<Discussion[]>([])
  const [total, setTotal] = useState(0)
  const [nodes, setNodes] = useState<DiscussionNode[]>([])
  const [node, setNode] = useState<DiscussionNode | null>(null)
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
    const globalNodes = getDiscussionNodes()
    Promise.all([
      getDiscussions(isNode ? { parentType, parentId: name, page } : { page }),
      globalNodes,
      isNode
        ? (typeName === 'node'
          ? globalNodes.then((items) => items.find((item) => item.docId === name) ?? null)
          : getDiscussionNode(typeName, name))
        : Promise.resolve(null),
    ])
      .then(([discussionPage, nextNodes, currentNode]) => {
        setItems(discussionPage.items)
        setTotal(discussionPage.total)
        setNodes(nextNodes)
        setNode(currentNode)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '讨论加载失败'))
      .finally(() => setLoaded(true))
  }, [isNode, name, page, parentType, typeName])

  useEffect(() => {
    const ids = [...new Set(items.map((item) => item.owner))].filter((id) => id > 0)
    if (!ids.length) {
      setUsers([])
      return
    }
    getUsersByIDs(ids).then(setUsers).catch(() => setUsers([]))
  }, [items])

  const context = useMemo(() => {
    const vnodes = nodes.map((item) => ({
      docId: item.docId,
      title: item.title,
      content: item.category,
      pic: item.pic ?? '',
    }))
    const vndict: Record<string, Record<string, { title: string }>> = { '20': {} }
    for (const item of nodes) vndict['20'][item.docId] = { title: item.title || item.docId }
    if (node && isNode) {
      vndict[String(parentType)] ||= {}
      vndict[String(parentType)][node.docId] = { title: node.title || node.docId }
    }
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
    const typeDisplay: Record<number, string> = {
      10: 'problem',
      20: isNode ? typeName : 'node',
      30: typeName === 'homework' ? 'homework' : 'contest',
      40: 'training',
    }
    return {
      page_name: isNode ? 'discussion_node' : 'discussion_main',
      ddocs,
      udict,
      vndict,
      vnodes,
      page,
      dpcount: Math.max(1, Math.ceil(total / 20)),
      vnode: node ? {
        _id: node.docId,
        id: node.docId,
        docId: node.docId,
        // Use the generic node sidebar for virtual contest/training nodes.
        type: 20,
        title: node.title || node.docId,
        pic: node.pic ?? '',
        owner: node.owner ?? 0,
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
          path: isNode ? `/discuss/${typeName}/${name}` : '/discuss',
          headers: { 'user-agent': navigator.userAgent },
        },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_CREATE_DISCUSSION: 1, PERM_EDIT_DISCUSSION: 2 },
      PRIV: { PRIV_USER_PROFILE: 1 },
      model: {
        document: { TYPE_PROBLEM: 10, TYPE_CONTEST: 30, TYPE_DISCUSSION_NODE: 20, TYPE_TRAINING: 40 },
        discussion: { typeDisplay },
      },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [isNode, items, name, node, nodes, page, parentType, total, typeName, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    switch (name) {
      case 'discussion_main': return '/discuss'
      case 'discussion_detail': {
        const did = encodeURIComponent(String(args.did ?? ''))
        return `/discuss/${did}${args.anchor ? `#${args.anchor}` : ''}`
      }
      case 'discussion_node': {
        const nextType = encodeURIComponent(String(args.type ?? (isNode ? typeName : 'node')))
        return `/discuss/${nextType}/${encodeURIComponent(String(args.name ?? ''))}`
      }
      case 'discussion_create': {
        const nextType = encodeURIComponent(String(args.type ?? (isNode ? typeName : 'node')))
        return `/discuss/${nextType}/${encodeURIComponent(String(args.name ?? ''))}/create`
      }
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      default: return '/'
    }
  }, [isNode, typeName])

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!loaded) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  if (isNode && !node) {
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
