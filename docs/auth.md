# 用户认证

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/auth/login` | 用户名/邮箱 + 密码登录 |
| GET | `/api/v1/auth/me` | 获取当前登录用户 |
| POST | `/api/v1/auth/logout` | 退出登录 |

登录成功设置 HttpOnly `sid` Cookie；判题机登录仍使用独立的 `/login` 兼容接口。

## 密码兼容

Hydo 用户密码格式：

```text
pbkdf2(password, salt, 100000, 64, sha256)
取 hex 前 64 个字符
hashType = "hydro"
```

Go 实现位于 `internal/user/password.go`。

## 前端

登录页结构和类名对应 Hydro `ui-default` 的 `user_login.html`，使用 `layout--immersive`、`.immersive--content`、`.inverse` 表单样式和原版背景资源。

## Session

- 浏览器用户和判题机 Session 存储在 MongoDB `session` 集合。
- Cookie 中的原始 token 不写入数据库，`_id` 使用 token 的 SHA-256 哈希。
- `expiresAt` 上有 TTL 索引 `session_ttl`，应用读取时也会立即检查过期。
- 默认有效期 `24h`，通过 `SESSION_TTL` 配置。
- API 重启后用户 Cookie 和判题机 Cookie 继续有效。

## 当前限制

- 尚未实现注册、找回密码、TFA、WebAuthn。
- 登录成功后暂不更新 `loginat`、登录 IP 等审计字段。
