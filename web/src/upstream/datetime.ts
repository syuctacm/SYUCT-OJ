// `datetimeSpan` derived from Hydro ui-default `backendlib/misc.ts` (AGPL-3.0).
// The moment token subset covers the patterns used by the ported templates.

function pad(value: number) {
  return String(value).padStart(2, '0')
}

function formatDate(date: Date, pattern: string) {
  const parts: Record<string, string> = {
    YYYY: String(date.getFullYear()),
    MM: pad(date.getMonth() + 1),
    M: String(date.getMonth() + 1),
    DD: pad(date.getDate()),
    D: String(date.getDate()),
    HH: pad(date.getHours()),
    H: String(date.getHours()),
    mm: pad(date.getMinutes()),
    m: String(date.getMinutes()),
    ss: pad(date.getSeconds()),
    s: String(date.getSeconds()),
  }
  return pattern.replace(/YYYY|MM|DD|HH|mm|ss|M|D|H|m|s/g, (token) => parts[token] ?? token)
}

export function datetimeSpan(
  value: Date | string | number | null | undefined,
  relative = true,
  format = 'YYYY-M-D H:mm:ss',
) {
  if (!value) return 'DATETIME_SPAN_ERROR'
  let date: Date
  if (value instanceof Date) {
    date = value
  } else if (typeof value === 'string' && /^[0-9a-fA-F]{24}$/.test(value)) {
    // ObjectId hex strings carry the submit time in their first 4 bytes,
    // matching Hydro's `datetimeSpan(ObjectId)`.
    date = new Date(Number.parseInt(value.slice(0, 8), 16) * 1000)
  } else {
    date = new Date(value)
  }
  if (Number.isNaN(date.getTime())) return 'DATETIME_SPAN_ERROR'
  return `<span class="time${relative ? ' relative' : ''}" data-timestamp="${date.getTime() / 1000}">${formatDate(date, format)}</span>`
}
