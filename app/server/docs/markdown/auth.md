# Auth 认证模块

> 标签: `Auth` | 基础路径: `/api/v1/auth`

## 接口列表

| 方法 | 路径 | 说明 | 需鉴权 |
|------|------|------|--------|
| POST | `/api/v1/auth/login` | 用户登录 | ❌ |
| POST | `/api/v1/auth/register` | 用户注册 | ❌ |
| POST | `/api/v1/auth/refresh_token` | 刷新 Token | ❌ |
| POST | `/api/v1/auth/logout` | 退出登录 | ✅ |
| POST | `/api/v1/auth/refresh` | 刷新当前 Token | ✅ |

---

## POST /api/v1/auth/login

用户登录，返回 JWT Token

### Request Body

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

### Response (200)

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

### Response (400)

```json
{
  "code": 2001,
  "message": "参数错误详情",
  "data": null
}
```

---

## POST /api/v1/auth/register

注册新用户

### Request Body

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

### Response (200)

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

### Response (400)

```json
{
  "code": 1,
  "message": "错误详情",
  "data": null
}
```

---

## POST /api/v1/auth/refresh_token

使用 refresh_token 获取新的 access token

### Request Body

```json
{
  "refresh_token": "eyJ..."
}
```

**字段说明:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| refresh_token | string | ✅ | 刷新令牌 |

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs..."
  }
}
```

### Response (401)

```json
{
  "code": 1002,
  "message": "invalid refresh token",
  "data": null
}
```

---

## POST /api/v1/auth/logout

退出登录（需要 Bearer Token）

### Headers

| 字段 | 值 |
|------|-----|
| Authorization | Bearer \<token\> |

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

---

## POST /api/v1/auth/refresh

刷新当前用户的 Token（需要 Bearer Token）

### Headers

| 字段 | 值 |
|------|-----|
| Authorization | Bearer \<token\> |

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs..."
  }
}
```

### Response (401)

```json
{
  "code": 2002,
  "message": "未授权",
  "data": null
}
```