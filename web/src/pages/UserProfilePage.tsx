import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getPublicUser, type PublicUser } from '../api.ts'

export function UserProfilePage() {
  const { uid = '' } = useParams()
  const [user, setUser] = useState<PublicUser | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getPublicUser(uid).then(setUser).catch((err: unknown) => setError(err instanceof Error ? err.message : '用户加载失败'))
  }, [uid])

  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>
  if (!user) return <section className="panel"><div className="empty-state">加载中...</div></section>

  return (
    <section className="panel profile-card">
      <div className="profile-avatar">{user.avatar ? <img src={user.avatar} alt="" /> : (user.displayName || user.username).slice(0, 1).toUpperCase()}</div>
      <div className="profile-main">
        <p className="eyebrow">USER #{user.id}</p>
        <h1 className="panel-title">{user.displayName || user.username}</h1>
        <p className="panel-subtitle">@{user.username}{user.studentId ? ` · ${user.studentId}` : ''}</p>
        <p className="profile-meta">注册时间：{new Date(user.regAt).toLocaleString('zh-CN', { hour12: false })}</p>
      </div>
    </section>
  )
}
