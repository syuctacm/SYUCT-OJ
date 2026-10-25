// @ts-nocheck -- ported from @hydrooj/ui-default components/sticky/sticky.page.js (AGPL-3.0).
import $ from 'jquery'

const MOBILE_CUTOFF = 600
const DESKTOP_CUTOFF = 1000
const stickies = new Map<Element, { cutoff: number; enabled: boolean }>()
let installed = false
let stickyReady: Promise<void> | null = null

function cutoffFor(value: string | undefined) {
  if (value === 'medium') return MOBILE_CUTOFF
  if (value === 'large') return DESKTOP_CUTOFF
  return 0
}

function updateStickies() {
  const navHeight = window.innerWidth <= MOBILE_CUTOFF ? 0 : $('.nav').height() || 0
  for (const [element, state] of stickies) {
    const $sticky = $(element)
    const shouldEnable = window.innerWidth >= state.cutoff
    if (shouldEnable && !state.enabled) {
      const options: Record<string, unknown> = { offset_top: 10 + navHeight }
      const $parent = $sticky.closest('[data-sticky-parent]')
      if ($parent.length) options.parent = $parent
      $sticky.stick_in_parent(options)
      state.enabled = true
    } else if (!shouldEnable && state.enabled) {
      $sticky.trigger('sticky_kit:detach')
      state.enabled = false
    }
  }
}

function scheduleUpdate() {
  requestAnimationFrame(updateStickies)
}

export async function initHydroSticky(root: ParentNode) {
  await stickyReady
  const elements = [
    ...(root instanceof Element && root.matches('[data-sticky]') ? [root] : []),
    ...root.querySelectorAll('[data-sticky]'),
  ]
  for (const element of elements) {
    if (stickies.has(element)) continue
    stickies.set(element, {
      cutoff: cutoffFor(element.getAttribute('data-sticky') || undefined),
      enabled: false,
    })
  }
  scheduleUpdate()
}

export function installHydroSticky() {
  if (installed) return stickyReady || Promise.resolve()
  installed = true
  window.$ = $
  window.jQuery = $
  stickyReady = Promise.resolve().then(() => import('./vendor/sticky-kit.js')).then(() => {
    let timer = 0
    window.addEventListener('resize', () => {
      window.clearTimeout(timer)
      timer = window.setTimeout(updateStickies, 300)
    })
    $(document).on('vjLayout.hydroSticky', scheduleUpdate)
  })
  return stickyReady
}

export async function ensureHydroStickyLoaded() {
  await installHydroSticky()
}

export function cleanupHydroSticky(root: ParentNode) {
  const elements = [
    ...(root instanceof Element && root.matches('[data-sticky]') ? [root] : []),
    ...root.querySelectorAll('[data-sticky]'),
  ]
  for (const element of elements) {
    const state = stickies.get(element)
    if (state?.enabled) $(element).trigger('sticky_kit:detach')
    stickies.delete(element)
  }
}

export function relayoutHydroSticky() {
  if (stickyReady) $('body').trigger('sticky_kit:recalc')
}
