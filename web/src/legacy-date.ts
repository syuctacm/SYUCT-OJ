export function dateParts(value: string | Date) {
  const date = typeof value === 'string' ? new Date(value) : value
  return {
    date: `${date.getFullYear()}-${date.getMonth() + 1}-${date.getDate()}`,
    time: `${date.getHours()}:${String(date.getMinutes()).padStart(2, '0')}`,
  }
}

export function parseDateTime(dateText: string, timeText: string): Date | null {
  const dateMatch = /^(\d{4})-(\d{1,2})-(\d{1,2})$/.exec(dateText.trim())
  const timeMatch = /^(\d{1,2}):(\d{2})$/.exec(timeText.trim())
  if (!dateMatch || !timeMatch) return null
  const year = Number(dateMatch[1])
  const month = Number(dateMatch[2])
  const day = Number(dateMatch[3])
  const hour = Number(timeMatch[1])
  const minute = Number(timeMatch[2])
  const parsed = new Date(year, month - 1, day, hour, minute, 0, 0)
  if (
    parsed.getFullYear() !== year
    || parsed.getMonth() !== month - 1
    || parsed.getDate() !== day
    || parsed.getHours() !== hour
    || parsed.getMinutes() !== minute
  ) return null
  return parsed
}

export function formatEndTime(beginAt: Date, durationHours: number) {
  const end = new Date(beginAt.getTime() + durationHours * 3600000)
  const { date, time } = dateParts(end)
  return `${date} ${time}`
}

export function addDays(value: Date, days: number) {
  return new Date(value.getTime() + days * 86400000)
}
