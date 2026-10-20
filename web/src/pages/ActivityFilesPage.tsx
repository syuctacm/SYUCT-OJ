import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  deleteActivityFiles,
  getActivityFiles,
  getContest,
  getHomework,
  getTraining,
  uploadActivityFiles,
  type ActivityFile,
  type ActivityFileResource,
} from '../api.ts'
import { useAuth } from '../auth.tsx'

type Kind = 'contest' | 'homework' | 'training'

const config: Record<Kind, {
  resource: ActivityFileResource
  title: string
  detail: (tid: string) => string
  loadOwner: (tid: string) => Promise<number>
}> = {
  contest: {
    resource: 'contests',
    title: '比赛文件',
    detail: (tid) => `/contest/${encodeURIComponent(tid)}`,
    loadOwner: (tid) => getContest(tid).then((item) => item.owner),
  },
  homework: {
    resource: 'homeworks',
    title: '作业文件',
    detail: (tid) => `/homework/${encodeURIComponent(tid)}`,
    loadOwner: (tid) => getHomework(tid).then((item) => item.owner),
  },
  training: {
    resource: 'trainings',
    title: '训练文件',
    detail: (tid) => `/training/${encodeURIComponent(tid)}`,
    loadOwner: (tid) => getTraining(tid).then((item) => item.owner),
  },
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MiB`
}

export function ActivityFilesPage({ kind }: { kind: Kind }) {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const page = config[kind]
  const [files, setFiles] = useState<ActivityFile[]>([])
  const [ownerID, setOwnerID] = useState<number | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    setLoading(true)
    setError('')
    Promise.all([getActivityFiles(page.resource, tid), page.loadOwner(tid)])
      .then(([nextFiles, owner]) => {
        setFiles(nextFiles)
        setOwnerID(owner)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '文件加载失败'))
      .finally(() => setLoading(false))
  }, [page, tid])

  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  const canManage = Boolean(user && (isAdmin || user.id === ownerID))

  function toggle(name: string) {
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  function toggleAll() {
    setSelected((current) => current.size === files.length ? new Set() : new Set(files.map((file) => file.name)))
  }

  async function upload(event: ChangeEvent<HTMLInputElement>) {
    const chosen = Array.from(event.target.files ?? [])
    event.target.value = ''
    if (!chosen.length) return
    setBusy(true)
    setError('')
    try {
      setFiles(await uploadActivityFiles(page.resource, tid, chosen))
      setSelected(new Set())
    } catch (err) {
      setError(err instanceof Error ? err.message : '文件上传失败')
    } finally {
      setBusy(false)
    }
  }

  async function removeSelected() {
    if (!selected.size) return
    setBusy(true)
    setError('')
    try {
      const names = Array.from(selected)
      await deleteActivityFiles(page.resource, tid, names)
      setFiles((current) => current.filter((file) => !selected.has(file.name)))
      setSelected(new Set())
    } catch (err) {
      setError(err instanceof Error ? err.message : '文件删除失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">{page.title}</h1>
            {canManage ? (
              <div className="section__tools">
                <button className="primary rounded button" type="button" onClick={() => inputRef.current?.click()} disabled={busy}>
                  上传文件
                </button>
                <input ref={inputRef} type="file" multiple hidden onChange={upload} />
              </div>
            ) : null}
          </div>
          <div className="section__body no-padding files">
            {loading ? <div className="nothing-placeholder">加载中...</div> : null}
            {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
            {!loading && !error && files.length === 0 ? <div className="nothing-placeholder">暂无文件</div> : null}
            {!loading && !error && files.length > 0 ? (
              <table className="data-table">
                <colgroup>
                  {canManage ? <col className="col--checkbox" /> : null}
                  <col className="col--name" />
                  <col className="col--size" />
                  <col className="col--operation" />
                </colgroup>
                <thead>
                  <tr>
                    {canManage ? (
                      <th className="col--checkbox">
                        <label className="compact checkbox">
                          <input type="checkbox" checked={selected.size === files.length} onChange={toggleAll} />
                        </label>
                      </th>
                    ) : null}
                    <th className="col--name">文件名</th>
                    <th className="col--size">大小</th>
                    <th className="col--operation"><span className="icon icon-wrench" /></th>
                  </tr>
                </thead>
                <tbody>
                  {files.map((file) => (
                    <tr key={file.name}>
                      {canManage ? (
                        <td className="col--checkbox">
                          <label className="compact checkbox">
                            <input type="checkbox" checked={selected.has(file.name)} onChange={() => toggle(file.name)} />
                          </label>
                        </td>
                      ) : null}
                      <td className="col--name"><a href={file.downloadUrl}>{file.name}</a></td>
                      <td className="col--size">{formatSize(file.size)}</td>
                      <td className="col--operation"><a href={file.downloadUrl} className="icon icon-download" aria-label="下载" /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : null}
          </div>
          {canManage && files.length > 0 ? (
            <div className="section__body">
              <button className="rounded button" type="button" onClick={removeSelected} disabled={!selected.size || busy}>
                删除选中
              </button>
            </div>
          ) : null}
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__body">
            <ol className="menu">
              <li className="menu__item"><Link className="menu__link" to={page.detail(tid)}><span className="icon icon-arrow-left" /> 返回详情</Link></li>
            </ol>
          </div>
        </div>
      </div>
    </div>
  )
}
