// @ts-nocheck -- ported from @hydrooj/ui-default components/table/StyledTable.js (AGPL-3.0).
import $ from 'jquery'
import _ from 'lodash'

import { ensureHydroStickyLoaded } from './sticky.ts'

const MOBILE_CUTOFF = 600

export async function initHydroTables(root: ParentNode) {
  // The sticky header copies the source table's classes. Exclude it here so
  // the SPA MutationObserver does not recursively wrap its own header.
  const tables = [
    ...(root instanceof Element && root.matches('.data-table:not(.section__table-header)') ? [root] : []),
    ...root.querySelectorAll('.data-table:not(.section__table-header)'),
  ]
  const pending = tables.filter((table) => (
    !(table as HTMLElement).dataset.hydroTable
    && table.closest('.section__body')
  ))
  if (!pending.length) return
  await ensureHydroStickyLoaded()
  for (const table of pending) {
    if (table.dataset.hydroTable) continue
    table.dataset.hydroTable = '1'
    const $table = $(table)
    const $container = $('<div class="section__table-container" />').insertBefore($table)
    const $header = $('<table />').attr('class', `${$table.attr('class') || ''} section__table-header`)
    $container.append($header).append($table)
    $table.children('colgroup').clone().appendTo($header)
    $table.children('thead').appendTo($header)
    const offsetTop = window.innerWidth <= MOBILE_CUTOFF ? 0 : ($('.nav').height() || 0)
    _.defer(() => $header.stick_in_parent({ parent: $container, offset_top: offsetTop }))
  }
}

export function cleanupHydroTables(root: ParentNode) {
  const tables = [
    ...(root instanceof Element && root.matches('.data-table:not(.section__table-header)') ? [root] : []),
    ...root.querySelectorAll('.data-table:not(.section__table-header)'),
  ]
  for (const table of tables) {
    const $header = table.closest('.section__table-container')?.querySelector('.section__table-header')
    if ($header) $($header).trigger('sticky_kit:detach')
  }
}
