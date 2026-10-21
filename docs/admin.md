# 后台管理

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/admin/stats` | 管理员只读统计 |
| GET | `/api/v1/admin/users` | 管理员用户列表和搜索 |
| PATCH | `/api/v1/admin/users/:uid/priv` | 修改指定用户的权限位 |

所有接口需要登录用户拥有 Hydro `PRIV_EDIT_SYSTEM` 权限，当前对应 `priv & 1 != 0`。

修改权限的请求体：

```json
{"priv": 3}
```

接口直接更新 Hydro 用户文档中的 `priv` 位掩码。为避免管理员意外锁死后台，当前禁止修改自己的权限；封禁、删除用户尚未实现。

## 统计项

- 用户数
- 题目数
- 比赛数
- 作业数
- 训练数
- 讨论数
- 提交记录数
- 今日提交数
- 待评测任务数

## 前端

- `/admin` 渲染上游 `manage_dashboard.html`：Messages、Information（域头像/名称）和 `manage_base.html` 控制面板侧栏
- `/admin/settings` 渲染上游 `manage_setting.html` + `partials/setting.html`，目前只暴露站点公告 `bulletin`（Markdown），保存后立即写入 `system` 集合并被首页读取
- `/admin/users` 渲染上游 `manage_user_priv.html`：默认权限行 + 按 `priv` 分组的用户表，`show_priv` 宏显示相对默认权限的 `+/-` 位
- 两个页面共用 `manage_base.html` 侧栏，`ui.getNodes('ControlPanel')` 只声明重写版已实现的控制面板节点
- 权限编辑复用模板自带的 `.dialog__body--set-priv` 表单：点击「Edit」把这段上游 DOM 克隆进对话框，勾选后 `PATCH /api/v1/admin/users/:uid/priv`
- 「Select User」打开用户搜索对话框（`UserSelectAutoComplete`），选中后直接进入该用户的权限对话框
- 只有管理员在导航栏看到“后台”入口

### 设置存储

`bulletin` 存在 Hydro 同名的 `system` 集合里（`{_id: "bulletin", value: "..."}`）。公开接口 `GET /api/v1/settings` 只返回 `bulletin`，管理接口 `GET/PUT /api/v1/admin/settings` 需要管理员权限，长度上限 20000。

### 模板层依赖

`manage_user_priv.html` 的 `show_priv` 宏需要 `bitand` 过滤器（`web/src/upstream/render.ts`），PRIV 位值来自 `web/src/upstream/priv.ts`，与 Hydro `packages/common/permission.ts` 保持一致。

## 尚未完成

- 用户封禁和删除
- 题目/比赛/训练编辑
- 操作日志和审计
- 控制面板其余节点（脚本管理、用户导入、配置管理）
- `manage_dashboard` 的 WebSocket 连接检查（`#messages` 目前为空）和「Restart」按钮（未实现，已从 DOM 移除）
- 修改系统默认权限（默认权限行的 Edit 链接已移除）
