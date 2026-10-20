import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useParams } from 'react-router-dom'
import { getPublicUser, type PublicUser } from '../api.ts'

function formatDate(value?: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

export function UserProfilePage() {
  const { uid = '' } = useParams()
  const [user, setUser] = useState<PublicUser | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getPublicUser(uid).then(setUser).catch((err: unknown) => setError(err instanceof Error ? err.message : '用户加载失败'))
  }, [uid])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!user) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <div className="row">
      <div className="medium-9 columns">
        <div className="section">
          <div className="profile-header user-profile-bg">
            <div className="profile-header__content">
              <div className="media">
                <div className="media__left">
                  {user.avatar ? <img src={user.avatar} width={120} height={120} className="large user-profile-avatar" alt="" /> : (
                    <div className="large user-profile-avatar" style={{ display: 'grid', placeItems: 'center', width: 120, height: 120, fontSize: 48, background: '#fff', color: '#56758f' }}>
                      {(user.displayName || user.username).slice(0, 1).toUpperCase()}
                    </div>
                  )}
                </div>
                <div className="media__body profile-header__main">
                  <h1>{user.username}{user.displayName ? <small> ({user.displayName})</small> : null}</h1>
                  <p>
                    UID: {user.id}, 注册于 {formatDate(user.regAt)}, 最后登录于 {formatDate(user.loginAt)}.
                  </p>
                  <p>已通过 {user.nAccept} 道题，累计提交 {user.nSubmit} 次。</p>
                </div>
              </div>
            </div>
          </div>
          <div className="profile-content">
            <div className="section__tab-container immersive memo">
              <ul className="section__tabs">
                <li className="section__tab-item">
                  <h1 className="section__tab-title">简介</h1>
                  <div className="section__tab-main">
                    {user.bio ? <div className="section__body typo richmedia"><ReactMarkdown remarkPlugins={[remarkGfm]}>{user.bio}</ReactMarkdown></div> : (
                      <div className="nothing-placeholder compact">这位用户还没有填写简介。</div>
                    )}
                  </div>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <div className="section side">
          <div className="section__body">
            <div className="balancer sidebar-user-stat">
              <div className="balancer__body"><div className="numbox"><div className="numbox__num medium">{user.nSubmit}</div><div className="numbox__text">提交</div></div></div>
              <div className="balancer__body"><div className="numbox"><div className="numbox__num medium">{user.nAccept}</div><div className="numbox__text">通过</div></div></div>
              <div className="balancer__body"><div className="numbox"><div className="numbox__num medium">0</div><div className="numbox__text">获赞题解</div></div></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
