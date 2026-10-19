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

## 当前限制

- Session 目前是内存实现，服务重启后失效；正式认证阶段会替换为持久化 Session。
- 尚未实现注册、找回密码、TFA、WebAuthn。
- 登录成功后暂不更新 `loginat`、登录 IP 等审计字段。
