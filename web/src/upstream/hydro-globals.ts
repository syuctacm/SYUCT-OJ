type HydroUserContext = {
  _id: number
  viewLang: string
  preferredEditorType: string
  fontFamily: string
  codeFontFamily: string
  codeLang?: string
  codeTemplate?: string
  skipAnimate?: boolean
}

type HydroUserPreferences = {
  viewLang?: string
  preferredEditorType?: string
  fontFamily?: string
  codeFontFamily?: string
  codeLang?: string
  codeTemplate?: string
  skipAnimate?: boolean
}

declare global {
  var UserContext: HydroUserContext
}

const DEFAULT_BODY_FONT = '"Open Sans", "Seravek", "Segoe UI", "Verdana", "PingFang SC", "Hiragino Sans GB", "Lantinghei SC", "Microsoft Yahei", "WenQuanYi Micro Hei", sans-serif'
const DEFAULT_CODE_FONT = '"Source Code Pro", "Consolas", "Lucida Console", monospace'

function fontStack(family: string | undefined, fallback: string) {
  const value = family?.trim()
  if (!value) return fallback
  if (value === 'sans') return fallback
  return `"${value.replaceAll('"', '')}", ${fallback}`
}

function applyFontPreferences(fontFamily?: string, codeFontFamily?: string) {
  document.documentElement.style.setProperty('--font-family', fontStack(fontFamily, DEFAULT_BODY_FONT))
  document.documentElement.style.setProperty('--code-font-family', fontStack(codeFontFamily, DEFAULT_CODE_FONT))
}

export function ensureHydroGlobals(userId = 0, preferences: HydroUserPreferences = {}) {
  globalThis.UserContext ??= {
    _id: 0,
    viewLang: 'zh',
    preferredEditorType: '',
    fontFamily: 'Open Sans',
    codeFontFamily: 'Source Code Pro',
    skipAnimate: false,
  }
  globalThis.UserContext._id = userId
  globalThis.UserContext.viewLang = preferences.viewLang || 'zh'
  globalThis.UserContext.preferredEditorType = preferences.preferredEditorType || ''
  globalThis.UserContext.fontFamily = preferences.fontFamily || 'Open Sans'
  globalThis.UserContext.codeFontFamily = preferences.codeFontFamily || 'Source Code Pro'
  globalThis.UserContext.codeLang = preferences.codeLang || ''
  globalThis.UserContext.codeTemplate = preferences.codeTemplate || ''
  globalThis.UserContext.skipAnimate = Boolean(preferences.skipAnimate)
  applyFontPreferences(preferences.fontFamily, preferences.codeFontFamily)
  return globalThis.UserContext
}
