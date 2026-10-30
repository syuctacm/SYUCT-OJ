import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  createDiscussionNode, deleteDiscussionNode, getAdminDiscussionNodes, updateDiscussionNode,
  type DiscussionNode,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { ErrorPage } from './ErrorPage.tsx'

export function AdminDiscussionNodesPage() {
  const { user, loading } = useAuth()
  const [nodes, setNodes] = useState<DiscussionNode[]>([])
  const [editing, setEditing] = useState<DiscussionNode | null>(null)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)

  const load = useCallback(async () => {
    if (!isAdmin) return
    setError('')
    try {
      setNodes(await getAdminDiscussionNodes())
    } catch (err) {
      setError(err instanceof Error ? err.message : '讨论节点加载失败')
    }
  }, [isAdmin])

  useEffect(() => {
    if (loading) return
    void load()
  }, [load, loading])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form[data-node-form]')
    if (!form) return
    const cleanups: Array<() => void> = []
    const reset = () => {
      setEditing(null)
      setNotice('')
      form.reset()
    }
    const onSubmit = async (event: SubmitEvent) => {
      event.preventDefault()
      setError('')
      setNotice('')
      const data = new FormData(form)
      const input = {
        docId: String(data.get('docId') ?? '').trim(),
        title: String(data.get('title') ?? '').trim(),
        category: String(data.get('category') ?? '').trim(),
        pic: String(data.get('pic') ?? '').trim(),
      }
      try {
        if (editing) {
          await updateDiscussionNode(editing.docId, input)
          setNotice(`已更新节点 ${editing.docId}`)
        } else {
          await createDiscussionNode(input)
          setNotice(`已创建节点 ${input.docId}`)
        }
        setEditing(null)
        await load()
      } catch (err) {
        setError(err instanceof Error ? err.message : '保存讨论节点失败')
      }
    }
    const onClick = async (event: Event) => {
      const target = event.target
      if (!(target instanceof Element)) return
      const row = target.closest<HTMLTableRowElement>('tr[data-node-id]')
      if (!row) return
      const node = {
        docId: row.dataset.nodeId ?? '',
        title: row.dataset.nodeTitle ?? '',
        category: row.dataset.nodeCategory ?? '',
        pic: row.dataset.nodePic ?? '',
      }
      if (target.closest('[data-node-edit]')) {
        event.preventDefault()
        setEditing(node)
        setNotice('')
        return
      }
      if (target.closest('[data-node-delete]')) {
        event.preventDefault()
        if (!(await confirmDialog(`确定删除讨论节点 ${node.docId}？`))) return
        try {
          await deleteDiscussionNode(node.docId)
          if (editing?.docId === node.docId) setEditing(null)
          setNotice(`已删除节点 ${node.docId}`)
          await load()
        } catch (err) {
          setError(err instanceof Error ? err.message : '删除讨论节点失败')
        }
      }
    }
    const resetButton = root.querySelector<HTMLElement>('[data-node-reset]')
    const onReset = () => reset()
    form.addEventListener('submit', onSubmit)
    root.addEventListener('click', onClick)
    resetButton?.addEventListener('click', onReset)
    cleanups.push(() => form.removeEventListener('submit', onSubmit))
    cleanups.push(() => root.removeEventListener('click', onClick))
    cleanups.push(() => resetButton?.removeEventListener('click', onReset))
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [editing, load])

  const context = useMemo(() => ({
    page_name: 'manage_discussion_nodes',
    nodes,
    editing,
  }), [editing, nodes])

  if (loading) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!isAdmin) return <ErrorPage status={403} />
  return (
    <>
      {notice ? <div className="row"><div className="columns"><blockquote className="note"><p>{notice}</p></blockquote></div></div> : null}
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="manage_discussion_nodes.html"
        context={context}
        onReady={handleReady}
      />
    </>
  )
}
