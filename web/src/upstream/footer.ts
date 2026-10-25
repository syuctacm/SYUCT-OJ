// @ts-nocheck -- ported from @hydrooj/ui-default components/footer/footer.page.ts (AGPL-3.0).
import $ from 'jquery'
import { slideDown, slideUp } from './motion.ts'

const MOBILE_CUTOFF = 600

export function initHydroFooter(root: ParentNode) {
  $(root).find('.footer__category.expandable > h1').off('click.hydroFooter').on('click.hydroFooter', async (event) => {
    if (window.innerWidth > MOBILE_CUTOFF) return
    const $category = $(event.currentTarget).closest('.footer__category')
    const $list = $category.find('.footer__category__expander')
    if ($category.hasClass('animating')) return
    $category.addClass('animating')
    if ($category.hasClass('expanded')) {
      $category.removeClass('expanded')
      await slideUp($list, 300, { opacity: 1 }, { opacity: 0 })
    } else {
      $category.addClass('expanded')
      await slideDown($list, 300, { opacity: 0 }, { opacity: 1 })
    }
    $category.removeClass('animating')
  })
}
