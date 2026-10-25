// @ts-nocheck -- ported from @hydrooj/ui-default components/menu/menu.page.js (AGPL-3.0).
import $ from 'jquery'

import { slideDown } from './motion.ts'

export function initHydroMenus(root: ParentNode) {
  window.setTimeout(() => {
    $(root).find('.menu.collapsed').each((_, menu) => {
      if ($(menu).attr('data-hydro-menu-expanded') === '1') return
      $(menu).attr('data-hydro-menu-expanded', '1')
      void slideDown($(menu), 500, { opacity: 0 }, { opacity: 1 })
    })
  }, 200)
}
