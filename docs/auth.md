# 用户认证

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/auth/login` | 用户名/邮箱 + 密码登录，启用 2FA 时还需 `tfa` |
| POST | `/api/v1/auth/webauthn/login/begin` | 根据用户名生成 Passkey 登录 challenge |
| POST | `/api/v1/auth/webauthn/login/finish` | 校验 Passkey assertion 并创建浏览器 Session |
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

## Passkey 登录

Passkey 使用 `github.com/go-webauthn/webauthn`，一期采用“先输入用户名，再选择该账号已注册 Passkey”的流程，不启用无需用户名的 discoverable login。

- `POST /api/v1/auth/webauthn/login/begin` 接收 `username`，返回 `options.publicKey` 和一次性 `token`
- 浏览器用 `@simplewebauthn/browser` 的 `startAuthentication` 完成认证器交互
- `POST /api/v1/auth/webauthn/login/finish` 接收 `token` / `credential`，验证 origin、RPID、challenge 和签名
- 验证成功后签发与密码登录相同的 HttpOnly `sid` Session，并更新 `loginat` / `loginip`
- challenge 保存 5 分钟，绑定 uid 和用途，成功或失败校验后都只能使用一次

配置项：

```text
WEBAUTHN_RPID=oj.example.com
WEBAUTHN_ORIGINS=https://oj.example.com
WEBAUTHN_DISPLAY_NAME=SYUCT OJ
```

生产环境必须使用 HTTPS 和同一主机名；本地开发使用 `http://localhost:5174`，不能把 `127.0.0.1` 与 RPID `localhost` 混用。`WEBAUTHN_RPID` 一旦有用户注册 Passkey 就必须保持稳定，修改后已有 Passkey 将全部失效。

旧 Hydro 的 `user.authenticators` 可通过 `cmd/migrate-webauthn` 导入，详见 `docs/migrations.md`。导入后会保留原来的 uid 字节 user handle，且不会删除旧字段。

## 密码兼容

Hydro 用户密码格式：

```text
pbkdf2(password, salt, 100000, 64, sha256)
取 hex 前 64 个字符
hashType = "hydro"
```

Go 实现位于 `internal/user/password.go`。

## 前端

登录页由 `HydroTemplate` 直接渲染 Hydro `ui-default` 的 `user_login.html`，继续使用 `layout--immersive`、`.immersive--content`、`.inverse` 表单样式和原版背景资源。React 只负责登录请求和错误状态；服务端返回 2FA required 时，页面会在原表单中插入 6 位验证码输入框。支持 HTTPS/localhost 时，原表单下方会显示“使用 Passkey 登录”按钮。暂不支持找回密码和 OAuth，因此对应入口会从原模板中移除。

## Session

- 浏览器用户和判题机 Session 存储在 MongoDB `session` 集合。
- Cookie 中的原始 token 不写入数据库，`_id` 使用 token 的 SHA-256 哈希。
- `expiresAt` 上有 TTL 索引 `session_ttl`，应用读取时也会立即检查过期。
- 默认有效期 `24h`，通过 `SESSION_TTL` 配置。
- API 重启后用户 Cookie 和判题机 Cookie 继续有效。
- 登录成功会更新 `loginat` / `loginip`（用户主页“最后登录”）。

## 当前限制

- 尚未实现找回密码和 OAuth。
- 注册 token 仍直接由接口返回；邮箱修改已支持可选 SMTP 发送确认链接，相关配置见 `.env.example`。
