# 用户主页 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/users/:uid` | 公开用户资料（含已通过题目、RP、排名） |
| GET | `/api/v1/ranking` | 全站 RP 排行榜（分页） |
| GET | `/api/v1/users?search=...` | 用户名/显示名搜索 |
| GET | `/api/v1/users?auto=1,2` | 按 UID 批量读取 |
| GET | `/api/v1/groups?search=...` | 分组名搜索 |
| GET | `/api/v1/groups?names=2026` | 按分组名批量读取 |

公开资料返回用户 ID、用户名、显示名、学号、头像、简介、注册/登录时间、提交/通过统计、已通过题目（`acceptedProblems`，最多 100 道、按 docId 排序、隐藏题不返回）以及从 `domain.user` 读取的 RP、等级和全站排名（RP 为 0 时排名为 0），不返回邮箱和密码字段。自动补全接口只返回 ID、用户名、显示名和头像；分组接口只返回分组名与成员 UID。

## 前端

- `/user/:uid`：用户主页
- `/ranking`：全站 RP 排行榜（上游 `ranking.html`）
- 登录后的导航栏用户名会链接到自己的主页
- 页面由 `HydroTemplate` 直接渲染上游 `user_detail.html`：资料头部、联系方式图标、简介标签页、已通过题目标签页和侧栏统计都来自原模板
- 活跃日历（原 Activity Calendar 插件）作为标签页挂载到原版标签结构中

## 头像

用户文档里的 `avatar` 是带 scheme 的字符串，渲染前统一走 `web/src/upstream/avatar.ts`（移植自上游 `packages/hydrooj/src/lib/avatar.ts`）：

| 值 | 渲染结果 |
|---|---|
| `gravatar:mail@example.com` | `//cn.gravatar.com/avatar/<md5(小写邮箱)>?d=mm&s=<size>` |
| `qq:123456789` | `//q1.qlogo.cn/g?b=qq&nk=123456789&s=160` |
| `github:hydro-dev` | `//github.com/hydro-dev.png?size=<min(size,460)>` |
| `url:https://…` | 原样输出 |
| 空值 | 空邮箱的 gravatar（与旧站一致） |

gravatar 需要 MD5，浏览器没有同步实现，因此在 `web/src/upstream/md5.ts` vendored 了一份 RFC 1321 实现，并用 RFC 测试向量和 `md5sum` 对照验证（含 UTF-8 中文）。Gravatar 前缀沿用上游默认值 `//cn.gravatar.com/avatar/`，等设置系统接入后再改成可配置项。

所有页面和两个用户自动补全组件都使用同一个 `avatarUrl()`，不再各自内联 `value || '/logo.png'`。

## 尚未完成

- 用户签名和社交信息（邮箱/QQ/微信/性别图标）
- 头像上传与 `/home/settings` 账户设置页
- 获赞题解（当前显示 0）
- 个人主页的账号自助侧栏（消息、设置、文件）暂不显示
- 在线状态（当前统一显示离线）
- 用户主页的题目/比赛历史
