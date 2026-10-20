import { NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../auth.tsx'

export function Layout() {
  const { user, logout } = useAuth()

  async function handleLogout() {
    await logout()
  }

  return (
    <>
      <header className="site-header">
        <div className="container header-inner">
          <NavLink to="/p" className="brand">
            <img src="/logo.png" alt="" className="brand-logo" />
            <span>SYUCT <strong>ACM</strong></span>
          </NavLink>
          <nav className="site-nav">
            <NavLink to="/p" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>题库</NavLink>
            <NavLink to="/contest" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>比赛</NavLink>
            <NavLink to="/record" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>提交记录</NavLink>
            <a className="nav-link" href="https://oj.syuctacm.cn" target="_blank" rel="noreferrer">旧 OJ</a>
          </nav>
          <div className="account">
            {user ? (
              <>
                <span className="account-name">{user.displayName || user.username}</span>
                <button className="link-button" onClick={handleLogout}>退出</button>
              </>
            ) : (
              <NavLink to="/login" className="nav-link">登录</NavLink>
            )}
          </div>
        </div>
      </header>
      <main className="container site-main">
        <Outlet />
      </main>
      <footer className="site-footer">
        <div className="container">SYUCT ACM · 沈阳化工大学 ACM-ICPC 集训队</div>
      </footer>
    </>
  )
}
