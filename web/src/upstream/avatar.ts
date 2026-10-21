// Ported from Hydro packages/hydrooj/src/lib/avatar.ts (AGPL-3.0). User avatars
// are stored as `provider:value` strings; templates call avatarUrl() to turn
// them into a real image URL.
import { md5 } from './md5.ts'

// Upstream default of the `avatar.gravatar_url` system setting.
const GRAVATAR_URL = '//cn.gravatar.com/avatar/'

const PROVIDERS: Record<string, (source: string, size: number) => string> = {
  gravatar: (email, size) => `${GRAVATAR_URL}${md5(String(email ?? '').trim().toLowerCase())}?d=mm&s=${size || 32}`,
  qq: (id) => `//q1.qlogo.cn/g?b=qq&nk=${(/(\d+)/.exec(id) || ['', ''])[1]}&s=160`,
  github: (id, size) => `//github.com/${id}.png?size=${Math.min(size, 460)}`,
  url: (url) => url,
}

export function avatarUrl(source?: string | null, size = 64, fallback = ''): string {
  let value = source ?? ''
  if (!value) value = fallback
  let index = value.indexOf(':')
  if (index === -1 && fallback) {
    value = fallback
    index = value.indexOf(':')
  }
  if (index === -1) return PROVIDERS.gravatar('', size)
  const provider = value.substring(0, index)
  const payload = value.substring(index + 1)
  if (!PROVIDERS[provider] || !payload) return PROVIDERS.gravatar('', size)
  return PROVIDERS[provider](payload, size)
}

export function isValidAvatar(input: string): boolean {
  if (!input) return true
  if (input.startsWith('url:')) return true
  if (input.startsWith('github:')) return /^[a-zA-Z0-9-]+$/.test(input.substring(7))
  if (input.startsWith('qq:')) return /^[1-9]\d{4,}$/.test(input.substring(3))
  if (input.startsWith('gravatar:')) return /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(input.substring(9))
  return false
}
