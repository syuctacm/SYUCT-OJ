# React 核心页面

## 已实现路由

| 路径 | 页面 |
|---|---|
| `/` | 首页 |
| `/p` | 题库列表与搜索 |
| `/p/:pid` | 题目详情 |
| `/p/:pid/submit` | 提交代码 |
| `/record` | 当前用户提交记录 |
| `/record/:rid` | 提交状态轮询 |
| `/contest` | 比赛列表 |
| `/contest/:tid` | 比赛详情 |
| `/homework` | 作业列表 |
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
- `web/src/index.css`：仅保留 React 页面与旧主题类名之间的兼容样式
- 页面使用 `.section`、`.section__header`、`.section__body`、`.section__footer`、`.data-table`、`.button`、`.textbox` 等原版类名
- 导航、移动端侧滑菜单、页脚使用 `ui-default` 对应的 DOM 结构和类名

主题来源：

```text
https://github.com/hydro-dev/Hydro
packages/ui-default
```

主题文件包含上游字体声明。当前仓库只内置实际使用的 Source Code Pro 字体文件，其余字体使用系统回退。

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
- 旧 URL `/d/system/...` 的完整兼容重定向
- 自动截图回归基线
