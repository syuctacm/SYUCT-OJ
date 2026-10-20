export type PageMeta = {
  name: string
  layout: 'basic' | 'immersive'
  title: string
}

const basic = (name: string, title: string): PageMeta => ({ name, layout: 'basic', title })

export function pageMetaForPath(pathname: string): PageMeta {
  if (pathname === '/') return basic('homepage', '首页')
  if (pathname.startsWith('/d/')) return basic('redirect', '跳转中')
  if (pathname === '/login') return { name: 'user_login', layout: 'immersive', title: '登录' }
  if (pathname === '/p') return basic('problem_main', '题库')
  if (pathname === '/p/new') return basic('problem_create', '创建题目')
  if (pathname.startsWith('/p/tag/')) return basic('problem_tag', '标签榜单')
  if (/^\/p\/[^/]+\/edit\/?$/.test(pathname)) return basic('problem_edit', '编辑题目')
  if (/^\/p\/[^/]+\/config\/?$/.test(pathname)) return basic('problem_config', '判题配置')
  if (/^\/p\/[^/]+\/files\/?$/.test(pathname)) return basic('problem_files', '题目文件')
  if (/^\/p\/[^/]+\/submit\/?$/.test(pathname)) return basic('problem_submit', '提交题目')
  if (/^\/p\/[^/]+\/?$/.test(pathname)) return basic('problem_detail', '题目详情')
  if (pathname === '/record') return basic('record_main', '评测记录')
  if (/^\/record\/[^/]+\/?$/.test(pathname)) return basic('record_detail', '评测详情')
  if (pathname === '/contest') return basic('contest_main', '比赛')
  if (pathname === '/contest/new') return basic('contest_create', '创建比赛')
  if (/^\/contest\/[^/]+\/edit\/?$/.test(pathname)) return basic('contest_edit', '编辑比赛')
  if (/^\/contest\/[^/]+\/?$/.test(pathname)) return basic('contest_detail', '比赛详情')
  if (pathname === '/training') return basic('training_main', '训练')
  if (/^\/training\/[^/]+\/?$/.test(pathname)) return basic('training_detail', '训练详情')
  if (pathname === '/homework') return basic('homework_main', '作业')
  if (/^\/homework\/[^/]+\/?$/.test(pathname)) return basic('homework_detail', '作业详情')
  if (pathname === '/discuss') return basic('discussion_main', '讨论')
  if (pathname === '/discuss/new') return basic('discussion_create', '创建讨论')
  if (/^\/discuss\/[^/]+\/?$/.test(pathname)) return basic('discussion_detail', '讨论详情')
  if (/^\/user\/[^/]+\/?$/.test(pathname)) return basic('user_detail', '用户主页')
  if (pathname === '/admin/users') return basic('manage_user_priv', '用户权限')
  if (pathname === '/admin') return basic('manage_dashboard', '后台')
  return basic('error', '页面不存在')
}

export function applyPageMeta(meta: PageMeta) {
  const root = document.documentElement
  for (const className of [...root.classList]) {
    if (className.startsWith('page--') || className.startsWith('layout--')) root.classList.remove(className)
  }
  root.classList.add(`layout--${meta.layout}`, `page--${meta.name}`)
  root.dataset.page = meta.name
  root.dataset.layout = meta.layout
  root.dataset.app = 'Hydro'
  root.dataset.mantineColorScheme = 'light'
  document.title = `${meta.title} - SYUCT OJ`
}
