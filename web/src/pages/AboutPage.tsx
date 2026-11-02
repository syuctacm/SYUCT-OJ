import { useMemo } from 'react'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { useSiteSettings } from '../site-settings.tsx'

const SECTIONS = [
  {
    id: 'about',
    title: '关于 SYUCT ACM OJ',
    content: 'SYUCT ACM OJ 是沈阳化工大学 ACM-ICPC 集训队使用的在线评测与训练平台，提供题库、比赛、作业、训练、讨论和提交记录等功能。',
  },
  {
    id: 'contact',
    title: '联系我们',
    content: '如需反馈问题、申请权限或联系管理员，请前往[讨论区](/discuss)。',
  },
  {
    id: 'admin',
    title: '管理与维护',
    content: '站点由集训队管理员维护。管理员可以在控制面板中管理用户、题目、比赛、训练和系统公告。',
  },
]

export function AboutPage() {
  const { settings } = useSiteSettings()
  const sections = useMemo(() => {
    const source = (settings.aboutMarkdown || '').replaceAll('{{ name }}', settings.siteName)
    if (!source.trim()) return SECTIONS
    const ids = [...source.matchAll(/^#\s+(\S+)\s+(.+)$/gm)]
    if (!ids.length) return [{ id: 'about', title: '关于', content: source }]
    return ids.map((match, index) => {
      const start = match.index ?? 0
      const next = ids[index + 1]?.index ?? source.length
      const chunk = source.slice(start, next)
      return {
        id: match[1],
        title: match[2],
        content: chunk.replace(/^#\s+.+\n?/, '').trim(),
      }
    })
  }, [settings.aboutMarkdown, settings.siteName])
  const context = useMemo(() => ({
    page_name: 'wiki_about',
    sections,
    model: { system: { get: (key: string) => (key === 'server.name' ? settings.siteName : '') } },
    handler: {
      user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false },
      domain: { ui: { name: settings.siteName } },
    },
  }), [sections, settings.siteName])
  return <HydroTemplate template="about.html" context={context} />
}
