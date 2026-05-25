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

---

## AuthResponse

认证响应结构

```json
{
  "code": 0,
  "message": "success",
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
| message | string | 提示信息 |
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
  "message": "success",
  "data": null
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| code | ErrCode | 业务状态码（0 表示成功） |
| message | string | 提示信息 |
| data | any | 业务数据（可为 null） |

---

## ErrCode

业务状态码枚举

| code | 名称 | 说明 |
|------|------|------|
| 0 | SUCCESS | 请求成功 |
| 1 | ERROR | 通用错误 |
| 1001 | TOKEN_NOT_EXIST | Token 不存在 |
| 1002 | TOKEN_WRONG | Token 无效 |
| 1003 | TOKEN_RUNTIME | Token 过期 |
| 2001 | INVALID_PARAMS | 请求参数错误 |
| 2002 | UNAUTHORIZED | 未授权 |
| 2003 | NOT_FOUND | 资源不存在 |
| 5001 | INTERNAL_ERROR | 服务器内部错误 |