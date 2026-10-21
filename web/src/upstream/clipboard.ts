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
    const link = (event.target as HTMLElement).closest<HTMLElement>('[data-copylink]')
    const target = link ?? (event.target as HTMLElement).closest<HTMLElement>('[data-copy]')
    if (!target) return
    const text = link
      ? new URL(link.getAttribute('data-copylink') ?? '', document.location.href).toString()
      : decode(target.getAttribute('data-copy') ?? '')
    if (!text) return
    if (link) event.preventDefault()
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
