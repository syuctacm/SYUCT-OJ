import { useMemo } from 'react'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

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
  const context = useMemo(() => ({
    page_name: 'wiki_about',
    sections: SECTIONS,
    model: { system: { get: (key: string) => (key === 'server.name' ? 'SYUCT ACM OJ' : '') } },
    handler: {
      user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false },
      domain: { ui: { name: 'SYUCT ACM OJ' } },
    },
  }), [])
  return <HydroTemplate template="about.html" context={context} />
}
