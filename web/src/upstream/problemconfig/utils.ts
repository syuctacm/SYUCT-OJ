// @ts-nocheck
const TIME_RE = /^([0-9]+(?:\.[0-9]*)?)([mu]?)s?$/i
const TIME_UNITS: Record<string, number> = { '': 1000, m: 1, u: 0.001 }
const MEMORY_RE = /^([0-9]+(?:\.[0-9]*)?)([kmg])b?$/i
const MEMORY_UNITS: Record<string, number> = { k: 1 / 1024, m: 1, g: 1024 }

export function parseTimeMS(value: string | number, throwOnError = true) {
  if (typeof value === 'number' || Number.isSafeInteger(+value)) return +value
  const match = TIME_RE.exec(value)
  if (!match && throwOnError) throw new Error(`${value} error parsing time`)
  if (!match) return 1000
  return Math.floor(Number.parseFloat(match[1]) * TIME_UNITS[match[2].toLowerCase()])
}

export function parseMemoryMB(value: string | number, throwOnError = true) {
  if (typeof value === 'number' || Number.isSafeInteger(+value)) return +value
  const match = MEMORY_RE.exec(value)
  if (!match && throwOnError) throw new Error(`${value} error parsing memory`)
  if (!match) return 256
  return Math.ceil(Number.parseFloat(match[1]) * MEMORY_UNITS[match[2].toLowerCase()])
}

const sortParts = /\D+|\d+/g

export function sortFiles<T extends Record<string, unknown>>(files: T[], key = '_id'): T[]
export function sortFiles(files: string[]): string[]
export function sortFiles(files: Array<string | Record<string, unknown>>, key = '_id') {
  if (!files?.length) return []
  const isString = typeof files[0] === 'string'
  const result = files
    .map((item) => {
      if (isString) return { name: item as string, weights: (item as string).match(sortParts) || [] }
      const value = String(item[key] || item.name || '')
      return { item, weights: value.match(sortParts) || [] }
    })
    .sort((a, b) => {
      let pos = 0
      let weightA = a.weights[pos]
      let weightB = b.weights[pos]
      while (weightA && weightB) {
        const numeric = Number(weightA) - Number(weightB)
        if (!Number.isNaN(numeric) && numeric !== 0) return numeric
        if (weightA !== weightB) return weightA > weightB ? 1 : -1
        pos += 1
        weightA = a.weights[pos]
        weightB = b.weights[pos]
      }
      return weightA ? 1 : -1
    })
  return result.map((entry) => (isString ? entry.name : entry.item))
}
