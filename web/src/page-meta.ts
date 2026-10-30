export type PageMeta = {
  name: string
  layout: 'basic' | 'immersive'
  title: string
}

const basic = (name: string, title: string): PageMeta => ({ name, layout: 'basic', title })

export function pageMetaForPath(pathname: string): PageMeta {
  if (pathname === '/') return basic('homepage', '首页')
  if (pathname.startsWith('/d/')) return basic('redirect', '跳转中')
  if (pathname === '/about') return basic('wiki_about', '关于')
  if (pathname === '/wiki/help') return basic('wiki_help', '帮助')
  if (pathname === '/status') return basic('status', '服务状态')
  if (pathname === '/login') return { name: 'user_login', layout: 'immersive', title: '登录' }
  if (pathname === '/lostpass') return { name: 'user_lostpass', layout: 'immersive', title: '找回密码' }
  if (/^\/lostpass\/[^/]+\/?$/.test(pathname)) return { name: 'user_lostpass_with_code', layout: 'immersive', title: '重置密码' }
  if (pathname === '/register') return { name: 'user_register', layout: 'immersive', title: '注册' }
  if (/^\/register\/[^/]+\/?$/.test(pathname)) return { name: 'user_register_with_code', layout: 'immersive', title: '注册' }
  if (pathname === '/p') return basic('problem_main', '题库')
  if (pathname === '/p/new') return basic('problem_create', '创建题目')
  if (pathname === '/problem/random') return basic('problem_random', '随机选题')
  if (pathname === '/problem/import/fps') return basic('problem_import_fps', '从 FPS 文件导入')
  if (pathname === '/problem/import/hydro') return basic('problem_import_hydro', '从 Hydro 压缩包导入')
  if (pathname.startsWith('/p/tag/')) return basic('problem_tag_stat', '标签榜单')
  if (/^\/p\/[^/]+\/edit\/?$/.test(pathname)) return basic('problem_edit', '编辑题目')
  if (/^\/p\/[^/]+\/config\/?$/.test(pathname)) return basic('problem_config', '判题配置')
  if (/^\/p\/[^/]+\/files\/?$/.test(pathname)) return basic('problem_files', '题目文件')
  if (/^\/p\/[^/]+\/submit\/?$/.test(pathname)) return basic('problem_submit', '提交题目')
  if (/^\/p\/[^/]+\/solution(\/[^/]+)?\/?$/.test(pathname)) return basic('problem_solution', '题解')
  if (/^\/p\/[^/]+\/statistics\/?$/.test(pathname)) return basic('problem_statistics', '题目统计')
  if (/^\/p\/[^/]+\/?$/.test(pathname)) return basic('problem_detail', '题目详情')
  if (pathname === '/record') return basic('record_main', '评测记录')
  if (/^\/record\/[^/]+\/?$/.test(pathname)) return basic('record_detail', '评测详情')
  if (pathname === '/contest') return basic('contest_main', '比赛')
  if (pathname === '/contest/new') return basic('contest_create', '创建比赛')
  if (/^\/contest\/[^/]+\/edit\/?$/.test(pathname)) return basic('contest_edit', '编辑比赛')
  if (/^\/contest\/[^/]+\/manag(ement)?\/?$/.test(pathname) || /^\/contest\/[^/]+\/files\/?$/.test(pathname)) return basic('contest_manage', '比赛管理')
  if (/^\/contest\/[^/]+\/problems\/?$/.test(pathname)) return basic('contest_problemlist', '比赛题目')
  if (/^\/contest\/[^/]+\/clarification\/?$/.test(pathname)) return basic('contest_clarification', '比赛答疑管理')
  if (/^\/contest\/[^/]+\/user\/?$/.test(pathname)) return basic('contest_user', '参赛者管理')
  if (/^\/contest\/[^/]+\/balloon\/?$/.test(pathname)) return basic('contest_balloon', '气球状态')
  if (/^\/contest\/[^/]+\/print\/?$/.test(pathname)) return basic('contest_print', '打印请求')
  if (/^\/contest\/[^/]+\/scoreboard\/?$/.test(pathname)) return basic('contest_scoreboard', '比赛榜单')
  if (/^\/contest\/[^/]+\/?$/.test(pathname)) return basic('contest_detail', '比赛详情')
  if (pathname === '/training') return basic('training_main', '训练')
  if (pathname === '/training/new') return basic('training_create', '创建训练')
  if (/^\/training\/[^/]+\/edit\/?$/.test(pathname)) return basic('training_edit', '编辑训练')
  if (/^\/training\/[^/]+\/files?\/?$/.test(pathname)) return basic('training_files', '训练文件')
  if (/^\/training\/[^/]+\/?$/.test(pathname)) return basic('training_detail', '训练详情')
  if (pathname === '/homework') return basic('homework_main', '作业')
  if (pathname === '/homework/new') return basic('homework_create', '创建作业')
  if (/^\/homework\/[^/]+\/edit\/?$/.test(pathname)) return basic('homework_edit', '编辑作业')
  if (/^\/homework\/[^/]+\/files?\/?$/.test(pathname)) return basic('homework_files', '作业文件')
  if (/^\/homework\/[^/]+\/scoreboard\/?$/.test(pathname)) return basic('homework_scoreboard', '作业榜单')
  if (/^\/homework\/[^/]+\/?$/.test(pathname)) return basic('homework_detail', '作业详情')
  if (pathname === '/discuss') return basic('discussion_main', '讨论')
  if (pathname === '/discuss/new') return basic('discussion_create', '创建讨论')
  if (/^\/discuss\/(?:node|contest|homework|training|problem)\/[^/]+\/create\/?$/.test(pathname)) return basic('discussion_create', '创建讨论')
  if (/^\/discuss\/[^/]+\/edit\/?$/.test(pathname)) return basic('discussion_edit', '编辑讨论')
  if (/^\/discuss\/(?:node|contest|homework|training|problem)\/[^/]+\/?$/.test(pathname)) return basic('discussion_node', '讨论节点')
  if (/^\/discuss\/[^/]+\/?$/.test(pathname)) return basic('discussion_detail', '讨论详情')
  if (/^\/user\/[^/]+\/?$/.test(pathname)) return basic('user_detail', '用户主页')
  if (pathname === '/ranking') return basic('ranking', '排行榜')
  if (pathname === '/home/messages') return basic('home_messages', '消息')
  if (pathname === '/home/security') return basic('home_security', '安全设置')
  if (pathname.startsWith('/home/changeMail/')) return basic('home_security', '确认邮箱变更')
  if (pathname === '/home/settings') return basic('home_account', '账户设置')
  if (pathname === '/file') return basic('home_files', '我的文件')
  if (pathname === '/admin/users') return basic('manage_user_priv', '用户权限')
  if (pathname === '/admin/users/import') return basic('manage_user_import', '用户导入')
  if (pathname === '/admin/settings') return basic('manage_setting', '系统设置')
  if (pathname === '/admin/audit') return basic('manage_audit', '操作审计')
  if (pathname === '/admin') return basic('manage_dashboard', '控制面板')
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
