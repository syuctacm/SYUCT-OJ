# React 核心页面

## 已实现路由

| 路径 | 页面 |
|---|---|
| `/` | 首页 |
| `/p` | 题库列表与搜索 |
| `/p/:pid` | 题目详情 |
| `/p/:pid/submit` | 提交代码 |
| `/p/new` | 创建题目 |
| `/p/:pid/edit` | 编辑题目 |
| `/p/:pid/config` | 判题配置 |
| `/p/:pid/files` | 测试数据与附加文件 |
| `/record` | 当前用户提交记录 |
| `/record/:rid` | 提交状态轮询 |
| `/contest` | 比赛列表 |
| `/contest/new` | 创建比赛 |
| `/contest/:tid` | 比赛详情 |
| `/contest/:tid/edit` | 编辑比赛 |
| `/homework` | 作业列表 |
| `/homework/new` | 创建作业 |
| `/homework/:tid/edit` | 编辑作业 |
| `/homework/:tid` | 作业详情 |
| `/training` | 训练列表 |
| `/training/:tid` | 训练详情 |
| `/discuss` | 讨论列表 |
| `/discuss/new` | 创建讨论 |
| `/discuss/:did` | 讨论详情与回复 |
| `/user/:uid` | 用户主页 |
| `/admin` | 管理员后台概览 |
| `/admin/users` | 管理员用户列表 |
| `/login` | 用户登录 |

## 首页

`/` 对照 Hydro `main.html` 和 `partials/homepage/*`：

- 比赛区块
- 讨论区块
- 训练区块
- 最近题目和推荐链接侧栏

数据通过现有比赛、讨论、训练和题目 API 并行读取。

## 视觉策略

前端不自行仿制 Hydro 主题，直接复用 Hydro `ui-default` 已编译主题：

- `web/public/hydro-theme.css`：Hydro `ui-default` 4.58.5 主题 CSS
- `web/public/nav_logo_dark.png`：旧站导航 Logo
- `web/public/hydro-icons.*`：Hydro 图标字体
- `web/src/index.css`：只保留 React 应用壳的兼容样式，不仿制页面视觉
- `web/src/legacy/`：从旧站原样迁移的独立插件 CSS，例如 Activity Calendar
- 页面使用 `.section`、`.section__header`、`.section__body`、`.section__footer`、`.data-table`、`.button`、`.textbox` 等原版类名
- `web/src/components/LegacyForm.tsx` 直接对应 Hydro `templates/components/form.html`，统一输出 `.row`、`.columns`、`.form__item`、`.textbox-container`、`.select-container`、`.textarea-container` 和 `.checkbox-container`
- 导航、移动端侧滑菜单、页脚使用 `ui-default` 对应的 DOM 结构和类名
- `web/src/page-meta.ts` 根据路由设置原模板的 `data-page`、`data-layout` 和 `page--*` 类
- 页面组件自己输出顶层 `.row`，`Layout` 的 `.main` 不再额外添加网格容器

主题来源：

```text
https://github.com/hydro-dev/Hydro
packages/ui-default
```

主题文件包含上游字体声明。当前仓库只内置实际使用的 Source Code Pro 字体文件，其余字体使用系统回退。

## 旧 URL 兼容

React Router 统一处理 `/d/:domainId/*`：

```text
/d/system/p/P1001              -> /p/P1001
/d/contest2025/contest/<tid>   -> /contest/<tid>
/d/select2025/record/<rid>     -> /record/<rid>
/d/system                      -> /
```

使用 `replace` 跳转，不会在浏览器历史中增加一层；query 和 hash 原样保留。

## 本地联调

```bash
# 后端
SERVER_PORT=18080 go run ./cmd/api

# 前端
cd web
VITE_API_PROXY=http://127.0.0.1:18080 pnpm dev
```

## 尚未完成

- 原版 KaTeX、代码高亮和富媒体渲染插件
- 原版全部下拉菜单和前端插件行为
- 自动截图回归基线
