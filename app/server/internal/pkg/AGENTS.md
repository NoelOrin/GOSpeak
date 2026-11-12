# pkg 模块

公共工具包，提供跨模块复用的基础能力。

## 文件说明

| 文件 | 职责 |
|------|------|
| errors.go | 业务错误码（`ErrCode`）与 `AppError` 类型 |
| jwt.go | JWT 生成/解析；每个 Token 携带 ULID 作为 JTI；签名密钥由 `authstate.GetSigningKey()` / `GetAllSigningKeys()` 提供（支持密钥轮换与历史密钥）；access 15m、refresh 7d |
| response.go | 统一响应封装：`Success` / `Fail` / `HandleError` |

## 子包

| 包 | 说明 |
|----|------|
| oauth/ | 通用 OAuth2 协议实现；通过 `GenericProvider` 适配 GitHub / Google / QQ 等端点，并支持通过 `oauth_providers` 配置自定义 provider |

> 权限码常量已移至 `internal/permcode/`（`permcode.go` 通用权限、`domain_permcode.go` Domain 专属权限），不再属于 pkg 子包。

## JWT Claims

```
username, display_name, user_uuid, role, token_version, permissions?（历史字段，当前未使用）
```

`token_version` 与 `User.TokenVersion` 绑定，改密/重置后递增使旧 token 失效。

## 错误码范围

> 完整错误码定义见 `errors.go`；统一响应字段为 `msg`（非 `message`）。

| 范围 | 类别 | 关键错误码 |
|------|------|-----------|
| 0 | 成功 | `SUCCESS` |
| 1xxx | 认证相关 | `TOKEN_NOT_EXIST`(1001)、`TOKEN_WRONG`(1002)、`TOKEN_EXPIRED`(1003)、`INVALID_PASSWORD`(1010)、`USER_NOT_FOUND`(1011)、`USERNAME_EXISTS`(1012)、`FORBIDDEN`(1013)、`TOKEN_REVOKED`(1014)、`USER_BANNED`(1015)、`USER_MUTED`(1016)、`RATE_LIMITED`(1017) |
| 2xxx | 参数校验 | `INVALID_PARAMS`(2001) |
| 3xxx | 资源相关 | `NOT_FOUND`(3001)、`ALREADY_EXISTS`(3002) |
| 5xxx | 服务端内部错误 | `INTERNAL_ERROR`(5001) |
| 6xxx | SFU / 媒体相关 | `SFU_NOT_CONFIGURED`(6001)、`SFU_ERROR`(6002) |
| 7xxx | OAuth 相关 | `OAUTH_PROVIDER_NOT_FOUND`(7001)、`OAUTH_PROVIDER_DISABLED`(7002)、`OAUTH_TOKEN_EXCHANGE_FAILED`(7003)、`OAUTH_GET_USER_FAILED`(7004) |
| 8xxx | 邮箱验证相关 | `EMAIL_ALREADY_EXISTS`(8001)、`EMAIL_CODE_INVALID`(8002)、`EMAIL_CODE_EXPIRED`(8003)、`EMAIL_CODE_ALREADY_USED`(8004)、`EMAIL_SEND_TOO_FREQUENT`(8005)、`EMAIL_SEND_FAILED`(8006)、`EMAIL_NOT_VERIFIED`(8007)、`EMAIL_CODE_MAX_ATTEMPTS`(8008)、`EMAIL_NOT_CONFIGURED`(8009)、`PASSWORD_RESET_DISABLED`(8010) |
| 81xx | 存储相关 | `STORAGE_NOT_CONFIGURED`(8101)、`STORAGE_ERROR`(8102)、`STORAGE_FILE_TOO_LARGE`(8103)、`STORAGE_FILE_TYPE_NOT_ALLOWED`(8104) |
