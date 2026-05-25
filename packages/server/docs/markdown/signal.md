# Signal 信令模块

> 标签: `Signal` | 基础路径: `/api/v1/signal`

## 接口列表

| 方法 | 路径 | 说明 | 需鉴权 |
|------|------|------|--------|
| POST | `/api/v1/signal/token` | 获取 LiveKit 加入 Token | ❌ |
| POST | `/api/v1/signal/signal` | 信令中转 | ❌ |
| GET | `/api/v1/signal/rooms` | LiveKit 房间列表 | ❌ |
| GET | `/api/v1/signal/participants` | 房间参与者列表 | ❌ |

---

## POST /api/v1/signal/token

生成 LiveKit 房间加入 Token

### Request Body

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

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "room": "my-room",
    "identity": "user-123"
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

## POST /api/v1/signal/signal

中转 WebRTC 信令消息（offer/answer/ICE candidate）

### Request Body

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
| data | object | ❌ | 信令数据 |

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "type": "offer",
    "room": "my-room"
  }
}
```

---

## GET /api/v1/signal/rooms

获取 LiveKit 中所有活跃房间列表

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "name": "my-room",
      "participantCount": 2
    }
  ]
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

## GET /api/v1/signal/participants

获取指定房间的所有参与者

### Query Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| room | string | ✅ | 房间名称 |

### 请求示例

```
GET /api/v1/signal/participants?room=my-room
```

### Response (200)

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "identity": "user-123",
      "name": "user-123"
    }
  ]
}
```

### Response (400)

```json
{
  "code": 2001,
  "message": "room is required",
  "data": null
}
```