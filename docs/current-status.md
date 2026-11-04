# 当前实现状态

更新日期：2026-11-04
开发分支：`rewrite`

## 已完成

- 单站点 Go API、MongoDB 数据兼容、Session、TOTP、Passkey、邮箱验证、找回密码和账号注销
- 题目、提交、hydrojudge 协议、记录、比赛、作业、训练、讨论、文件、消息、签到、倒计时、活跃日历和标签榜单
- React SPA 直接复用 Hydro `ui-default` 模板和主题；页面按路由懒加载
- 登录弹窗、TOTP、移动端返回路径、Prism、样例输入输出、相对时间、题面标记、富媒体链接、403/404/500 错误页
- 用户偏好完整度：主题、圆角、相对时间、默认编辑器、不可见字符、自动格式化选项、自定义本机字体和页脚主题切换
- 用户主页最近题解历史、题解数量和被赞数
- 后台 IP 自动登录映射可视化管理
- 用户时区偏好及比赛/作业时间、日期展示的时区转换
- 后台默认权限读取、编辑和新用户权限继承
- 后台核心站点设置：站点名、Logo、Gravatar、页脚、公告、About、Scratchpad 和首页布局
- 后台分页默认值：题目、比赛、讨论、回复、记录、排行榜、题解和训练列表
- 讨论详情回复分页复用 Hydro 原模板分页器，并支持 URL `page` 参数
- 后台训练报名用户展示开关，关闭后隐藏报名列表并禁止查看他人训练进度
- 后台题目分类 YAML，驱动题库侧栏、题目编辑分类和 `category:` 搜索
- 后台 SMTP 配置、密码只写不读、运行时更新和 465 隐式 TLS
- 完整合并 Hydro 4.58.5 核心与 `ui-default` 中文语言包，消除模板中的 `page.*` 原始 key
- 后台判题语言表：`hydrooj.langs` YAML、语言继承和运行时 hub 更新
- 后台文件与代码限制：用户/活动/题目附件数量和大小、提交代码长度
- 后台提交与自测频率限制，支持按 IP 或用户计数
- 后台用户/未保存会话过期时间配置，运行时生效
- 本地 Docker 判题闭环、迁移演练、生产形态 Compose 冒烟脚本
- 公开页面自动截图回归基线和 ImageMagick 差异检查

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

- 尚未消费的高级系统配置：配置 schema 编辑器、CDN 等部署配置

### 发布验收

- 真实生产备份副本的迁移 `--dry-run`、计数校验、切换和回滚演练
- 当前开发环境没有 `/home/syuct/backup`，该验收只能在备份副本挂载到本地后继续

## 明确不在范围

- 多域/多租户
- VJudge 远程评测
- HydroAC 远程评测/LSP
- Cordis/addon 插件加载器
- Hydro 后台脚本/插件任务管理器；维护任务统一通过仓库 CLI、迁移脚本和 Docker 脚本执行
- SSR/Next.js
- 第一阶段 PostgreSQL/Redis/Kafka/微服务化
