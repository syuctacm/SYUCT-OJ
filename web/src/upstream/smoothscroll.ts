// @ts-nocheck -- ported from @hydrooj/ui-default components/smoothscroll/smoothscroll.page.js (AGPL-3.0).
import 'jquery.easing'

import $ from 'jquery'

const ANCHOR_REGEX = /^#[^ ]+$/
let installed = false

export function installHydroSmoothScroll() {
  if (installed) return
  installed = true
  const offsetHeight = () => 10 + ($('.nav').height() || 0)

  function scrollIfAnchor(href: string, pushToHistory = false) {
    if (!ANCHOR_REGEX.test(href)) return false
    const match = document.getElementById(href.slice(1))
    if (!match) return false
    const $content = $(match).closest('.collapsible-content')
    if ($content.length && !$content.hasClass('expanded')) {
      const $inner = $content.find('.collapsible-content__inner')
      $inner.scrollTop(0)
      $content.find('.collapsible-toggle__link').trigger('click')
      $content.addClass('expanded')
      $inner.css('max-height', 'none')
    }
    requestAnimationFrame(() => {
      const anchorOffset = window.scrollY + match.getBoundingClientRect().top - offsetHeight()
      $('html,body').animate({ scrollTop: anchorOffset }, 200, 'easeOutCubic')
    })
    if (pushToHistory) window.history.pushState({}, document.title, `${window.location.pathname}${href}`)
    return true
  }

  $(document).on('vjPageFullyInitialized.hydroSmoothScroll', () => {
    scrollIfAnchor(window.location.hash)
  })
  window.addEventListener('hashchange', () => scrollIfAnchor(window.location.hash))
  document.body.addEventListener('click', (event) => {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    const anchor = (event.target as Element)?.closest('a')
    const href = anchor?.getAttribute('href')
    if (href && scrollIfAnchor(href, true)) event.preventDefault()
  })
}
