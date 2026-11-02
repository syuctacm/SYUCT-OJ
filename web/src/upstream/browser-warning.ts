// Ported from @hydrooj/ui-default components/browser-update.page.ts (AGPL-3.0).
import { infoDialog } from '../components/dialogs.tsx'

function isSupported() {
  try {
    if (!navigator.userAgent.includes('Chrome/')) return false
    const version = Number(navigator.userAgent.split('Chrome/')[1]?.split('.')[0])
    return Number.isSafeInteger(version) && version >= 80
  } catch {
    return false
  }
}

export async function warnUnsupportedSuperAdminBrowser(priv?: number) {
  if (priv !== -1 || isSupported()) return
  const lastWarning = Number(globalThis.localStorage?.getItem('su-browser-warn') ?? 0)
  if (Number.isFinite(lastWarning) && Date.now() - lastWarning < 24 * 60 * 60 * 1000) return
  globalThis.localStorage?.setItem('su-browser-warn', Date.now().toString())
  await infoDialog(
    '身为超级管理员，您仍在使用过于老旧的浏览器。\n'
    + '这些浏览器可能不支持 Hydro 管理面板的部分功能，\n'
    + '我们强烈建议您升级到最新的 Chrome 浏览器。',
  )
}
