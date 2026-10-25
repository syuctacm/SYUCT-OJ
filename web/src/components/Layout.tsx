import { useCallback, useLayoutEffect, useMemo, useRef } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { applyPageMeta, pageMetaForPath } from '../page-meta.ts'
import { initHydroFooter } from '../upstream/footer.ts'
import { buildLayoutUrl, layoutChromeContext } from '../upstream/layout.ts'
import { initHydroNavigation } from '../upstream/navigation.ts'
import { cleanupHydroOverlays } from '../upstream/overlays.ts'
import { installHydroRuntime } from '../upstream/runtime.ts'
import { HydroTemplate } from './HydroTemplate.tsx'
import { NotificationsHost } from './Notification.tsx'
import { PageLoader } from './PageLoader.tsx'

export function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const mainRef = useRef<HTMLDivElement>(null)
  const pageName = pageMetaForPath(location.pathname).name

  useLayoutEffect(() => {
    applyPageMeta(pageMetaForPath(location.pathname))
  }, [location.pathname])

  useLayoutEffect(() => {
    if (!mainRef.current) return
    return installHydroRuntime(mainRef.current)
  }, [])

  const handleLogout = useCallback(async () => {
    await logout()
    navigate('/login')
  }, [logout, navigate])

  const handleNavReady = useCallback((root: HTMLDivElement) => {
    const cleanupNavigation = initHydroNavigation(root, { onLogout: handleLogout })
    return () => {
      cleanupNavigation()
      cleanupHydroOverlays(root)
    }
  }, [handleLogout])

  const handleFooterReady = useCallback((root: HTMLDivElement) => {
    initHydroFooter(root)
  }, [])

  const context = useMemo(
    () => layoutChromeContext(user, pageName),
    [pageName, user],
  )

  return (
    <>
      <PageLoader />
      <HydroTemplate
        template="partials/nav.html"
        className="hydro-template-contents"
        context={context}
        url={buildLayoutUrl}
        onReady={handleNavReady}
      />
      <div className="slideout-panel" id="panel">
        <div className="slideout-overlay" />
        <HydroTemplate
          template="partials/header_mobile.html"
          className="hydro-template-contents"
          context={context}
          url={buildLayoutUrl}
        />
        <div className="main" ref={mainRef}><Outlet /></div>
        <HydroTemplate
          template="partials/footer.html"
          className="hydro-template-contents"
          context={context}
          url={buildLayoutUrl}
          onReady={handleFooterReady}
        />
      </div>
      <NotificationsHost />
    </>
  )
}
