import { useEffect, useLayoutEffect, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { applyPageMeta, pageMetaForPath } from '../page-meta.ts'

const navClass = ({ isActive }: { isActive: boolean }) => `nav__item${isActive ? ' nav--active' : ''}`

export function Layout() {
  const { user, logout } = useAuth()
  const location = useLocation()
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    setMenuOpen(false)
  }, [location.pathname])

  useLayoutEffect(() => {
    applyPageMeta(pageMetaForPath(location.pathname))
  }, [location.pathname])

  useEffect(() => {
    document.documentElement.classList.toggle('slideout-open', menuOpen)
    return () => document.documentElement.classList.remove('slideout-open')
  }, [menuOpen])

  async function handleLogout() {
    await logout()
  }

  return (
    <>
      <nav className="nav slideout-menu" id="menu">
        <div className="row"><div className="columns clearfix">
          <ol className="nav__list nav__list--main clearfix">
            <li className="nav__list-item">
              <NavLink to="/"><img className="nav__logo" src="/nav_logo_dark.png" alt="SYUCT ACM" /></NavLink>
            </li>
            <li className="nav__list-item"><NavLink to="/" end className={navClass}>首页</NavLink></li>
            <li className="nav__list-item"><NavLink to="/p" className={navClass}>题库</NavLink></li>
            <li className="nav__list-item"><NavLink to="/training" className={navClass}>训练</NavLink></li>
            <li className="nav__list-item"><NavLink to="/contest" className={navClass}>比赛</NavLink></li>
            <li className="nav__list-item"><NavLink to="/homework" className={navClass}>作业</NavLink></li>
            <li className="nav__list-item"><NavLink to="/discuss" className={navClass}>讨论</NavLink></li>
            <li className="nav__list-item"><NavLink to="/record" className={navClass}>评测记录</NavLink></li>
          </ol>
          <ol className="nav__list nav__list--secondary clearfix">
            {user ? (
              <>
                {((user.priv ?? 0) & 1) !== 0 ? (
                  <li className="nav__list-item"><NavLink to="/admin" className={navClass}>后台</NavLink></li>
                ) : null}
                <li className="nav__list-item"><NavLink to={`/user/${user.id}`} className={navClass}>{user.displayName || user.username}</NavLink></li>
                <li className="nav__list-item"><button type="button" className="nav__item legacy-nav-button" onClick={handleLogout}>退出</button></li>
              </>
            ) : (
              <li className="nav__list-item"><NavLink to="/login" className={navClass}>登录</NavLink></li>
            )}
          </ol>
        </div></div>
      </nav>
      <nav className="nav--shadow" />

      <div className="slideout-panel" id="panel">
        <div className="slideout-overlay" onClick={() => setMenuOpen(false)} />
        <div className="header--mobile">
          <div className="row"><div className="columns clearfix">
            <div className="float-left">
              <NavLink className="header--mobile__domain" to="/" target="_self">
                <img src="/nav_logo_dark.png" alt="SYUCT ACM" />
              </NavLink>
            </div>
            <div className="float-right">
              <button type="button" className="header__hamburger" aria-label="打开菜单" onClick={() => setMenuOpen((open) => !open)}>
                <div className={`hamburger hamburger--spin${menuOpen ? ' is-active' : ''}`}>
                  <span className="hamburger-box"><span className="hamburger-inner" /></span>
                </div>
              </button>
            </div>
          </div></div>
        </div>

        <div className="main">
          <Outlet />
        </div>

        <div className="footer">
          <div className="row"><div className="columns">
            <div className="row footer__links">
              <div className="medium-3 large-2 columns footer__category expandable">
                <h1>状态<span className="expand-icon"><span className="icon icon-expand_more" /></span></h1>
                <div className="footer__category__expander"><ul className="footer__category__list">
                  <li className="footer__category__item"><NavLink to="/record">评测记录</NavLink></li>
                </ul></div>
              </div>
              <div className="medium-3 large-2 columns footer__category expandable">
                <h1>开发<span className="expand-icon"><span className="icon icon-expand_more" /></span></h1>
                <div className="footer__category__expander"><ul className="footer__category__list">
                  <li className="footer__category__item"><a href="https://github.com/hydro-dev/Hydro" target="_blank" rel="noreferrer">Open Source</a></li>
                </ul></div>
              </div>
              <div className="medium-3 large-2 columns footer__category expandable">
                <h1>支持<span className="expand-icon"><span className="icon icon-expand_more" /></span></h1>
                <div className="footer__category__expander"><ul className="footer__category__list">
                  <li className="footer__category__item"><NavLink to="/discuss">帮助与反馈</NavLink></li>
                </ul></div>
              </div>
              <div className="medium-3 large-2 columns footer__category expandable end">
                <h1>关于<span className="expand-icon"><span className="icon icon-expand_more" /></span></h1>
                <div className="footer__category__expander"><ul className="footer__category__list">
                  <li className="footer__category__item">SYUCT ACM</li>
                </ul></div>
              </div>
            </div>
            <div className="footer__extra-link clearfix">
              <div className="footer__extra-left">
                <ol className="clearfix">
                  <li className="footer__extra-link-item">沈阳化工大学 ACM-ICPC 集训队</li>
                </ol>
              </div>
              <div className="footer__extra-right">
                <ol className="clearfix">
                  <li className="footer__extra-link-item">Powered by <a href="https://hydro.js.org">Hydro</a> Community</li>
                </ol>
              </div>
            </div>
          </div></div>
        </div>
      </div>
    </>
  )
}
