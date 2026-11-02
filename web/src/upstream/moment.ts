export type LegacyMoment = {
  format: (pattern: string) => string
  add: (amount: number, unit: 'hour' | 'hours' | 'day' | 'days') => LegacyMoment
  toDate: () => Date
  valueOf: () => number
}

import { formatInTimeZone } from './timezone.ts'

export function legacyMoment(input: Date | string | number): LegacyMoment {
  const date = input instanceof Date ? new Date(input) : new Date(input)
  return {
    format(pattern) {
      return formatInTimeZone(date, pattern)
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
