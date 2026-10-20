import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
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

  if (!user || ((user.priv ?? 0) & 1) === 0) return <div className="row"><div className="columns"><div className="nothing-placeholder">需要管理员权限</div></div></div>
  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder error-state">{error}</div></div></div>

  return (
    <div className="row" data-sticky-parent>
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">后台概览</h1>
            <div className="section__tools">
              <Link className="primary rounded button" to="/admin/users">用户管理</Link>
            </div>
          </div>
          <div className="section__body no-padding">
            <table className="data-table">
              <colgroup><col className="col--name" /><col className="col--stat" /></colgroup>
              <thead><tr><th className="col--name">项目</th><th className="col--stat">数量</th></tr></thead>
              <tbody>
                {labels.map(([key, label]) => (
                  <tr key={key}>
                    <td className="col--name">{label}</td>
                    <td className="col--stat">{stats ? stats[key] : '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div className="medium-3 columns"><div data-sticky="large">
        <div className="section side">
          <ol className="menu">
            <li className="menu__item">
              <div className="menu__link expandable"><span className="icon icon-info" /> 管理</div>
              <ol className="menu collapsed">
                <li className="menu__item"><Link className="menu__link" to="/admin/users"><span className="icon icon-user" /> 用户权限</Link></li>
              </ol>
            </li>
          </ol>
        </div>
      </div></div>
    </div>
  )
}
