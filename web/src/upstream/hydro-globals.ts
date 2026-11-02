type HydroUserContext = {
  _id: number
  priv: number
  viewLang: string
  preferredEditorType: string
  fontFamily: string
  codeFontFamily: string
  codeLang?: string
  codeTemplate?: string
  skipAnimate?: boolean
  showTimeAgo?: boolean
  theme: 'light' | 'dark'
  rounded?: boolean
  showInvisibleChar?: boolean
  formatCode?: boolean
}

type HydroUserPreferences = {
  priv?: number
  viewLang?: string
  preferredEditorType?: 'sv' | 'monaco'
  fontFamily?: string
  codeFontFamily?: string
  codeLang?: string
  codeTemplate?: string
  skipAnimate?: boolean
  showTimeAgo?: boolean
  theme?: 'light' | 'dark'
  rounded?: boolean
  showInvisibleChar?: boolean
  formatCode?: boolean
}

declare global {
  var UserContext: HydroUserContext
}

const DEFAULT_BODY_FONT = '"Open Sans", "Seravek", "Segoe UI", "Verdana", "PingFang SC", "Hiragino Sans GB", "Lantinghei SC", "Microsoft Yahei", "WenQuanYi Micro Hei", sans-serif'
const DEFAULT_CODE_FONT = '"Source Code Pro", "Consolas", "Lucida Console", monospace'
const ROUNDED_STYLE_ID = 'hydro-rounded-style'

function fontStack(family: string | undefined, fallback: string, customFont?: string | null) {
  const families = [customFont, family]
    .map((value) => value?.trim())
    .filter((value): value is string => Boolean(value) && value !== 'sans')
    .map((value) => `"${value.replaceAll('"', '')}"`)
  return families.length ? `${[...new Set(families)].join(', ')}, ${fallback}` : fallback
}

function applyFontPreferences(fontFamily?: string, codeFontFamily?: string) {
  const customFont = globalThis.localStorage?.getItem('customFont')
  const customCodeFont = globalThis.localStorage?.getItem('customCodeFont')
  document.documentElement.style.setProperty('--font-family', fontStack(fontFamily, DEFAULT_BODY_FONT, customFont))
  document.documentElement.style.setProperty('--code-font-family', fontStack(codeFontFamily, DEFAULT_CODE_FONT, customCodeFont))
}

function applyTheme(value?: string) {
  const theme = value === 'dark' ? 'dark' : 'light'
  const root = document.documentElement
  root.classList.remove('theme--light', 'theme--dark')
  root.classList.add(`theme--${theme}`)
  root.dataset.mantineColorScheme = theme
  root.style.colorScheme = theme
  if (globalThis.localStorage) globalThis.localStorage.setItem('mantine-color-scheme-value', theme)
}

function applyRounded(enabled?: boolean) {
  const existing = document.getElementById(ROUNDED_STYLE_ID)
  if (!enabled) {
    existing?.remove()
    return
  }
  if (existing) return
  const style = document.createElement('style')
  style.id = ROUNDED_STYLE_ID
  style.textContent = `
    .section { border-radius: 8px; }
    .section__table-header { border-radius: 8px 8px 0 0; }
  `
  document.head.append(style)
}

export function ensureHydroGlobals(userId = 0, preferences: HydroUserPreferences = {}) {
  globalThis.UserContext ??= {
    _id: 0,
    priv: 0,
    viewLang: 'zh',
    preferredEditorType: 'sv',
    fontFamily: 'Open Sans',
    codeFontFamily: 'Source Code Pro',
    skipAnimate: false,
    showTimeAgo: true,
    theme: 'light',
    rounded: false,
    showInvisibleChar: false,
    formatCode: true,
  }
  globalThis.UserContext._id = userId
  globalThis.UserContext.priv = preferences.priv ?? 0
  globalThis.UserContext.viewLang = preferences.viewLang || 'zh'
  globalThis.UserContext.preferredEditorType = preferences.preferredEditorType || 'sv'
  globalThis.UserContext.fontFamily = preferences.fontFamily || 'Open Sans'
  globalThis.UserContext.codeFontFamily = preferences.codeFontFamily || 'Source Code Pro'
  globalThis.UserContext.codeLang = preferences.codeLang || ''
  globalThis.UserContext.codeTemplate = preferences.codeTemplate || ''
  globalThis.UserContext.skipAnimate = Boolean(preferences.skipAnimate)
  globalThis.UserContext.showTimeAgo = preferences.showTimeAgo ?? true
  globalThis.UserContext.theme = preferences.theme === 'dark' ? 'dark' : 'light'
  globalThis.UserContext.rounded = Boolean(preferences.rounded)
  globalThis.UserContext.showInvisibleChar = Boolean(preferences.showInvisibleChar)
  globalThis.UserContext.formatCode = preferences.formatCode ?? true
  applyTheme(globalThis.UserContext.theme)
  applyRounded(globalThis.UserContext.rounded)
  applyFontPreferences(preferences.fontFamily, preferences.codeFontFamily)
  return globalThis.UserContext
}
