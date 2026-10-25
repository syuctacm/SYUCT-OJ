// @ts-nocheck -- ported from @hydrooj/ui-default components/hotkey/hotkey.page.js (AGPL-3.0).
import $ from 'jquery'
import _ from 'lodash'

const KEY_MAP: Record<number, string> = {
  10: 'enter', 13: 'enter', 27: 'esc', 37: 'left', 38: 'up', 39: 'right', 40: 'down',
  112: 'f1', 113: 'f2', 114: 'f3', 115: 'f4', 116: 'f5', 117: 'f6', 118: 'f7',
  119: 'f8', 120: 'f9', 121: 'f10', 122: 'f11', 123: 'f12',
}

let installed = false

function isHotkeyMatch(sortedHotkey: string[], definition: string) {
  return _.isEqual(sortedHotkey, definition.toLowerCase().split('+').sort())
}

function testElementHotkey(hotkey: string[], element: Element, attr: string) {
  const $element = $(element)
  if (!$element.is(':visible')) return false
  let triggered = false
  for (const singleDefinition of String($element.attr(attr)).split(',')) {
    const [definition, trigger] = singleDefinition.split(':')
    if (!isHotkeyMatch(hotkey, definition)) continue
    triggered = true
    if (trigger === 'submit') $element.closest('form').trigger('submit')
    else if (trigger === undefined) $element.trigger('click')
    else $element.trigger(trigger)
  }
  return triggered
}

export function installHydroHotkeys() {
  if (installed) return
  installed = true
  $(document).on('keydown.hydroHotkeys', (event) => {
    const hotkey = ['alt', 'ctrl', 'shift'].filter((key) => event[`${key}Key`])
    if (event.metaKey && !event.ctrlKey) hotkey.push('ctrl')
    if (KEY_MAP[event.which] !== undefined) hotkey.push(KEY_MAP[event.which])
    else hotkey.push(String.fromCharCode(event.which).toLowerCase())
    hotkey.sort()

    let triggered = false
    $('[data-global-hotkey]').each((_, element) => {
      triggered = testElementHotkey(hotkey, element, 'data-global-hotkey') || triggered
    })
    $(event.target).parents('[data-hotkey]').each((_, element) => {
      triggered = testElementHotkey(hotkey, element, 'data-hotkey') || triggered
    })
    if (triggered) event.preventDefault()
  })
}
