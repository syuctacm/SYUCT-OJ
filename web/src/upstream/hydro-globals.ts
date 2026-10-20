type HydroUserContext = {
  _id: number
  viewLang: string
  preferredEditorType: string
  codeFontFamily: string
}

declare global {
  var UserContext: HydroUserContext
}

export function ensureHydroGlobals(userId = 0, viewLang = 'zh') {
  globalThis.UserContext ??= {
    _id: 0,
    viewLang,
    preferredEditorType: '',
    codeFontFamily: '"Source Code Pro", "Consolas", monospace',
  }
  globalThis.UserContext._id = userId
  globalThis.UserContext.viewLang = viewLang
  return globalThis.UserContext
}
