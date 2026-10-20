export type LegacyMoment = {
  format: (pattern: string) => string
  add: (amount: number, unit: 'hour' | 'hours' | 'day' | 'days') => LegacyMoment
  toDate: () => Date
  valueOf: () => number
}

function pad(value: number) {
  return String(value).padStart(2, '0')
}

export function legacyMoment(input: Date | string | number): LegacyMoment {
  const date = input instanceof Date ? new Date(input) : new Date(input)
  return {
    format(pattern) {
      const parts: Record<string, string> = {
        YYYY: String(date.getFullYear()),
        MM: pad(date.getMonth() + 1),
        M: String(date.getMonth() + 1),
        DD: pad(date.getDate()),
        D: String(date.getDate()),
        HH: pad(date.getHours()),
        H: String(date.getHours()),
        mm: pad(date.getMinutes()),
      }
      return pattern.replace(/YYYY|MM|DD|HH|mm|M|D|H/g, (token) => parts[token] ?? token)
    },
    add(amount, unit) {
      const next = new Date(date)
      if (unit === 'hour' || unit === 'hours') next.setHours(next.getHours() + amount)
      else next.setDate(next.getDate() + amount)
      return legacyMoment(next)
    },
    toDate: () => new Date(date),
    valueOf: () => date.getTime(),
  }
}
