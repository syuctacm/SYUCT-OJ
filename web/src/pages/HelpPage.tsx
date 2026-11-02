import { useMemo } from 'react'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { useSiteSettings } from '../site-settings.tsx'

export function HelpPage() {
  const { settings } = useSiteSettings()
  const context = useMemo(() => ({
    page_name: 'wiki_help',
    model: { system: { get: (key: string) => (key === 'server.name' ? settings.siteName : '') } },
    handler: {
      user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false },
      domain: { ui: { name: settings.siteName } },
    },
  }), [settings.siteName])
  return <HydroTemplate template="wiki_help.html" context={context} />
}
