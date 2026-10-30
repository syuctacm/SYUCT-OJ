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
| `/training/new` | 创建训练 |
| `/training/:tid/edit` | 编辑训练 |
| `/training/:tid` | 训练详情 |
| `/discuss` | 讨论列表 |
| `/discuss/node/:name/create` | 在讨论节点创建主题 |
| `/discuss/new` | 旧创建链接，跳转到节点路由 |
| `/discuss/:did/edit` | 编辑讨论 |
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
- `web/src/upstream/render.ts` 从固定版本 `@hydrooj/ui-default@4.58.5` 读取原始 Nunjucks 模板和 `zh.yaml`，`HydroTemplate` 用原模板输出页面
- `web/src/upstream/hydrate.tsx` 只在原模板 DOM 上挂载日期控件、自动补全和 Markdown 编辑器，不重新生成页面结构
- `web/src/upstream/hydro-editor/mdeditor.ts` 由 `pnpm --dir web sync:hydro-ui` 从上游编辑器官网源码生成，仅改 import 路径
- `web/src/components/autocomplete/` 移植 Hydro `@hydrooj/components` 的自动补全，用于题目、用户、分组和语言选择
- `web/public/modules/katex/` 提供原主题 CSS 引用的 KaTeX 字体
- 导航、移动端侧滑菜单、页脚使用 `ui-default` 对应的 DOM 结构和类名
- `web/src/page-meta.ts` 根据路由设置原模板的 `data-page`、`data-layout` 和 `page--*` 类
- 首页、登录页、比赛/作业/训练/讨论创建编辑页、比赛答疑管理、排行榜、讨论列表、讨论详情、用户主页、题目列表、题目详情/编辑/文件/提交页、比赛列表、比赛详情、比赛题目列表、比赛榜单、作业列表、作业详情、训练列表、训练详情、记录列表和记录详情已直接渲染上游模板；页面组件不再自己维护这些页的顶层 `.row`

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

## 内容运行时

- 代码块通过 Prism 高亮，样例输入/输出按原版双栏展示
- 支持相对时间、题面划词标记、题目列表新标签页打开
- 样例输入可填入 Scratchpad 预测试面板
- 403 / 404 / 500 使用 Hydro 错误模板；页面渲染异常由 React error boundary 兜底

## 尚未完成

- 将剩余尚未迁移页面从手写 TSX 结构迁移到 `HydroTemplate`
- 语言自动补全改为读取服务端语言配置
- 原版全部下拉菜单和前端插件行为
- 自动截图回归基线

已完成：题目/比赛/作业/训练 Markdown 图片上传、讨论图片上传、头像上传、TOTP 两步验证、邮箱修改、首页签到/倒计时、标签榜单原插件模板和 CSV 导出。
