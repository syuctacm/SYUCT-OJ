import { useMemo } from 'react'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function HelpPage() {
  const context = useMemo(() => ({
    page_name: 'wiki_help',
    model: { system: { get: (key: string) => (key === 'server.name' ? 'SYUCT ACM OJ' : '') } },
    handler: {
      user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false },
      domain: { ui: { name: 'SYUCT ACM OJ' } },
    },
  }), [])
  return <HydroTemplate template="wiki_help.html" context={context} />
}
