import $ from 'jquery'
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef } from 'react'
import { Outlet, useLocation, useNavigationType, useNavigate } from 'react-router-dom'
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
import { SignInDialog } from './SignInDialog.tsx'

export function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const navigationType = useNavigationType()
  const mainRef = useRef<HTMLDivElement>(null)
  const scrollPositions = useRef(new Map<string, number>())
  const activeLocationKey = useRef(location.key)
  const pendingScrollRestore = useRef<number | null>(null)
  const firstLocation = useRef(true)
  const pageName = pageMetaForPath(location.pathname).name

  useLayoutEffect(() => {
    applyPageMeta(pageMetaForPath(location.pathname))
  }, [location.pathname])

  useEffect(() => {
    const previous = history.scrollRestoration
    history.scrollRestoration = 'manual'
    return () => {
      history.scrollRestoration = previous
    }
  }, [])

  useEffect(() => {
    let frame = 0
    const saveCurrent = () => {
      scrollPositions.current.set(activeLocationKey.current, window.scrollY)
    }
    const saveScroll = () => {
      window.cancelAnimationFrame(frame)
      frame = window.requestAnimationFrame(() => {
        if (activeLocationKey.current === location.key) {
          scrollPositions.current.set(location.key, window.scrollY)
        }
      })
    }
    saveScroll()
    document.addEventListener('click', saveCurrent, true)
    window.addEventListener('scroll', saveScroll, { passive: true })
    return () => {
      window.cancelAnimationFrame(frame)
      document.removeEventListener('click', saveCurrent, true)
      window.removeEventListener('scroll', saveScroll)
    }
  }, [location.key])

  useLayoutEffect(() => {
    activeLocationKey.current = location.key
    if (firstLocation.current) {
      firstLocation.current = false
      return
    }
    if (!location.hash) {
      $(document).trigger('vjPageLoading')
    }
    if (location.hash) {
      pendingScrollRestore.current = null
      return
    }
    if (navigationType === 'POP') {
      const top = scrollPositions.current.get(location.key) ?? 0
      pendingScrollRestore.current = top > 0 ? top : null
      return
    }
    pendingScrollRestore.current = null
    const frame = window.requestAnimationFrame(() => {
      window.scrollTo({ top: 0, left: 0, behavior: 'auto' })
    })
    return () => window.cancelAnimationFrame(frame)
  }, [location.hash, location.key, location.pathname, location.search, navigationType])

  useEffect(() => {
    const restore = () => {
      const top = pendingScrollRestore.current
      if (top === null) return
      pendingScrollRestore.current = null
      window.requestAnimationFrame(() => {
        window.scrollTo({ top, left: 0, behavior: 'auto' })
      })
    }
    $(document).on('vjPageFullyInitialized.hydroScrollRestore', restore)
    return () => {
      $(document).off('vjPageFullyInitialized.hydroScrollRestore', restore)
    }
  }, [])

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
      {!user ? <SignInDialog /> : null}
    </>
  )
}
