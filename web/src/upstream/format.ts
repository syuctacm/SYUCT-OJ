// Copied from Hydro framework/utils/lib/common.ts so ui-default templates can
// call the same string formatting helpers they use on the server.
declare global {
  interface String {
    format(...args: unknown[]): string
    formatFromArray(args: unknown[]): string
  }
}

String.prototype.format ||= function formatStr(...args: unknown[]) {
  let result = this
  if (args.length) {
    if (args.length === 1 && typeof args[0] === 'object' && args[0] !== null) {
      const values = args[0] as Record<string, unknown>
      for (const key in values) {
        if (!key.startsWith('_') && values[key] !== undefined) {
          const reg = new RegExp(`(\\{${key}\\})`, 'g')
          result = result.replace(reg, String(values[key]))
        }
      }
    } else {
      return this.formatFromArray(args)
    }
  }
  return result.toString()
}

String.prototype.formatFromArray = function formatFromArray(args: unknown[]) {
  let result = this
  for (let i = 0; i < args.length; i++) {
    if (args[i] !== undefined) {
      const reg = new RegExp(`(\\{)${i}(\\})`, 'g')
      result = result.replace(reg, String(args[i]))
    }
  }
  return result.toString()
}

export function size(bytes: number, base = 1) {
  let value = bytes * base
  const unit = 1024
  const unitNames = ['Bytes', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB']
  for (const unitName of unitNames) {
    if (value < unit) return `{0} {1}`.format(Math.round(value * 10) / 10, unitName)
    value /= unit
  }
  return `${Math.round(value * unit)} ${unitNames.at(-1)}`
}
