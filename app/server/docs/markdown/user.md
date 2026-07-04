# User 用户管理模块

> 标签: `User` | 基础路径: `/api/v1/user`

## 接口列表

| 方法 | 路径 | 说明 | 需鉴权 |
|------|------|------|--------|
| GET | `/api/v1/user/profile` | 获取个人信息 | ✅ |
| GET | `/api/v1/user/list` | 用户列表（分页） | ✅ |
| GET | `/api/v1/user/{id}` | 获取用户详情 | ✅ |
| DELETE | `/api/v1/user/{id}` | 删除用户 | ✅ |

---

## GET /api/v1/user/profile

获取当前登录用户的个人信息

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
    "username": "john",
    "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  }
}
```

### Response (404)

```json
{
  "code": 2003,
  "message": "用户不存在",
  "data": null
}
```

---

## GET /api/v1/user/list

获取用户列表（分页）

### Headers

| 字段 | 值 |
|------|-----|
| Authorization | Bearer \<token\> |

### Query Parameters

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| page | int | ❌ | 1 | 页码 |
| page_size | int | ❌ | 20 | 每页数量（最大 100） |

### 请求示例

```
GET /api/v1/user/list?page=1&page_size=20
```

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      {
        "id": 1,
        "uuid": "xxx",
        "name": "john",
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1
  }
}
```

### Response (500)

```json
{
  "code": 5001,
  "message": "服务器内部错误",
  "data": null
}
```

---

## GET /api/v1/user/{id}

根据 ID 获取用户详情

### Headers

| 字段 | 值 |
|------|-----|
| Authorization | Bearer \<token\> |

### Path Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | int | ✅ | 用户 ID |

### 请求示例

```
GET /api/v1/user/1
```

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "uuid": "xxx",
    "name": "john",
    "created_at": "2025-01-01T00:00:00Z",
    "updated_at": "2025-01-01T00:00:00Z"
  }
}
```

### Response (404)

```json
{
  "code": 2003,
  "message": "用户不存在",
  "data": null
}
```

---

## DELETE /api/v1/user/{id}

根据 ID 删除用户

### Headers

| 字段 | 值 |
|------|-----|
| Authorization | Bearer \<token\> |

### Path Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | int | ✅ | 用户 ID |

### 请求示例

```
DELETE /api/v1/user/1
```

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

### Response (500)

```json
{
  "code": 5001,
  "message": "删除失败",
  "data": null
}
```