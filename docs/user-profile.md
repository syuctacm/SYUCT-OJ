# 用户主页 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/users/:uid` | 公开用户资料 |
| GET | `/api/v1/users?search=...` | 用户名/显示名搜索 |
| GET | `/api/v1/users?auto=1,2` | 按 UID 批量读取 |
| GET | `/api/v1/groups?search=...` | 分组名搜索 |
| GET | `/api/v1/groups?names=2026` | 按分组名批量读取 |

公开资料返回用户 ID、用户名、显示名、学号、头像、简介、注册/登录时间和提交/通过统计，不返回邮箱和密码字段。自动补全接口只返回 ID、用户名、显示名和头像；分组接口只返回分组名与成员 UID。

## 前端

- `/user/:uid`：用户主页
- 登录后的导航栏用户名会链接到自己的主页
- 页面结构和类名对应 Hydro `user_detail.html` 的 `.profile-header`、`.profile-content` 和侧栏统计。活跃日历由原 Activity Calendar 插件迁移为 React 组件。

## 尚未完成

- 用户签名和社交信息
- 用户主页的题目/比赛历史
