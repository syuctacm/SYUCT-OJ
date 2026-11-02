// `datetimeSpan` derived from Hydro ui-default `backendlib/misc.ts` (AGPL-3.0).
// The moment token subset covers the patterns used by the ported templates.
import { formatInTimeZone } from './timezone.ts'

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
  return `<span class="time${relative ? ' relative' : ''}" data-timestamp="${date.getTime() / 1000}">${formatInTimeZone(date, format)}</span>`
}
