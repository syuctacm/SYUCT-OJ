// @ts-nocheck -- ported from @hydrooj/ui-default components/form/multiSelectCheckbox.page.js (AGPL-3.0).
import $ from 'jquery'

const lastSelectionByGroup: Record<string, JQuery<HTMLElement>> = {}
let installed = false

function isInGroup(group: string) {
  return (_index: number, element: Element) => element.getAttribute('data-checkbox-group') === group
}

function onRangeRoleCheckboxClick(event: JQuery.ClickEvent) {
  const $current = $(event.currentTarget)
  const targetGroup = $current.attr('data-checkbox-group') ?? ''
  const $targets = $('[type="checkbox"]')
    .filter(isInGroup(targetGroup))
    .filter(':visible')
    .filter(':not(:disabled)')
  if (event.shiftKey && lastSelectionByGroup[targetGroup]) {
    const destCheck = lastSelectionByGroup[targetGroup].prop('checked')
    const from = $targets.index(lastSelectionByGroup[targetGroup])
    const to = $targets.index($current)
    const start = Math.min(from, to)
    const end = Math.max(from, to) + 1
    $targets.slice(start, end).prop('checked', destCheck)
  }
  lastSelectionByGroup[targetGroup] = $current
}

function onToggleRoleCheckboxClick(event: JQuery.ClickEvent) {
  const $current = $(event.currentTarget)
  const targetGroup = $current.attr('data-checkbox-toggle') ?? ''
  const $targets = $('[type="checkbox"]')
    .filter(isInGroup(targetGroup))
    .filter(':visible')
    .filter(':not(:disabled)')
  const destCheck = $current.prop('checked')
  $targets.prop('checked', destCheck)
}

export function installHydroMultiSelectCheckboxes() {
  if (installed) return
  installed = true
  $(document).on('click.hydroMultiSelect', '[data-checkbox-range]', onRangeRoleCheckboxClick)
  $(document).on('click.hydroMultiSelect', '[data-checkbox-toggle]', onToggleRoleCheckboxClick)
}
