// Ported from Hydro ui-default components/time/time.page.js (AGPL-3.0).
// The locale wording uses Intl.RelativeTimeFormat to stay close to timeago.js.
const relativeFormatter = new Intl.RelativeTimeFormat('zh-CN', { numeric: 'auto' })

function relativeText(timestamp: number) {
  const diffSeconds = timestamp - Date.now() / 1000
  const absolute = Math.abs(diffSeconds)
  if (absolute < 45) return '刚刚'
  if (absolute < 90) return relativeFormatter.format(Math.sign(diffSeconds), 'minute')
  const minutes = Math.round(diffSeconds / 60)
  if (Math.abs(minutes) < 60) return relativeFormatter.format(minutes, 'minute')
  const hours = Math.round(minutes / 60)
  if (Math.abs(hours) < 24) return relativeFormatter.format(hours, 'hour')
  const days = Math.round(hours / 24)
  if (Math.abs(days) < 30) return relativeFormatter.format(days, 'day')
  const months = Math.round(days / 30)
  if (Math.abs(months) < 12) return relativeFormatter.format(months, 'month')
  return relativeFormatter.format(Math.round(months / 12), 'year')
}

function relativeTimeElements(root: ParentNode) {
  const elements = [
    ...(root instanceof HTMLElement && root.matches('span.time.relative[data-timestamp]') ? [root] : []),
    ...root.querySelectorAll<HTMLElement>('span.time.relative[data-timestamp]'),
  ]
  return elements
}

export function initHydroRelativeTime(root: ParentNode) {
  if (globalThis.UserContext?.showTimeAgo === false) return
  for (const element of relativeTimeElements(root)) {
    if (element.dataset.hasTimeago) continue
    const timestamp = Number(element.dataset.timestamp)
    if (!Number.isFinite(timestamp)) continue
    const absolute = element.textContent?.trim() || new Date(timestamp * 1000).toLocaleString('zh-CN', { hour12: false })
    element.dataset.hasTimeago = '1'
    element.dataset.tooltip = absolute
    element.setAttribute('datetime', new Date(timestamp * 1000).toISOString())
    const text = relativeText(timestamp)
    if (text) element.textContent = text
  }
}

let refreshInstalled = false

export function installHydroRelativeTimeRefresh() {
  if (refreshInstalled) return
  refreshInstalled = true
  window.setInterval(() => {
    for (const element of document.querySelectorAll<HTMLElement>('span.time.relative[data-timestamp][data-has-timeago]')) {
      const timestamp = Number(element.dataset.timestamp)
      if (!Number.isFinite(timestamp)) continue
      const text = relativeText(timestamp)
      if (text && element.textContent !== text) element.textContent = text
    }
  }, 60_000)
}
