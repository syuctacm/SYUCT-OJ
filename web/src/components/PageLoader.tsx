// @ts-nocheck -- ported from @hydrooj/ui-default entry.js/hydro.ts page-loader behavior.
import $ from 'jquery'
import { useLayoutEffect, useRef } from 'react'

const LOADER_TIMEOUT = 15000

export function PageLoader() {
  const ref = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    const $loader = $(ref.current)
    let visible = false
    let timeout = 0
    const show = () => {
      visible = true
      window.clearTimeout(timeout)
      timeout = window.setTimeout(hide, LOADER_TIMEOUT)
      $loader.stop(true, true).fadeIn(200)
    }

    const hide = () => {
      if (!visible) return
      visible = false
      window.clearTimeout(timeout)
      $loader.stop(true, true).fadeOut(200)
    }

    $(document).on('vjPageLoading.hydroPageLoader', show)
    $(document).on('vjPageFullyInitialized.hydroPageLoader', hide)
    show()

    return () => {
      window.clearTimeout(timeout)
      $(document).off('vjPageLoading.hydroPageLoader', show)
      $(document).off('vjPageFullyInitialized.hydroPageLoader', hide)
      $loader.stop(true, true).hide()
    }
  }, [])

  return <div ref={ref} className="page-loader nojs--hide" style={{ display: 'none' }}><div className="loader" /></div>
}
