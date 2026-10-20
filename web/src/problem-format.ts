export function formatTimeLimit(value?: string) {
  if (!value) return ''
  return /^\d+(?:\.\d+)?$/.test(value) ? `${value}ms` : value
}

export function formatMemoryLimit(value?: string) {
  if (!value) return ''
  const match = value.match(/^(\d+(?:\.\d+)?)\s*(?:m|mb|mib)?$/i)
  return match ? `${match[1]}MiB` : value
}
