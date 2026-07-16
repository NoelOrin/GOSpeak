# pkg 模块

公共工具包，提供跨模块复用的基础能力。

## 文件说明

| 文件 | 职责 |
|------|------|
| errors.go | 业务错误码（`ErrCode`）与 `AppError` 类型 |
| jwt.go | JWT 生成/解析；每个 Token 携带 ULID 作为 JTI；签名密钥由 `redis.GetSigningKey()` / `GetAllSigningKeys()` 提供（支持密钥轮换与历史密钥）；access 15m、refresh 7d、bot 可永久 |
| response.go | 统一响应封装：`Success` / `Fail` / `HandleError` |

## 子包

| 包 | 说明 |
|----|------|
| oauth/ | 通用 OAuth2 协议实现；`GitHubProvider` / `GoogleProvider` / `QQProvider` 预设端点，并支持通过 `oauth_providers` 配置的自定义 provider |
| permcode/ | 权限码常量（`user:read` 等），供 middleware 与路由使用 |

## JWT Claims

```
username, display_name, user_uuid, role, token_version, permissions?(bot)
```

`token_version` 与 `User.TokenVersion` 绑定，改密/重置后递增使旧 token 失效。

## 错误码范围

| 范围 | 类别 |
|------|------|
| 0 | 成功 |
| 1xxx | 认证相关（含 `TOKEN_REVOKED=1014` 黑名单/版本失效）|
| 2xxx | 参数校验 |
| 3xxx | 资源相关 |
| 5xxx | 服务端内部错误 |
| 6xxx | SFU / 媒体相关 |
| 7xxx | OAuth 相关 |
