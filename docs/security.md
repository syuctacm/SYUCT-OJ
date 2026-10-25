# 安全设置

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/user/security/sessions` | 当前用户的活动会话列表 |
| DELETE | `/api/v1/user/security/sessions/:digest` | 退出指定会话 |
| DELETE | `/api/v1/user/security/sessions` | 退出全部会话 |
| POST | `/api/v1/user/security/password` | 修改密码（`current` / `password` / `verifyPassword`） |
| POST | `/api/v1/user/security/tfa/setup` | 生成 TOTP secret、otpauth URI 和二维码 |
| POST | `/api/v1/user/security/tfa/enable` | 校验 6 位验证码并启用 2FA |
| POST | `/api/v1/user/security/tfa/disable` | 校验当前密码并关闭 2FA |
| POST | `/api/v1/user/security/webauthn/register/begin` | 生成 Passkey 注册 challenge |
| POST | `/api/v1/user/security/webauthn/register/finish` | 校验注册结果并保存 Passkey |
| GET | `/api/v1/user/security/webauthn` | 当前用户的 Passkey 列表 |
| DELETE | `/api/v1/user/security/webauthn/:id` | 删除当前用户的一个 Passkey |
| POST | `/api/v1/user/security/email` | 校验当前密码并创建邮箱变更 token / 发送确认邮件 |
| POST | `/api/v1/user/security/email/confirm` | 使用 token 确认新邮箱 |

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

`/home/security` 直接渲染上游 `home_security.html`，保留四个区块：

- **修改密码**：校验当前密码、两次新密码一致；成功后注销该用户全部会话并跳转登录页（与上游一致）
- **关联账户**：显示当前邮箱并支持修改；要求当前密码，新邮箱通过一次性 token 确认
- **认证器**：点击“添加认证器”可选择 TOTP 或 Passkey；TOTP 显示二维码和 Base32 密钥，Passkey 使用浏览器认证器完成注册。两类认证器都在上游 `home_security.html` 的 `autherlist` 区块中展示，并可按账号删除
- **活动会话**：最后活动时间、IP、由 UA 解析出的操作系统与浏览器、当前会话标记；可单独退出其它会话，或退出全部会话

邮箱变更 token 默认 24 小时有效，只能由发起变更的 uid 使用。配置 `MAIL_HOST` / `MAIL_PORT` / `MAIL_USERNAME` / `MAIL_PASSWORD` / `MAIL_FROM` 后通过 SMTP 发送确认链接；未配置 SMTP 时本地模式会直接返回确认链接，便于本地 Docker 验证。

TOTP 使用 SHA-1、6 位、30 秒时间片，并允许前后各一个时间片；secret 只保存在用户文档的 `tfa` 字段，`auth/me` 只返回布尔状态。

Passkey 使用稳定的 32 字节 `webauthnId` 作为 WebAuthn user handle。数据库集合：

- `webauthn_credentials`：按 credential ID 保存公钥、计数器、传输方式、名称和注册时间
- `webauthn_challenges`：保存 5 分钟有效的一次性注册/登录 challenge，并建立 TTL 索引

注册需要已登录 Session；删除接口按 `uid` 和 credential ID 双重过滤，不能删除其它账号的 Passkey。生产配置 `WEBAUTHN_RPID` / `WEBAUTHN_ORIGINS` 必须对应当前 HTTPS 站点。

已有 Hydro WebAuthn 数据通过 `cmd/migrate-webauthn` 一次性导入，工具支持 dry-run、冲突预检和幂等复跑；原始 `user.authenticators` 不会被自动删除。

UA 解析是本地的小函数（Chrome/Firefox/Safari/Edge/Opera + Windows/macOS/Linux/Android/iOS），没有引入 ua-parser-js。

## 尚未迁移

- 第三方账号绑定（OAuth）

以上区块会在页面上移除，不会出现无法使用的控件。
