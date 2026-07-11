# API 参考

## 基础信息

- **Base URL**: `/api/v1`
- **响应格式**:

```json
// 成功
{ "code": 0, "msg": "success", "data": { ... } }

// 错误
{ "code": 1011, "msg": "user not found", "data": null }
```

## 认证 API

### 登录

```
POST /api/v1/auth/login
```

```json
// Request
{ "username": "demo", "password": "123456" }

// Response
{
  "code": 0,
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "user": { "id": 1, "uuid": "...", "name": "demo", "role": "user" }
  }
}
```

### 注册

```
POST /api/v1/auth/register
```

```json
// Request
{ "username": "demo", "password": "123456" }

// Response: 同登录
```

### 刷新 Token

```
POST /api/v1/auth/refresh
Authorization: Bearer <refresh_token>
```

### 修改密码

```
POST /api/v1/auth/change_password
Authorization: Bearer <access_token>
```

```json
{ "old_password": "123456", "new_password": "654321" }
```

### 首次修改密码

```
POST /api/v1/auth/first_change_password
Authorization: Bearer <access_token>
```

```json
{ "new_password": "654321" }
```

### 登出

```
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
```

## 用户 API

### 获取用户资料

```
POST /api/v1/user/profile
Authorization: Bearer <access_token>
```

### 用户列表

```
POST /api/v1/user/list
Authorization: Bearer <access_token>
```

### 获取用户（按 ID）

```
POST /api/v1/user/get
Authorization: Bearer <access_token>
```

```json
{ "id": 1 }
```

### 删除用户（Admin）

```
POST /api/v1/user/delete
Authorization: Bearer <access_token>  # role: admin
```

```json
{ "id": 1 }
```

### 更新角色（Admin）

```
POST /api/v1/user/update-role
Authorization: Bearer <access_token>  # role: admin
```

```json
{ "id": 1, "role": "admin" }
```

## 信令 API

### 获取加入 Token

```
POST /api/v1/signal/token
```

```json
// Request
{ "room": "lobby", "identity": "user-xxx" }

// Response
{
  "code": 0,
  "data": {
    "token": "eyJ...",
    "serverUrl": "wss://...",
    "room": "lobby",
    "identity": "user-xxx"
  }
}
```

SRS 模式下还包含：

```json
{
  "token": "...",
  "serverUrl": "http://localhost",
  "whipUrl": "/rtc/v1/whip/",
  "stream": "room-user-xxx",
  "room": "lobby",
  "identity": "user-xxx"
}
```

### 信令消息

```
POST /api/v1/signal/signal
```

### 列出房间

```
GET /api/v1/signal/rooms
```

### 列出参与者

```
GET /api/v1/signal/participants?room=lobby
```

### LiveKit Webhook

```
POST /api/v1/signal/webhook
```

## SFU 配置 API

### 获取配置

```
POST /api/v1/sfu/config
Authorization: Bearer <access_token>  # perm: sfu_manage
```

### 更新配置

```
POST /api/v1/sfu/update-config
Authorization: Bearer <access_token>  # perm: sfu_manage
```

## OAuth API

### OAuth 登录（重定向）

```
GET /api/v1/oauth/login/:provider
```

示例：`GET /api/v1/oauth/login/github`

### OAuth 回调

```
GET /api/v1/oauth/callback/:provider?code=xxx&state=yyy
```

### 列出 Provider（Admin）

```
GET /api/v1/oauth/admin/providers
Authorization: Bearer <access_token>  # admin
```

### 创建 Provider（Admin）

```
POST /api/v1/oauth/admin/providers
Authorization: Bearer <access_token>  # admin
```

```json
{
  "name": "github",
  "client_id": "xxx",
  "client_secret": "xxx",
  "redirect_url": "https://example.com/api/v1/oauth/callback/github",
  "enabled": true
}
```

### 更新 Provider（Admin）

```
PUT /api/v1/oauth/admin/providers
Authorization: Bearer <access_token>  # admin
```

### 删除 Provider（Admin）

```
DELETE /api/v1/oauth/admin/providers/:id
Authorization: Bearer <access_token>  # admin
```

## SRS Callback

SRS HTTP Hooks 回调地址（由 SRS 配置 `http_hooks` 调用）：

```
POST /api/v1/srs/callback
```

## 其他

### 健康检查

```
GET /ping
```

### Swagger 文档

```
GET /swagger/index.html
```

## WebSocket 事件

**端点**: `ws://<host>/socket.io/`

### 客户端 → 服务端

| 事件 | 数据 | 说明 |
|------|------|------|
| `room:create` | `{ room }` | 创建房间 |
| `room:join` | `{ room, identity }` | 加入房间 |
| `room:leave` | `{ room }` | 离开房间 |
| `room:list` | — | 请求房间列表 |

### 服务端 → 客户端

| 事件 | 数据 | 说明 |
|------|------|------|
| `room:created` | `RoomInfo` | 房间已创建 |
| `room:joined` | `{ room, members[] }` | 已加入房间 |
| `room:left` | `{ room, identity }` | 已离开房间 |
| `room:updated` | `RoomInfo` | 房间更新 |
| `member:joined` | `MemberInfo` | 成员加入 |
| `member:left` | `{ identity }` | 成员离开 |
| `member:updated` | `MemberInfo` | 成员更新 |
| `room:list:result` | `{ rooms[] }` | 房间列表 |

## 错误码

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 1001-1014 | 认证错误（Token 不存在/错误/过期/撤销等）|
| 1010 | 密码错误 |
| 1011 | 用户不存在 |
| 1012 | 用户名已存在 |
| 1013 | 禁止访问 |
| 2001 | 参数错误 |
| 3001 | 资源不存在 |
| 3002 | 资源已存在 |
| 5001 | 服务器内部错误 |
| 6001-6002 | SFU 错误 |
| 7001-7004 | OAuth 错误 |
