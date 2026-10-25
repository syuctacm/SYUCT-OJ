// @ts-nocheck -- ported from @hydrooj/ui-default dropdown and tooltip components (AGPL-3.0).
import $ from 'jquery'
import Drop from 'tether-drop'

const MOBILE_CUTOFF = 600
const tooltips = new WeakMap<Element, any>()
const dropdowns = new WeakMap<Element, any>()
let installed = false

function closestTooltipElement(target: Element | null) {
  for (let element = target, depth = 0; element && depth < 3; element = element.parentElement, depth++) {
    if (element.getAttribute('data-tooltip')) return element
  }
  return null
}

function constructTooltip(element: Element) {
  let instance = tooltips.get(element)
  if (instance) return instance
  instance = new Drop({
    target: element,
    classes: 'tooltip',
    position: element.getAttribute('data-tooltip-pos') || 'top center',
    openOn: 'hover',
    constrainToWindow: true,
    constrainToScrollParent: false,
    content: element.getAttribute('data-tooltip') || '',
  })
  instance.on('open', () => {
    if (instance.drop) instance.drop.style.zIndex = String(100000 + Date.now() % 1000)
  })
  tooltips.set(element, instance)
  return instance
}

function constructDropdown(element: Element) {
  let instance = dropdowns.get(element)
  if (instance) return instance
  if (element.hasAttribute('data-dropdown-trigger-desktop-only') && window.innerWidth <= MOBILE_CUTOFF) return null
  const selector = element.getAttribute('data-dropdown-target')
  const target = selector ? document.querySelector(selector) : null
  if (!target) return null
  instance = new Drop({
    target: element,
    classes: `dropdown ${element.getAttribute('data-dropdown-custom-class') || ''}`,
    content: target,
    position: element.getAttribute('data-dropdown-pos') || 'bottom left',
    openOn: 'hover',
    constrainToWindow: !element.hasAttribute('data-dropdown-disabledconstrainToWindow'),
    constrainToScrollParent: false,
  })
  dropdowns.set(element, instance)
  return instance
}

export function installHydroOverlays() {
  if (installed) return
  installed = true
  $(document).on('mouseover.hydroTooltip', (event) => {
    const element = closestTooltipElement(event.target)
    if (!element) return
    constructTooltip(element).open()
  })
  $(document).on('mouseenter.hydroDropdown', '[data-dropdown-target]', (event) => {
    constructDropdown(event.currentTarget)?.open()
  })
}

export function cleanupHydroOverlays(root: ParentNode) {
  const elements = [
    ...(root instanceof Element && (root.matches('[data-dropdown-target]') || root.matches('[data-tooltip]')) ? [root] : []),
    ...root.querySelectorAll('[data-dropdown-target], [data-tooltip]'),
  ]
  for (const element of elements) {
    const dropdown = dropdowns.get(element)
    if (dropdown) dropdown.destroy()
    dropdowns.delete(element)
    const tooltip = tooltips.get(element)
    if (tooltip) tooltip.destroy()
    tooltips.delete(element)
  }
}
