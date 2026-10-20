import { useEffect, useState } from 'react'
import { getAdminStats, type AdminStats } from '../api.ts'
import { useAuth } from '../auth.tsx'

const labels: Array<[keyof AdminStats, string]> = [
  ['users', '用户'],
  ['problems', '题目'],
  ['contests', '比赛'],
  ['homeworks', '作业'],
  ['trainings', '训练'],
  ['discussions', '讨论'],
  ['records', '提交记录'],
  ['submissions_today', '今日提交'],
  ['queued_tasks', '待评测任务'],
]

export function AdminPage() {
  const { user } = useAuth()
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getAdminStats().then(setStats).catch((err: unknown) => setError(err instanceof Error ? err.message : '后台数据加载失败'))
  }, [])

  if (!user || ((user.priv ?? 0) & 1) === 0) return <section className="panel"><div className="empty-state error-state">需要管理员权限</div></section>
  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">ADMIN</p>
          <h1 className="panel-title">后台概览</h1>
          <p className="panel-subtitle">只读统计，当前不会修改线上数据</p>
        </div>
      </div>
      <div className="admin-grid">
        {labels.map(([key, label]) => (
          <div className="admin-stat" key={key}>
            <span>{label}</span>
            <strong>{stats ? stats[key] : '-'}</strong>
          </div>
        ))}
      </div>
    </section>
  )
}
