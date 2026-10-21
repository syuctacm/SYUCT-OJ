// Fills the upstream `#printKioskContainer`: a task list for the caller and,
// for contest editors, a kiosk that claims pending tasks one at a time.
import { useCallback, useEffect, useState } from 'react'
import {
  allocatePrintTask, getPrintTasks, updatePrintTask,
  type PrintTask, type UserChoice,
} from '../api.ts'

function escapeHtml(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

function openPrintWindow(task: PrintTask) {
  const win = window.open('', '_blank', 'width=800,height=600,popup=1')
  if (!win) return
  win.document.write(`<!doctype html><html><head><title>${escapeHtml(task.title)}</title>`
    + '<style>body{font-family:monospace;white-space:pre-wrap;word-break:break-word;padding:16px}</style>'
    + `</head><body><pre>${escapeHtml(task.content)}</pre></body></html>`)
  win.document.close()
  win.focus()
  win.print()
}

export function PrintKioskPanel({ tid, kiosk }: { tid: string; kiosk: boolean }) {
  const [tasks, setTasks] = useState<PrintTask[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])
  const [isEditor, setIsEditor] = useState(false)
  const [current, setCurrent] = useState<{ task: PrintTask; user?: UserChoice } | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    const data = await getPrintTasks(tid)
    setTasks(data.items)
    setUsers(data.users)
    setIsEditor(data.isEditor)
  }, [tid])

  useEffect(() => {
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '打印队列加载失败'))
  }, [load])

  useEffect(() => {
    if (!kiosk || !isEditor) return
    let cancelled = false
    const tick = async () => {
      try {
        const next = await allocatePrintTask(tid)
        if (cancelled) return
        if (next) {
          setCurrent(next)
          await load()
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : '取打印任务失败')
      }
    }
    void tick()
    const timer = window.setInterval(() => void tick(), 5000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [isEditor, kiosk, load, tid])

  async function finish(task: PrintTask) {
    try {
      await updatePrintTask(tid, task.id, 'printed')
      setCurrent(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : '打印任务更新失败')
    }
  }

  const nameOf = (uid: number) => {
    const item = users.find((user) => user.id === uid)
    return item?.displayName || item?.username || `UID ${uid}`
  }

  if (error) return <blockquote className="warn"><p>{error}</p></blockquote>
  return (
    <div>
      {kiosk && isEditor ? (
        <div className="section__body">
          <blockquote className="note"><p>打印台已启用，正在轮询待打印任务…</p></blockquote>
          {current ? (
            <div>
              <h2>{current.task.title}</h2>
              <p>提交者：{current.user?.displayName || current.user?.username || `UID ${current.task.owner}`}</p>
              <pre style={{ maxHeight: 320, overflow: 'auto' }}>{current.task.content}</pre>
              <button type="button" className="rounded primary button" onClick={() => openPrintWindow(current.task)}>打印</button>{' '}
              <button type="button" className="rounded button" onClick={() => void finish(current.task)}>标记完成</button>
            </div>
          ) : <p>暂无待打印任务。</p>}
        </div>
      ) : null}
      <table className="data-table">
        <colgroup>
          <col className="col--status" />
          <col className="col--name" />
          <col className="col--user" />
          <col className="col--actions" />
        </colgroup>
        <thead>
          <tr>
            <th className="col--status">状态</th>
            <th className="col--name">文件</th>
            <th className="col--user">提交者</th>
            <th className="col--actions">操作</th>
          </tr>
        </thead>
        <tbody>
          {tasks.map((task) => (
            <tr key={task.id}>
              <td className="col--status">{task.status}</td>
              <td className="col--name">{task.title}</td>
              <td className="col--user">{nameOf(task.owner)}</td>
              <td className="col--actions">
                <a href="javascript:;" onClick={() => openPrintWindow(task)}>预览打印</a>
                {isEditor && task.status !== 'printed'
                  ? <> | <a href="javascript:;" onClick={() => void finish(task)}>标记完成</a></>
                  : null}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
