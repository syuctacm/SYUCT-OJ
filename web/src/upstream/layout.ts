import type { User } from '../api.ts'

export const LAYOUT_PRIV = {
  PRIV_USER_PROFILE: 1,
  PRIV_REGISTER_USER: 2,
  PRIV_CREATE_FILE: 3,
  PRIV_EDIT_SYSTEM: 4,
} as const

const NAV_ITEMS = [
  { name: 'homepage', displayName: '首页', args: { prefix: 'homepage' } },
  { name: 'problem_main', displayName: '题库', args: { prefix: 'problem_' } },
  { name: 'training_main', displayName: '训练', args: { prefix: 'training_' } },
  { name: 'contest_main', displayName: '比赛', args: { prefix: 'contest_' } },
  { name: 'homework_main', displayName: '作业', args: { prefix: 'homework_' } },
  { name: 'discussion_main', displayName: '讨论', args: { prefix: 'discussion_' } },
  { name: 'record_main', displayName: '评测记录', args: { prefix: 'record_' } },
]

function userHandler(user: User | null) {
  const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
  const uname = user?.displayName && user.displayName !== user.username
    ? `${user.displayName} (${user.username})`
    : user?.username || ''
  return {
    _id: user?.id ?? 0,
    uname,
    displayName: user?.displayName ?? '',
    viewLang: user?.viewLang || 'zh',
    hasPriv: (privilege: number) => {
      if (privilege === LAYOUT_PRIV.PRIV_USER_PROFILE) return Boolean(user)
      if (privilege === LAYOUT_PRIV.PRIV_REGISTER_USER) return !user
      if (privilege === LAYOUT_PRIV.PRIV_CREATE_FILE) return Boolean(user)
      if (privilege === LAYOUT_PRIV.PRIV_EDIT_SYSTEM) return isAdmin
      return false
    },
    hasPerm: () => false,
    own: () => false,
  }
}

export function layoutChromeContext(user: User | null, pageName: string) {
  const handlerUser = userHandler(user)
  return {
    page_name: pageName,
    overrideNav: NAV_ITEMS.map((item) => ({ ...item, checker: () => true })),
    handler: {
      user: handlerUser,
      domain: { name: 'SYUCT ACM', avatar: '', ui: {} },
      args: { domainId: 'system' },
      request: { query: {}, headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: user?.viewLang || 'zh' },
    },
    model: {
      system: { get: (key: string) => key === 'server.login' },
      setting: { SETTINGS_BY_KEY: { viewLang: { range: { zh: '简体中文' } } } },
    },
    PRIV: LAYOUT_PRIV,
    ctx: {
      setting: {
        get: (key: string) => key === 'ui-default.nav_logo_dark' ? '/nav_logo_dark.png' : '',
      },
    },
    global: { Hydro: { version: { hydrooj: '4.58.5' } } },
    UserContext: user,
  }
}

export function buildLayoutUrl(name: string, args: Record<string, unknown> = {}) {
  switch (name) {
    case 'homepage': return '/'
    case 'problem_main': return '/p'
    case 'training_main': return '/training'
    case 'contest_main': return '/contest'
    case 'homework_main': return '/homework'
    case 'discussion_main': return '/discuss'
    case 'record_main': return '/record'
    case 'user_login': return '/login'
    case 'user_register': return '/register'
    case 'user_logout': return '/logout'
    case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
    case 'home_messages': return '/home/messages'
    case 'home_security': return '/home/security'
    case 'home_files': return '/file'
    case 'home_settings':
      return `/home/settings?category=${encodeURIComponent(String(args.category ?? 'account'))}`
    case 'manage_dashboard': return '/admin'
    case 'manage_user_priv': return '/admin/users'
    case 'manage_user_import': return '/admin/users/import'
    case 'manage_setting': return '/admin/settings'
    case 'wiki_help':
    case 'wiki_about':
      return '/discuss'
    default:
      return '/'
  }
}
