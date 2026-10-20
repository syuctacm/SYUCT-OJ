# React 核心页面

## 已实现路由

| 路径 | 页面 |
|---|---|
| `/` | 重定向到题库 |
| `/p` | 题库列表与搜索 |
| `/p/:pid` | 题目详情 |
| `/p/:pid/submit` | 提交代码 |
| `/contest` | 比赛列表 |
| `/contest/:tid` | 比赛详情 |
| `/discuss` | 讨论列表 |
| `/discuss/:did` | 讨论详情 |
| `/homework` | 作业列表 |
| `/homework/:tid` | 作业详情 |
| `/training` | 训练列表 |
| `/training/:tid` | 训练详情 |
| `/record` | 当前用户提交记录 |
| `/record/:rid` | 提交状态轮询 |
| `/login` | 用户登录 |

## 当前视觉策略

- 使用浅色 Hydro 风格布局：白色面板、细边框、数据表格和蓝色主色。
- 复用现有 `logo.png`。
- 后续视觉对齐阶段继续对照旧 `ui-default` 的模板、Stylus 变量和响应式断点逐项调整。

## 本地联调

```bash
# 后端
SERVER_PORT=18080 go run ./cmd/api

# 前端
cd web
VITE_API_PROXY=http://127.0.0.1:18080 pnpm dev
```

## 尚未完成

- 提交记录列表页
- 题目 Markdown 的完整旧站样式（KaTeX、代码高亮、媒体标签）
- 比赛、训练、作业、讨论、后台页面
- 旧 URL `/d/system/...` 的完整兼容重定向
- Playwright 截图基线与像素级视觉回归
