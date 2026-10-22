# 安全设置

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/user/security/sessions` | 当前用户的活动会话列表 |
| DELETE | `/api/v1/user/security/sessions/:digest` | 退出指定会话 |
| DELETE | `/api/v1/user/security/sessions` | 退出全部会话 |
| POST | `/api/v1/user/security/password` | 修改密码（`current` / `password` / `verifyPassword`） |

会话摘要（`digest`）是会话 token 的 SHA-256，浏览器拿不到、也无法用它对其它账号操作：所有接口都按 `userId` 过滤。

## 会话存储

`session` 集合在原有字段上补了三项元数据：

```js
{ _id: <token sha256>, userId, kind, createdAt, updateAt, expiresAt, createIp, createUa }
```

- 登录时记录 UA 与 IP（`IssueForUserMeta`）
- 每次带 Cookie 访问时，如果 `updateAt` 超过 10 分钟未更新则刷新一次，避免每个请求都写库
- 老会话没有 `updateAt` 时回退到 `createdAt` 显示

## 前端

`/home/security` 直接渲染上游 `home_security.html`，保留两个区块：

- **修改密码**：校验当前密码、两次新密码一致；成功后注销该用户全部会话并跳转登录页（与上游一致）
- **活动会话**：最后活动时间、IP、由 UA 解析出的操作系统与浏览器、当前会话标记；可单独退出其它会话，或退出全部会话

UA 解析是本地的小函数（Chrome/Firefox/Safari/Edge/Opera + Windows/macOS/Linux/Android/iOS），没有引入 ua-parser-js。

## 尚未迁移

- 修改邮箱：需要邮件服务，暂未接入
- 2FA / Passkey 认证器
- 第三方账号绑定（OAuth）

以上区块会在页面上移除，不会出现无法使用的控件。
