# 用户认证

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/auth/login` | 用户名/邮箱 + 密码登录，启用 2FA 时还需 `tfa` |
| GET | `/api/v1/auth/me` | 获取当前登录用户 |
| POST | `/api/v1/auth/logout` | 退出登录 |
| POST | `/api/v1/auth/register` | 提交注册邮箱，返回注册 token |
| GET | `/api/v1/auth/register/:code` | 校验 token 并返回绑定邮箱 |
| POST | `/api/v1/auth/register/:code` | 设置用户名/密码，创建账号并自动登录 |

登录成功设置 HttpOnly `sid` Cookie；判题机登录仍使用独立的 `/login` 兼容接口。

## 注册

沿用 Hydro 的邮箱 token 流程（`TokenModel.TYPE_REGISTRATION = 2`）：

1. `POST /api/v1/auth/register` 提交邮箱，在 `token` 集合写入 24h 有效的注册 token
2. 未配置 SMTP 时（当前实现），接口直接返回 token，前端跳转 `/register/<code>` —— 与上游未配置邮件时的行为一致
3. `GET /api/v1/auth/register/:code` 校验 token 并返回邮箱
4. `POST /api/v1/auth/register/:code` 校验用户名（3-31 字符或 2 个汉字）、密码（6-255）与两次输入一致后创建账号：
   - uid 取当前最大值 +1（最小 2）
   - `priv` 使用上游 `PRIV_DEFAULT`（16842756）
   - 写入 `domain.user { domainId: "system", uid, join: true }`
   - 删除注册 token、创建 Session、设置 Cookie（自动登录）

前端 `/register` 与 `/register/:code` 直接渲染上游 `user_register.html` / `user_register_with_code.html`。

## 密码兼容

Hydo 用户密码格式：

```text
pbkdf2(password, salt, 100000, 64, sha256)
取 hex 前 64 个字符
hashType = "hydro"
```

Go 实现位于 `internal/user/password.go`。

## 前端

登录页由 `HydroTemplate` 直接渲染 Hydro `ui-default` 的 `user_login.html`，继续使用 `layout--immersive`、`.immersive--content`、`.inverse` 表单样式和原版背景资源。React 只负责登录请求和错误状态；服务端返回 2FA required 时，页面会在原表单中插入 6 位验证码输入框。暂不支持找回密码和 OAuth，因此对应入口会从原模板中移除。

## Session

- 浏览器用户和判题机 Session 存储在 MongoDB `session` 集合。
- Cookie 中的原始 token 不写入数据库，`_id` 使用 token 的 SHA-256 哈希。
- `expiresAt` 上有 TTL 索引 `session_ttl`，应用读取时也会立即检查过期。
- 默认有效期 `24h`，通过 `SESSION_TTL` 配置。
- API 重启后用户 Cookie 和判题机 Cookie 继续有效。
- 登录成功会更新 `loginat` / `loginip`（用户主页“最后登录”）。

## 当前限制

- 尚未实现找回密码、WebAuthn / Passkey。
- 注册 token 仍直接由接口返回；邮箱修改已支持可选 SMTP 发送确认链接，相关配置见 `.env.example`。
