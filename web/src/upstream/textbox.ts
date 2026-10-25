// @ts-nocheck -- ported from @hydrooj/ui-default components/form/textbox.page.ts (AGPL-3.0).
import $ from 'jquery'

let installed = false

export function installHydroTextboxFocus() {
  if (installed) return
  installed = true
  $(document).on('focusin.hydroTextbox', '.textbox.material input', (event) => {
    $(event.currentTarget).parent().addClass('focus')
  })
  $(document).on('focusout.hydroTextbox', '.textbox.material input', (event) => {
    $(event.currentTarget).parent().removeClass('focus')
  })
}
