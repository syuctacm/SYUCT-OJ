// Ported from Hydro ui-default components/clipboard.page.ts (AGPL-3.0).
import { notifyError, notifySuccess } from '../components/Notification.tsx'
import { translate } from './render.ts'

function substitute(template: string, data: string) {
  return template.replace('{data}', data)
}

function decode(value: string) {
  try {
    return atob(value)
  } catch {
    return ''
  }
}

export function mountClipboard(root: HTMLElement) {
  const onClick = async (event: Event) => {
    const target = (event.target as HTMLElement).closest<HTMLElement>('[data-copy]')
    if (!target) return
    const encoded = target.getAttribute('data-copy') ?? ''
    const text = encoded ? decode(encoded) : ''
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      notifySuccess(substitute(translate('"{data}" copied to clipboard!'), text), 2000)
    } catch {
      notifyError(substitute(translate('Copy "{data}" failed :('), text))
    }
  }
  root.addEventListener('click', onClick)
  return () => root.removeEventListener('click', onClick)
}
