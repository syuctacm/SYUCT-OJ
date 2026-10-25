// @ts-nocheck -- vendored from @hydrooj/ui-default utils/slide.ts and hydro.ts (AGPL-3.0).
// The motion timings and easing must stay identical to the upstream site.
import 'jquery.easing'
import 'jquery.transit'

import $ from 'jquery'

const ANIMATION_STEP = 50
let transitionStyleInstalled = false

function delay(ms: number) {
  return new Promise<void>((resolve) => { window.setTimeout(resolve, ms) })
}

function installTransitionStyle() {
  if (transitionStyleInstalled) return
  transitionStyleInstalled = true
  const style = document.createElement('style')
  style.dataset.hydroMotion = 'sections'
  style.textContent = `.section {
    transition: transform .5s, opacity .5s;
    transition-timing-function: ease-out-cubic;
  }`
  document.head.append(style)
}

export async function animateSections(sections: Element[] | NodeListOf<Element>) {
  const items = Array.from(sections).filter((element) => !element.classList.contains('visible'))
  if (!items.length) return 0
  if (globalThis.UserContext?.skipAnimate) {
    items.forEach((element) => element.classList.add('visible'))
    return items.length
  }
  installTransitionStyle()
  for (const [index, element] of items.entries()) {
    element.classList.add('visible')
    if (index < 5) await delay(ANIMATION_STEP)
  }
  await delay(500)
  return items.length
}

export function notifyHydroPageReady(root: ParentNode = document.body) {
  $(root).trigger('vjLayout')
  $(document).trigger('vjPageFullyInitialized')
}

export async function slideDown(
  $element: JQuery<HTMLElement>,
  duration: number,
  fromCss: Record<string, unknown> = {},
  toCss: Record<string, unknown> = {},
) {
  const originalStyle = $element.attr('style') || ''
  $element.css({ position: 'absolute', visibility: 'none', display: 'block' })
  const height = $element.outerHeight()
  $element.attr('style', originalStyle)
  $element.css({ height: 0, overflow: 'hidden', display: 'block', ...fromCss })
  $element.height()
  await $element.transition({ height, ...toCss }, { duration, easing: 'easeOutCubic' }).promise()
  $element.attr('style', originalStyle)
  $element.css({ display: 'block' })
}

export async function slideUp(
  $element: JQuery<HTMLElement>,
  duration: number,
  fromCss: Record<string, unknown> = {},
  toCss: Record<string, unknown> = {},
) {
  const originalStyle = $element.attr('style') || ''
  const height = $element.outerHeight()
  $element.css({ height, overflow: 'hidden', display: 'block', ...fromCss })
  $element.height()
  await $element.transition({ height: 0, ...toCss }, { duration, easing: 'easeOutCubic' }).promise()
  $element.attr('style', originalStyle)
  $element.css({ display: 'none' })
}
