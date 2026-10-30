const DEFAULT_LOGIN_TARGET = '/'

export function safeLoginRedirect(value: string | null | undefined): string {
  const target = String(value ?? '').trim()
  if (!target.startsWith('/') || target.startsWith('//')) return DEFAULT_LOGIN_TARGET
  if (target === '/login' || target.startsWith('/login?') || target.startsWith('/login#')) {
    return DEFAULT_LOGIN_TARGET
  }
  return target
}

export function loginPath(redirect: string): string {
  const target = safeLoginRedirect(redirect)
  return target === DEFAULT_LOGIN_TARGET
    ? '/login'
    : `/login?redirect=${encodeURIComponent(target)}`
}
