export function legacyDomainTarget(pathname: string, search: string, hash: string) {
  const match = /^\/d\/[^/]+(\/.*)?$/.exec(pathname)
  const rest = match?.[1] || '/'
  const target = rest === '/' ? '/' : rest
  return `${target}${search}${hash}`
}
