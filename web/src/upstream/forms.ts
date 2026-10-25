// @ts-nocheck -- ported from @hydrooj/ui-default components/form/form.page.ts (AGPL-3.0).
import $ from 'jquery'

let installed = false

export function installHydroForms() {
  if (installed) return
  installed = true
  $(document).on('vjFormDisableUpdate.hydroForms', 'input, select, textarea', (event) => {
    const $input = $(event.currentTarget)
    const $formItem = $input.closest('.form__item')
    $formItem[$input.prop('disabled') ? 'addClass' : 'removeClass']('is--disabled')
  })

  const submitting = new WeakSet<Element>()
  $(document).on('click.hydroForms', '[type="submit"]', (event) => {
    if (submitting.has(event.currentTarget)) event.preventDefault()
    submitting.add(event.currentTarget)
    window.setTimeout(() => submitting.delete(event.currentTarget), 5000)
  })
}
