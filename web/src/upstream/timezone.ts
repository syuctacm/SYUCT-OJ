export const DEFAULT_TIME_ZONE = 'Asia/Shanghai'

export type ZonedDateParts = {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  second: number
}

const formatterCache = new Map<string, Intl.DateTimeFormat>()

function formatterFor(timeZone: string) {
  let formatter = formatterCache.get(timeZone)
  if (!formatter) {
    formatter = new Intl.DateTimeFormat('en-US', {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hourCycle: 'h23',
    })
    formatterCache.set(timeZone, formatter)
  }
  return formatter
}

export function currentTimeZone() {
  return globalThis.UserContext?.timeZone || DEFAULT_TIME_ZONE
}

export function zonedDateParts(date: Date, timeZone = currentTimeZone()): ZonedDateParts {
  try {
    const values = Object.fromEntries(
      formatterFor(timeZone).formatToParts(date)
        .filter((part) => part.type !== 'literal')
        .map((part) => [part.type, Number(part.value)]),
    ) as Record<string, number>
    return {
      year: values.year,
      month: values.month,
      day: values.day,
      hour: values.hour === 24 ? 0 : values.hour,
      minute: values.minute,
      second: values.second,
    }
  } catch {
    return {
      year: date.getFullYear(),
      month: date.getMonth() + 1,
      day: date.getDate(),
      hour: date.getHours(),
      minute: date.getMinutes(),
      second: date.getSeconds(),
    }
  }
}

function pad(value: number) {
  return String(value).padStart(2, '0')
}

export function formatInTimeZone(date: Date, pattern: string, timeZone = currentTimeZone()) {
  const parts = zonedDateParts(date, timeZone)
  const values: Record<string, string> = {
    YYYY: String(parts.year),
    MM: pad(parts.month),
    M: String(parts.month),
    DD: pad(parts.day),
    D: String(parts.day),
    HH: pad(parts.hour),
    H: String(parts.hour),
    mm: pad(parts.minute),
    m: String(parts.minute),
    ss: pad(parts.second),
    s: String(parts.second),
  }
  return pattern.replace(/YYYY|MM|DD|HH|mm|ss|M|D|H|m|s/g, (token) => values[token] ?? token)
}

export function zonedInputParts(date: Date, timeZone = currentTimeZone()) {
  return {
    date: formatInTimeZone(date, 'YYYY-MM-DD', timeZone),
    time: formatInTimeZone(date, 'HH:mm', timeZone),
  }
}

export function parseZonedDateTime(dateText: string, timeText: string, timeZone = currentTimeZone()) {
  const dateMatch = /^(\d{4})-(\d{1,2})-(\d{1,2})$/.exec(dateText.trim())
  const timeMatch = /^(\d{1,2}):(\d{2})(?::(\d{2}))?$/.exec(timeText.trim() || '00:00')
  if (!dateMatch || !timeMatch) return new Date(Number.NaN)
  const desired = Date.UTC(
    Number(dateMatch[1]), Number(dateMatch[2]) - 1, Number(dateMatch[3]),
    Number(timeMatch[1]), Number(timeMatch[2]), Number(timeMatch[3] ?? 0),
  )
  let guess = desired
  for (let attempt = 0; attempt < 4; attempt += 1) {
    const actual = zonedDateParts(new Date(guess), timeZone)
    const rendered = Date.UTC(actual.year, actual.month - 1, actual.day, actual.hour, actual.minute, actual.second)
    const correction = desired - rendered
    if (correction === 0) break
    guess += correction
  }
  return new Date(guess)
}

export function addZonedDays(date: Date, days: number, timeZone = currentTimeZone()) {
  const parts = zonedDateParts(date, timeZone)
  const target = new Date(Date.UTC(parts.year, parts.month - 1, parts.day + days))
  const dateText = `${target.getUTCFullYear()}-${pad(target.getUTCMonth() + 1)}-${pad(target.getUTCDate())}`
  const timeText = `${pad(parts.hour)}:${pad(parts.minute)}:${pad(parts.second)}`
  return parseZonedDateTime(dateText, timeText, timeZone)
}

export function localeStringInTimeZone(
  date: Date,
  locale = 'zh-CN',
  options: Intl.DateTimeFormatOptions = { hour12: false },
  timeZone = currentTimeZone(),
) {
  try {
    return date.toLocaleString(locale, { ...options, timeZone })
  } catch {
    return date.toLocaleString(locale, options)
  }
}
