import $ from 'jquery'

import { installHydroMultiSelectCheckboxes } from './checkboxes.ts'
import { initHydroCollapsibleContent } from './collapsible.ts'
import { animateSections, notifyHydroPageReady } from './motion.ts'
import { installHydroForms } from './forms.ts'
import { initHydroHighlighter } from './highlighter.ts'
import { installHydroHotkeys } from './hotkeys.ts'
import { initHydroMarkers } from './marker.ts'
import { initHydroMenus } from './menu.ts'
import { installHydroFilePreview } from './preview.tsx'
import { cleanupHydroOverlays, installHydroOverlays } from './overlays.ts'
import { initHydroProblemList } from './problem-list.ts'
import { installHydroOmnisearch } from './omnisearch.tsx'
import { installHydroSmoothScroll } from './smoothscroll.ts'
import { cleanupHydroSticky, initHydroSticky, installHydroSticky, relayoutHydroSticky } from './sticky.ts'
import { initHydroTabs, installHydroTabListener } from './tabs.ts'
import { cleanupHydroTables, initHydroTables } from './table.ts'
import { installHydroTextboxFocus } from './textbox.ts'
import { initHydroRelativeTime, installHydroRelativeTimeRefresh } from './time.ts'

function isHydroPageLoading(main: HTMLElement) {
  const placeholder = main.querySelector('.nothing-placeholder')
  if (!placeholder) return false
  return /加载中|loading/i.test(placeholder.textContent ?? '')
}

let installed = false

export function installHydroRuntime(main: HTMLElement) {
  if (!installed) {
    installed = true
    installHydroHotkeys()
    installHydroMultiSelectCheckboxes()
    installHydroOverlays()
    installHydroOmnisearch()
    installHydroFilePreview()
    installHydroForms()
    installHydroSmoothScroll()
    void installHydroSticky()
    installHydroTabListener()
    installHydroTextboxFocus()
    installHydroRelativeTimeRefresh()
  }

  let frame = 0
  let running = false
  let reschedule = false

  async function hydrate() {
    if (running) {
      reschedule = true
      return
    }
    running = true
    try {
      if (isHydroPageLoading(main)) return
      initHydroProblemList(main)
      await initHydroHighlighter(main)
      initHydroRelativeTime(main)
      initHydroMarkers(main)
      initHydroCollapsibleContent(main)
      initHydroTabs(main)
      initHydroMenus(main)
      await initHydroTables(main)
      await initHydroSticky(main)
      await animateSections(main.querySelectorAll('.section:not(.visible)'))
      relayoutHydroSticky()
      $(main).trigger('vjContentNew')
      notifyHydroPageReady(main)
    } finally {
      running = false
      if (reschedule) {
        reschedule = false
        schedule()
      }
    }
  }

  function schedule() {
    window.cancelAnimationFrame(frame)
    frame = window.requestAnimationFrame(() => { void hydrate() })
  }

  const observer = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      for (const node of mutation.removedNodes) {
        if (node instanceof Element) {
          cleanupHydroSticky(node)
          cleanupHydroTables(node)
          cleanupHydroOverlays(node)
        }
      }
    }
    const affectsLoadingPlaceholder = mutations.some((mutation) => (
      mutation.type === 'characterData'
      && mutation.target.parentElement?.closest('.nothing-placeholder')
    ))
    if (affectsLoadingPlaceholder || mutations.some((mutation) => mutation.addedNodes.length || mutation.removedNodes.length)) schedule()
  })
  observer.observe(main, { childList: true, characterData: true, subtree: true })
  schedule()

  return () => {
    observer.disconnect()
    window.cancelAnimationFrame(frame)
  }
}
