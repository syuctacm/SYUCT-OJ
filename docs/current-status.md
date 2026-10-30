# 当前实现状态

更新日期：2026-10-31  
开发分支：`rewrite`

## 已完成

- 单站点 Go API、MongoDB 数据兼容、Session、TOTP、Passkey、邮箱验证、找回密码和账号注销
- 题目、提交、hydrojudge 协议、记录、比赛、作业、训练、讨论、文件、消息、签到、倒计时、活跃日历和标签榜单
- React SPA 直接复用 Hydro `ui-default` 模板和主题；页面按路由懒加载
- 登录弹窗、TOTP、移动端返回路径、Prism、样例输入输出、相对时间、题面标记、富媒体链接、403/404/500 错误页
- 本地 Docker 判题闭环、迁移演练、生产形态 Compose 冒烟脚本

## 验证基线

```bash
go test ./... -count=1
go vet ./...
go build ./...
pnpm --dir web lint
pnpm --dir web build
```

涉及 MongoDB 的集成测试使用：

```bash
TEST_MONGO_URI='mongodb://syuct:syuct_dev_password@127.0.0.1:27017/?authSource=admin' go test ./... -count=1
```

前端视觉和交互记录见 [parity-checklist.md](./parity-checklist.md)。

## 仍然存在的缺口

### 功能

- OAuth 第三方登录/绑定；当前没有配置 provider
- 比赛组队模式
- 讨论节点的创建、编辑和删除管理
- 作业 assign/分组访问控制、题目依赖和进度
- 用户主页可选题解历史；上游模板只展示题解数量和被赞数
- 后台脚本管理、配置管理、默认权限编辑、IP 自动登录可视化页面
- Hack、交互题和特殊 checker 的完整兼容验证

### 前端低优先级

- `queryLocalFonts()` 本地字体选择弹窗
- 超级管理员浏览器版本提示
- `preferredEditorType` 等少量上游偏好项
- 自动截图回归基线

### 发布验收

- 真实生产备份副本的迁移 `--dry-run`、计数校验、切换和回滚演练
- 当前开发环境没有 `/home/syuct/backup`，该验收只能在备份副本挂载到本地后继续

## 明确不在范围

- 多域/多租户
- VJudge 远程评测
- HydroAC 远程评测/LSP
- Cordis/addon 插件加载器
- SSR/Next.js
- 第一阶段 PostgreSQL/Redis/Kafka/微服务化
