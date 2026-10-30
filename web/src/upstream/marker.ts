// @ts-nocheck -- ported from @hydrooj/ui-default components/marker (AGPL-3.0).
// The selection toolbar, colors and close thresholds stay identical upstream.
import $ from 'jquery'
import _ from 'lodash'

const MARKER_ID = `marker_${Math.floor(Math.random() * 0xFFFFFF).toString(16)}`
const MARKER_OFFSET = 20
const MARKER_MAX_DISTANCE = 60

let markerInstance: Marker | null = null

function delay(ms: number) {
  return new Promise<void>((resolve) => window.setTimeout(resolve, ms))
}

function distanceToRect(px: number, py: number, rect: DOMRect) {
  const cx = (rect.left + rect.right) / 2
  const cy = (rect.top + rect.bottom) / 2
  const dx = Math.max(Math.abs(px - cx) - (rect.right - rect.left) / 2, 0)
  const dy = Math.max(Math.abs(py - cy) - (rect.bottom - rect.top) / 2, 0)
  return Math.sqrt(dx * dx + dy * dy)
}

class Marker {
  static exists() {
    return markerInstance && document.getElementById(MARKER_ID)
  }

  $dom: JQuery<HTMLElement>
  $container: JQuery<HTMLElement> | null = null
  isOpen = false
  bindedHandlers = false

  constructor() {
    if (Marker.exists()) return markerInstance
    markerInstance?.destroy()
    this.$dom = $(`
      <div class="marker" id="${MARKER_ID}"><div class="marker__toolbar">
        <div class="marker__action" data-color="#ffff00" data-tooltip="标记黄色"><span class="marker__icon icon-yellow"></span></div>
        <div class="marker__action" data-color="#47ff6f" data-tooltip="标记绿色"><span class="marker__icon icon-green"></span></div>
        <div class="marker__action" data-color="#ff3b30" data-tooltip="标记红色"><span class="marker__icon icon-red"></span></div>
        <div class="marker__action" data-color="transparent" data-tooltip="清除标记"><span class="icon icon-erase"></span></div>
      </div></div>
    `).appendTo('body')
    this.$dom.find('.marker__toolbar').on('click', '.marker__action', this.onMarkerActionClick.bind(this))
    this.$dom.on('mousedown', this.onMarkerMouseDown.bind(this))
    this._onKeyDown = this.onKeyDown.bind(this)
    this._onScroll = _.throttle(this.onScroll.bind(this), 50)
    this._onMouseDown = this.onMouseDown.bind(this)
    this._onMouseMove = _.throttle(this.onMouseMove.bind(this), 50)
    markerInstance = this
  }

  markSelection(color: string) {
    const selection = window.getSelection()
    if (!selection?.rangeCount) return
    const range = selection.getRangeAt(0)
    this.$container?.attr('contentEditable', true)
    selection.removeAllRanges()
    selection.addRange(range)
    if (!document.execCommand('HiliteColor', false, color)) {
      document.execCommand('BackColor', false, color)
    }
    this.$container?.removeAttr('contentEditable')
    selection.removeAllRanges()
  }

  showAtPosition($container: JQuery<HTMLElement>, x: number, y: number) {
    this.$container = $container
    this.$dom.css({ display: 'block', opacity: 0 })
    const rect = this.$dom[0].getBoundingClientRect()
    this.$dom.css({
      left: x + MARKER_OFFSET,
      top: y - (rect.bottom - rect.top) - MARKER_OFFSET,
    })
    this.isOpen = true
    this.$dom.addClass('open')
    this.updateMarkerOpacity(x, y)
    this.bindEventHandlersForClosing()
  }

  async close() {
    if (!this.isOpen) return
    this.isOpen = false
    this.unbindEventHandlers()
    this.$dom.css({ opacity: 0 }).removeClass('open')
    await delay(200)
    if (!this.isOpen) this.$dom.hide()
  }

  bindEventHandlersForClosing() {
    if (this.bindedHandlers) return
    $(document).on('keydown', this._onKeyDown)
    $(window).on('scroll', this._onScroll)
    $(document).on('mousedown', this._onMouseDown)
    $(document).on('mousemove', this._onMouseMove)
    this.bindedHandlers = true
  }

  unbindEventHandlers() {
    if (!this.bindedHandlers) return
    $(document).off('keydown', this._onKeyDown)
    $(window).off('scroll', this._onScroll)
    $(document).off('mousedown', this._onMouseDown)
    $(document).off('mousemove', this._onMouseMove)
    this.bindedHandlers = false
  }

  onMarkerActionClick(ev: JQuery.ClickEvent) {
    const color = $(ev.currentTarget).attr('data-color')
    if (color) this.markSelection(color)
    void this.close()
  }

  onMarkerMouseDown(ev: JQuery.MouseDownEvent) {
    ev.stopPropagation()
    ev.preventDefault()
  }

  onKeyDown() {
    void this.close()
  }

  onScroll() {
    void this.close()
  }

  onMouseMove(ev: JQuery.MouseMoveEvent) {
    this.updateMarkerOpacity(ev.clientX, ev.clientY)
  }

  onMouseDown() {
    void this.close()
  }

  updateMarkerOpacity(x: number, y: number) {
    const markerRect = this.$dom[0].getBoundingClientRect()
    const distance = distanceToRect(x, y, markerRect)
    if (distance > MARKER_MAX_DISTANCE) {
      void this.close()
      return
    }
    this.$dom.css('opacity', 1 - distance / MARKER_MAX_DISTANCE)
  }

  destroy() {
    void this.close()
    this.$dom.remove()
    markerInstance = null
  }
}

function showMarker($container: JQuery<HTMLElement>, x: number, y: number) {
  const marker = new Marker()
  marker.showAtPosition($container, x, y)
}

export function initHydroMarkers(root: ParentNode) {
  const elements = [
    ...(root instanceof Element && root.matches('[data-marker-enabled]') ? [root] : []),
    ...root.querySelectorAll<HTMLElement>('[data-marker-enabled]'),
  ]
  for (const element of elements) {
    if (element.dataset.hydroMarker) continue
    element.dataset.hydroMarker = '1'
    const $element = $(element)
    $element.on('mouseup', async (event: JQuery.MouseUpEvent) => {
      await delay(1)
      const selection = window.getSelection()
      if (!selection?.rangeCount) return
      if (selection.getRangeAt(0).collapsed) return
      showMarker($element, event.clientX, event.clientY)
    })
  }
}
