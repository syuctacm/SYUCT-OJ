// @ts-nocheck -- ported from @hydrooj/ui-default components/discussion/collapsible.page.tsx (AGPL-3.0).
// Only the locale shim differs; thresholds, markup, animation and event handling stay upstream.
import 'jquery.easing'

import $ from 'jquery'

const labels = {
  'Show more': '显示更多',
  'Show less': '显示更少',
}

function i18n(key: string) {
  return labels[key] ?? key
}

export function initHydroCollapsibleContent(root: ParentNode = document) {
  $(root).find('.collapsible-content').each(function () {
    const $content = $(this)
    let $inner = $content.find('.collapsible-content__inner')
    if ($inner.length) return
    const $children = $content.contents().detach()
    $inner = $('<div class="collapsible-content__inner"></div>')
    $inner.append($children)
    $content.append($inner)
    $content.find('.collapsible-toggle').remove()
    const scrollHeight = $inner[0].scrollHeight
    const clientHeight = $inner[0].clientHeight
    if (scrollHeight > clientHeight + 10) {
      const $toggle = $(`<div class="collapsible-toggle">
        <a href="javascript:;" class="collapsible-toggle__link">${i18n('Show more')}</a>
      </div>`)
      $content.append($toggle)
      const $toggleLink = $toggle.find('.collapsible-toggle__link')
      requestAnimationFrame(() => {
        $toggle.addClass('visible')
      })
      $toggleLink.on('click', (event) => {
        event.preventDefault()
        if ($content.hasClass('expanded')) {
          const currentHeight = $inner[0].scrollHeight
          $inner.css('max-height', `${currentHeight}px`)
          requestAnimationFrame(() => {
            $content.removeClass('expanded')
            $inner.css('max-height', '')
            $toggleLink.text(i18n('Show more'))
          })
        } else {
          const currentMaxHeight = $inner.css('max-height')
          $inner.css('max-height', 'none')
          const fullHeight = $inner[0].scrollHeight
          $inner.css('max-height', currentMaxHeight)
          requestAnimationFrame(() => {
            $inner.css('max-height', `${fullHeight}px`)
            $content.addClass('expanded')
            $toggleLink.text(i18n('Show less'))
          })
        }
      })
    } else $content.addClass('expanded')
  })
}
