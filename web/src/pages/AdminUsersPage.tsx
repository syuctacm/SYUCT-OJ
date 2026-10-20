import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { getAdminUsers, updateAdminUserPriv, type AdminUserPage } from '../api.ts'
import { useAuth } from '../auth.tsx'

export function AdminUsersPage() {
  const { user } = useAuth()
  const [query, setQuery] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')
  const [data, setData] = useState<AdminUserPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busyUID, setBusyUID] = useState<number | null>(null)

  useEffect(() => {
    if (!user || ((user.priv ?? 0) & 1) === 0) {
      setLoading(false)
      return
    }
    setLoading(true)
    getAdminUsers(submittedQuery)
      .then(setData)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '用户加载失败'))
      .finally(() => setLoading(false))
  }, [submittedQuery, user])

  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmittedQuery(query)
  }

  async function toggleAdmin(uid: number, currentPriv: number) {
    const nextPriv = (currentPriv & 1) === 1 ? (currentPriv & ~1) : (currentPriv | 1)
    if (!window.confirm(nextPriv & 1 ? '确认将该用户设为管理员？' : '确认取消该用户的管理员权限？')) return
    setBusyUID(uid)
    setError('')
    try {
      await updateAdminUserPriv(uid, nextPriv)
      setData((current) => current ? {
        ...current,
        items: current.items.map((item) => item.id === uid ? { ...item, priv: nextPriv } : item),
      } : current)
    } catch (err) {
      setError(err instanceof Error ? err.message : '权限更新失败')
    } finally {
      setBusyUID(null)
    }
  }

  if (!user || ((user.priv ?? 0) & 1) === 0) return <section className="panel"><div className="empty-state error-state">需要管理员权限</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">ADMIN · USERS</p>
          <h1 className="panel-title">用户管理</h1>
        </div>
        <form className="search" onSubmit={search}>
          <input className="input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="用户名 / 邮箱 / 姓名 / 学号" />
          <button className="button" type="submit">搜索</button>
        </form>
      </div>
      {loading ? <div className="empty-state">加载中...</div> : null}
      {error ? <div className="empty-state error-state">{error}</div> : null}
      {!loading && !error ? (
        <div className="table-wrap">
          <table className="data-table">
            <thead><tr><th className="col-id">UID</th><th>用户名</th><th>姓名</th><th>学号</th><th>邮箱</th><th>权限</th><th>操作</th><th>注册时间</th></tr></thead>
            <tbody>
              {data?.items.map((item) => (
                <tr key={item.id}>
                  <td className="mono">{item.id}</td>
                  <td><Link className="problem-link" to={`/user/${item.id}`}>{item.username}</Link></td>
                  <td>{item.displayName || '-'}</td>
                  <td className="mono">{item.studentId || '-'}</td>
                  <td>{item.email}</td>
                  <td className="mono">{item.priv}</td>
                  <td><button className="link-button" disabled={busyUID === item.id || item.id === user.id} onClick={() => toggleAdmin(item.id, item.priv)}>{(item.priv & 1) === 1 ? '取消管理员' : '设为管理员'}</button></td>
                  <td>{new Date(item.regAt).toLocaleString('zh-CN', { hour12: false })}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {data?.items.length === 0 ? <div className="empty-state">没有匹配用户</div> : null}
        </div>
      ) : null}
      <div className="panel-footer">
        共 {data?.total ?? 0} 个用户 · <Link to="/admin">返回后台概览</Link>
      </div>
    </section>
  )
}
