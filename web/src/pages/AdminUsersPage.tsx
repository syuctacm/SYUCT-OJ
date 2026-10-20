import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { getAdminUsers, updateAdminUserPriv, type AdminUserPage } from '../api.ts'
import { useAuth } from '../auth.tsx'

function privilegeDescription(value: number) {
  if (value === -1) return ['PRIV_ALL']
  if (value === 0) return ['PRIV_NONE']
  const values: string[] = []
  if (value & 1) values.push('+PRIV_EDIT_SYSTEM')
  if (value & 2) values.push('+PRIV_USER_PROFILE')
  if (!values.length) values.push(`+${value}`)
  return values
}

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
      setData((current) => current ? { ...current, items: current.items.map((item) => item.id === uid ? { ...item, priv: nextPriv } : item) } : current)
    } catch (err) {
      setError(err instanceof Error ? err.message : '权限更新失败')
    } finally {
      setBusyUID(null)
    }
  }

  if (!user || ((user.priv ?? 0) & 1) === 0) return <div className="row"><div className="columns"><div className="nothing-placeholder">需要管理员权限</div></div></div>

  return (
    <div className="row"><div className="columns">
      <div className="section">
        <div className="section__header">
          <h1 className="section__title">用户权限</h1>
          <div className="section__tools">
            <form className="form--inline" onSubmit={search}>
              <input className="inline compact textbox" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="用户名 / 邮箱 / 姓名 / 学号" />
              <button className="primary rounded button compact" type="submit"><span className="icon icon-search" /> 搜索</button>
            </form>
          </div>
        </div>
        <div className="section__body no-padding">
          {loading ? <div className="nothing-placeholder">加载中...</div> : null}
          {error ? <div className="nothing-placeholder error-state">{error}</div> : null}
          {!loading && !error ? (
            <table className="data-table">
              <colgroup><col className="col--uid" /><col className="col--user" /><col className="col--priv_info" /><col className="col--priv" /><col className="col--actions" /></colgroup>
              <thead><tr><th className="col--uid">用户 ID</th><th className="col--user">用户名</th><th className="col--priv_info">权限</th><th className="col--priv">值</th><th className="col--actions">操作</th></tr></thead>
              <tbody>
                <tr data-uid="default">
                  <td className="col--uid">#</td>
                  <td className="col--user">默认权限</td>
                  <td className="col--priv_info">PRIV_NONE</td>
                  <td className="col--priv">0</td>
                  <td className="col--actions">-</td>
                </tr>
                {data?.items.map((item) => (
                  <tr data-uid={item.id} key={item.id}>
                    <td className="col--uid">{item.id}</td>
                    <td className="col--user"><Link to={`/user/${item.id}`}>{item.displayName || item.username}</Link></td>
                    <td className="col--priv_info">
                      {privilegeDescription(item.priv).map((value) => <p className={value.startsWith('-') ? 'priv_minus' : 'priv_plus'} key={value}>{value}</p>)}
                    </td>
                    <td className="col--priv" title={String(item.priv)}>{item.priv}</td>
                    <td className="col--actions">
                      <button className="link-button" disabled={busyUID === item.id || item.id === user.id} onClick={() => toggleAdmin(item.id, item.priv)}>
                        {(item.priv & 1) === 1 ? '取消管理员' : '设为管理员'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : null}
        </div>
        <div className="section__footer">共 {data?.total ?? 0} 个用户 · <Link to="/admin">返回后台概览</Link></div>
      </div>
    </div></div>
  )
}
