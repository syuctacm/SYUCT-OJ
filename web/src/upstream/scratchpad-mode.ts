// @ts-nocheck -- ported from @hydrooj/ui-default pages/problem_detail.page.tsx (AGPL-3.0).
import $ from 'jquery'
import 'jquery.easing'
import 'jquery.transit'

export class ProblemPageExtender {
  isExtended = false
  inProgress = false
  $content = $('.problem-content-container')
  $contentBound = this.$content.closest('.section')
  $scratchpadContainer = $('.scratchpad-container')
  originalOverflow = ''

  async extend() {
    if (this.inProgress || this.isExtended) return
    this.inProgress = true
    try {
      const bound = this.$contentBound.get(0)?.getBoundingClientRect()
      this.$content.transition({ opacity: 0 }, { duration: 100 })
      await new Promise((resolve) => window.setTimeout(resolve, 100))
      $('body').addClass('header--collapsed mode--scratchpad')
      if (bound) {
        this.$scratchpadContainer.css({
          left: bound.left,
          top: bound.top,
          width: bound.width,
          height: bound.height,
        })
      }
      await this.$scratchpadContainer.show().transition({
        left: 0,
        top: 0,
        width: '100%',
        height: '100%',
      }, {
        duration: 500,
        easing: 'easeOutCubic',
      }).promise()
      $('.main > .row').hide()
      $('.footer').hide()
      $(window).scrollTop(0)
      this.originalOverflow = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      this.isExtended = true
    } finally {
      this.inProgress = false
    }
  }

  async collapse() {
    if (this.inProgress || !this.isExtended) return
    this.inProgress = true
    try {
      $(window).scrollTop(0)
      $('.main > .row').show()
      $('.footer').show()
      const bound = this.$contentBound.get(0)?.getBoundingClientRect()
      $('body').removeClass('header--collapsed mode--scratchpad')
      if (bound) {
        await this.$scratchpadContainer.transition({
          left: bound.left,
          top: bound.top,
          width: bound.width,
          height: bound.height,
        }, {
          duration: 500,
          easing: 'easeOutCubic',
        }).promise()
      }
      this.$scratchpadContainer.hide()
      this.$content.transition({ opacity: 1 }, { duration: 100 })
      document.body.style.overflow = this.originalOverflow
      this.isExtended = false
    } finally {
      this.inProgress = false
    }
  }
}

export async function fadeInScratchpad() {
  await $('#scratchpad').transition({ opacity: 1 }, 200, 'easeOutCubic').promise()
}

export async function fadeOutScratchpad() {
  await $('#scratchpad').transition({ opacity: 0 }, 200, 'easeOutCubic').promise()
}
