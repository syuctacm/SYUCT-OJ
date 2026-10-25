// @ts-nocheck -- ported from @hydrooj/ui-default components/tab/Tab.js (AGPL-3.0).
import 'jquery.easing'
import 'jquery.transit'

import $ from 'jquery'

const TAB_TRANSITION_DURATION = 300
const tabInstances = new WeakMap<Element, HydroTabs>()
let listenerInstalled = false

class HydroTabs {
  constructor($dom: JQuery<HTMLElement>) {
    this.$source = $dom
    this.attached = false
    this.isAnimating = false
    this.currentIndex = 0
    this.hasMemo = false
    this.attach()
  }

  async switchToTab(idx: number) {
    if (idx === this.currentIndex || this.isAnimating) return
    const $tabs = this.$content.children()
    const $currentTab = $tabs.eq(this.currentIndex)
    const $newTab = $tabs.eq(idx)
    if (!$newTab.length) return

    this.$header.children().removeClass('selected').eq(idx).addClass('selected')
    this.isAnimating = true
    $newTab.addClass('active')
    const animateParameter: { from: string; to: string } = idx < this.currentIndex
      ? { from: '-100%', to: '0%' }
      : { from: '0%', to: '-100%' }
    $newTab.css('opacity', 0)
    this.$content.css('x', animateParameter.from).width()
    $currentTab.transition({ opacity: 0 }, { duration: TAB_TRANSITION_DURATION, easing: 'linear' })
    $newTab.transition({ opacity: 1 }, { duration: TAB_TRANSITION_DURATION, easing: 'linear' })
    await this.$content.transition({ x: animateParameter.to }, {
      duration: TAB_TRANSITION_DURATION,
      easing: 'easeOutCubic',
    }).promise()

    this.$content.children().eq(this.currentIndex).removeClass('active')
    this.$content.css('x', '0')
    $newTab.trigger('vjContentShow')
    this.currentIndex = idx
    if (this.hasMemo) location.hash = `#tab-${idx}`
    this.isAnimating = false
  }

  attach() {
    if (this.attached) return false
    const $container = this.$source.closest('.section__tab-container')
    const $headerWrapper = $(document.createElement('div'))
      .addClass('section__tab-header-wrapper')
      .appendTo($container)
    const $contentWrapper = $(document.createElement('div')).appendTo($container)
    this.$header = $(document.createElement('ul'))
      .addClass('section__tab-header')
      .attr('data-slideout-ignore', 'on')
      .appendTo($headerWrapper)
    this.$content = $(document.createElement('div'))
      .addClass('section__tab-content')
      .appendTo($contentWrapper)

    this.$source.find('.section__tab-title').each((idx, element) => {
      $(document.createElement('li')).text($(element).text())
        .addClass('section__tab-header-item')
        .attr('data-tab-index', idx)
        .appendTo(this.$header)
    })
    this.$source.find('.section__tab-main').appendTo(this.$content)
    this.$source.remove()
    this.$dom = $container

    this.hasMemo = this.$dom.hasClass('memo')
    this.currentIndex = 0
    if (this.hasMemo) {
      const getIndex = () => {
        const match = /^tab-(\d+)$/.exec(location.hash.substring(1))
        const idx = match ? +match[1] : null
        return idx !== null && !Number.isNaN(idx) && idx < this.$header.children().length ? idx : null
      }
      const handleHashChange = () => {
        const idx = getIndex()
        if (typeof idx === 'number') this.switchToTab(idx)
      }
      $(window).on('hashchange.hydroTabs', handleHashChange)
      handleHashChange()
    }
    this.$content.children().eq(this.currentIndex).addClass('active')
    this.$header.children().eq(this.currentIndex).addClass('selected')
    this.attached = true
    return true
  }
}

export function initHydroTabs(root: ParentNode) {
  for (const element of root.querySelectorAll('.section__tabs')) {
    const container = element.closest('.section__tab-container')
    if (!container || tabInstances.has(container)) continue
    tabInstances.set(container, new HydroTabs($(element)))
  }
}

export function installHydroTabListener() {
  if (listenerInstalled) return
  listenerInstalled = true
  $(document).on('click', '[data-tab-index]', (event) => {
    const container = $(event.currentTarget).closest('.section__tab-container').get(0)
    const instance = container ? tabInstances.get(container) : null
    if (instance) void instance.switchToTab(Number($(event.currentTarget).attr('data-tab-index')))
  })
}
