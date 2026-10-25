// @ts-nocheck -- ported from @hydrooj/ui-default entry.js/hydro.ts page-loader behavior.
import $ from 'jquery'
import { useLayoutEffect, useRef } from 'react'

export function PageLoader() {
  const ref = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    const $loader = $(ref.current)
    $loader.fadeIn(500)
    const finish = () => {
      $loader.fadeOut(200)
    }
    $(document).one('vjPageFullyInitialized.hydroPageLoader', finish)
    return () => {
      $(document).off('vjPageFullyInitialized.hydroPageLoader', finish)
      $loader.stop(true, true).hide()
    }
  }, [])

  return <div ref={ref} className="page-loader nojs--hide" style={{ display: 'none' }}><div className="loader" /></div>
}
