# Models 数据模型

> 接口通用数据模型定义

---

## model.User

用户模型

```json
{
  "id": 1,
  "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "name": "john",
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z"
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | integer | 用户 ID（自增主键） |
| uuid | string | 用户唯一标识 |
| name | string | 用户名 |
| created_at | string | 创建时间 (ISO 8601) |
| updated_at | string | 更新时间 (ISO 8601) |

> 注：此为 `model.User` 的**简化视图**。完整模型还包含 `display_name`、`avatar`、`email`、`email_verified`、`is_bot`（已废弃，Bot 模块已移除）等字段，详见 `internal/model/user.go`。`AuthResponse.user` 同样为简化视图，二者字段对齐。

---

## AuthResponse

认证响应结构

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "uuid": "xxx",
      "name": "john",
      "created_at": "2025-01-01T00:00:00Z",
      "updated_at": "2025-01-01T00:00:00Z"
    }
  }
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| code | ErrCode | 业务状态码 |
| msg | string | 提示信息 |
| data.token | string | JWT access token |
| data.refresh_token | string | 刷新 token |
| data.user | User | 用户信息 |

---

## LoginRequest / RegisterRequest

登录/注册请求

```json
{
  "username": "john",
  "password": "123456"
}
```

**字段说明:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | ✅ | 用户名 |
| password | string | ✅ | 密码 |

---

## RefreshTokenRequest

刷新 Token 请求

```json
{
  "refresh_token": "eyJ..."
}
```

**字段说明:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| refresh_token | string | ✅ | 刷新令牌 |

---

## JoinRoomRequest

加入房间请求

```json
{
  "room": "my-room",
  "identity": "user-123"
}
```

**字段说明:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| room | string | ✅ | 房间名称 |
| identity | string | ✅ | 用户身份标识 |

---

## SignalRequest

信令消息请求

```json
{
  "type": "offer",
  "room": "my-room",
  "identity": "user-123",
  "data": {}
}
```

**字段说明:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| type | string | ✅ | 信令类型 (offer/answer/ice_candidate) |
| room | string | ❌ | 房间名称 |
| identity | string | ❌ | 用户身份标识 |
| data | object | ❌ | 信令数据体 |

---

## Response

通用响应结构

```json
{
  "code": 0,
  "msg": "success",
  "data": null
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| code | ErrCode | 业务状态码（0 表示成功） |
| msg | string | 提示信息 |
| data | any | 业务数据（可为 null） |

---

## ErrCode

业务状态码枚举

| code | 名称 | 说明 |
|------|------|------|
| 0 | SUCCESS | 请求成功 |
| 1001 | TOKEN_NOT_EXIST | 未提供 Token |
| 1002 | TOKEN_WRONG | Token 无效 |
| 1003 | TOKEN_EXPIRED | Token 过期 |
| 1010 | INVALID_PASSWORD | 密码错误 |
| 1011 | USER_NOT_FOUND | 用户不存在 |
| 1012 | USERNAME_EXISTS | 用户名已存在 |
| 1013 | FORBIDDEN | 禁止访问（权限不足） |
| 1014 | TOKEN_REVOKED | Token 已吊销（黑名单 / 版本失效） |
| 1015 | USER_BANNED | 用户被封禁 |
| 1016 | USER_MUTED | 用户被禁言 |
| 1017 | RATE_LIMITED | 触发限流 |
| 2001 | INVALID_PARAMS | 请求参数错误 |
| 3001 | NOT_FOUND | 资源不存在 |
| 3002 | ALREADY_EXISTS | 资源已存在 |
| 5001 | INTERNAL_ERROR | 服务器内部错误 |
| 6001 | SFU_NOT_CONFIGURED | SFU 未配置 |
| 6002 | SFU_ERROR | SFU 错误 |
| 7001 | OAUTH_PROVIDER_NOT_FOUND | OAuth 提供商不存在 |
| 7002 | OAUTH_PROVIDER_DISABLED | OAuth 提供商已禁用 |
| 7003 | OAUTH_TOKEN_EXCHANGE_FAILED | OAuth Token 交换失败 |
| 7004 | OAUTH_GET_USER_FAILED | OAuth 获取用户信息失败 |
| 8001 | EMAIL_ALREADY_EXISTS | 邮箱已存在 |
| 8002 | EMAIL_CODE_INVALID | 邮箱验证码无效 |
| 8003 | EMAIL_CODE_EXPIRED | 邮箱验证码过期 |
| 8004 | EMAIL_CODE_ALREADY_USED | 邮箱验证码已使用 |
| 8005 | EMAIL_SEND_TOO_FREQUENT | 邮箱发送过于频繁 |
| 8006 | EMAIL_SEND_FAILED | 邮箱发送失败 |
| 8007 | EMAIL_NOT_VERIFIED | 邮箱未验证 |
| 8008 | EMAIL_CODE_MAX_ATTEMPTS | 邮箱验证码尝试超限 |
| 8009 | EMAIL_NOT_CONFIGURED | 邮箱服务未配置 |
| 8010 | PASSWORD_RESET_DISABLED | 密码重置已禁用 |
| 8101 | STORAGE_NOT_CONFIGURED | 存储未配置 |
| 8102 | STORAGE_ERROR | 存储错误 |
| 8103 | STORAGE_FILE_TOO_LARGE | 文件过大 |
| 8104 | STORAGE_FILE_TYPE_NOT_ALLOWED | 文件类型不允许 |