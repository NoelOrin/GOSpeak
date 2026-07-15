# 权限体系文档

**更新时间**: 2026-07-10

---

## 一、权限码定义

权限码定义在 `internal/permcode/permcode.go`，所有权限均通过 `RequirePermission` 中间件检查。

| 权限码 | 说明 | 文件位置 |
|--------|------|----------|
| `room:create` | 创建语音房间 | `internal/permcode/permcode.go` |
| `room:read` | 查看房间列表和详情 | |
| `room:update` | 修改房间名称、人数上限等 | |
| `room:delete` | 删除房间 | |
| `user:read` | 查看用户列表和详情 | |
| `user:update` | 修改用户信息 | |
| `user:delete` | 删除用户账号 | |
| `role:read` | 查看角色列表 | |
| `role:manage` | 创建、删除角色和分配权限 | |
| `signal:kick` | 将用户从语音房间中踢出 | |
| `mute:manage` | 对用户进行全局禁言/取消禁言 | |
| `sfu:manage` | 查看和修改 SFU 提供商配置 | |
| `bot:manage` | 创建、查看、吊销 Bot 专用 API Key | |

---

## 二、角色定义

| 角色 | 说明 | 默认权限 |
|------|------|----------|
| `admin` | 管理员，拥有全部权限 | 所有权限 |
| `user` | 普通用户 | `room:create`, `room:read`, `user:read`, `role:read` |
| `ban` | 被封禁用户 | 无任何权限 |

### 默认管理员账号

首次启动 `seedAdminUser` 会在库中不存在 `admin` 时自动创建：

| 字段 | 值 |
|------|-----|
| 用户名 | `admin` |
| 默认密码 | `admin123` |

常量定义：`service.DefaultAdminPassword`（`app/server/internal/service/auth_service.go`）。  
登录时若密码仍为默认值，返回 `need_change_password=true`，应走 `POST /api/v1/auth/first_change_password` 强制改密。**生产环境务必立即修改。**

---

## 三、默认角色权限矩阵

| 权限 | admin | user | ban |
|------|:-----:|:----:|:---:|
| **房间** | | | |
| `room:create` | ✅ | ✅ | ❌ |
| `room:read` | ✅ | ✅ | ❌ |
| `room:update` | ✅ | ❌ | ❌ |
| `room:delete` | ✅ | ❌ | ❌ |
| **用户** | | | |
| `user:read` | ✅ | ✅ | ❌ |
| `user:update` | ✅ | ❌ | ❌ |
| `user:delete` | ✅ | ❌ | ❌ |
| **角色** | | | |
| `role:read` | ✅ | ✅ | ❌ |
| `role:manage` | ✅ | ❌ | ❌ |
| **语音** | | | |
| `signal:kick` | ✅ | ❌ | ❌ |
| `mute:manage` | ✅ | ❌ | ❌ |
| **系统** | | | |
| `sfu:manage` | ✅ | ❌ | ❌ |
| `bot:manage` | ✅ | ❌ | ❌ |

---

## 四、中间件

### 4.1 RequirePermission

基于权限码的鉴权，检查用户角色是否拥有指定权限。

```go
middleware.RequirePermission(permcode.PermSFUManage)
```

### 4.2 RequireRole

基于角色的鉴权，检查用户角色是否匹配。

```go
middleware.RequireRole("admin")
```

### 4.3 RequireOwnerOrPermission

资源归属校验：拥有资源归属或拥有指定权限码即可放行。

```go
middleware.RequireOwnerOrPermission("room_owner", permcode.PermRoomUpdate)
```

### 4.4 BanCheck

检查用户角色是否为 `ban`，是则拒绝访问。

---

## 五、API 路由权限

### 5.1 公开路由（无需鉴权）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | 登录 |
| POST | `/api/v1/auth/register` | 注册 |
| POST | `/api/v1/auth/refresh_token` | 刷新 Token |
| POST | `/api/v1/auth/reset_password` | 重置密码 |
| POST | `/api/v1/signal/signal` | WebRTC 信令中继 |
| GET | `/api/v1/signal/rooms` | 获取房间列表 |
| GET | `/api/v1/signal/participants` | 获取房间参与者 |
| POST | `/api/v1/signal/webhook` | LiveKit Webhook |
| POST | `/api/v1/srs/callback` | SRS 回调 |
| GET | `/api/v1/oauth/login/:provider` | OAuth 登录 |
| GET | `/api/v1/oauth/callback/:provider` | OAuth 回调 |
| POST | `/api/v1/email/send_code` | 发送邮箱验证码 |
| POST | `/api/v1/email/verify_code` | 验证邮箱验证码 |

### 5.2 受保护路由（需 JWT 鉴权）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| **认证** | | | |
| POST | `/api/v1/auth/logout` | JWT | 登出 |
| POST | `/api/v1/auth/refresh` | JWT | 刷新 Token |
| POST | `/api/v1/auth/change_password` | JWT | 修改密码 |
| POST | `/api/v1/auth/first_change_password` | JWT | 首次修改密码 |
| **用户** | | | |
| POST | `/api/v1/user/profile` | JWT | 获取个人资料 |
| POST | `/api/v1/user/info` | JWT | 获取用户信息 |
| POST | `/api/v1/user/update-profile` | JWT | 更新个人资料 |
| POST | `/api/v1/user/upload-avatar` | JWT | 上传头像 |
| POST | `/api/v1/user/list` | `user:read` | 用户列表 |
| POST | `/api/v1/user/get` | `user:read` | 获取用户详情 |
| POST | `/api/v1/user/delete` | `user:delete` | 删除用户 |
| POST | `/api/v1/user/update-role` | `user:update` | 更新用户角色 |
| **房间** | | | |
| POST | `/api/v1/room/create` | `room:create` | 创建房间 |
| POST | `/api/v1/room/list` | `room:read` | 房间列表 |
| POST | `/api/v1/room/get` | `room:read` | 获取房间详情 |
| POST | `/api/v1/room/update` | `room:update` | 更新房间 |
| POST | `/api/v1/room/delete` | `room:delete` | 删除房间 |
| **角色** | | | |
| POST | `/api/v1/role/list` | `role:read` | 角色列表 |
| POST | `/api/v1/role/create` | `role:manage` | 创建角色 |
| POST | `/api/v1/role/update` | `role:manage` | 更新角色 |
| POST | `/api/v1/role/delete` | `role:manage` | 删除角色 |
| **权限** | | | |
| POST | `/api/v1/permission/list` | `role:read` | 权限列表 |
| POST | `/api/v1/permission/role` | `role:read` | 角色权限 |
| POST | `/api/v1/permission/sync` | `role:manage` | 同步角色权限 |
| **禁言** | | | |
| POST | `/api/v1/mute/create` | `mute:manage` | 创建禁言 |
| POST | `/api/v1/mute/cancel` | `mute:manage` | 取消禁言 |
| POST | `/api/v1/mute/status` | `mute:manage` | 禁言状态 |
| POST | `/api/v1/mute/list` | `mute:manage` | 禁言列表 |
| **SFU 配置** | | | |
| POST | `/api/v1/sfu/config` | `sfu:manage` | 获取配置 |
| POST | `/api/v1/sfu/config/:provider` | `sfu:manage` | 获取 Provider 配置 |
| POST | `/api/v1/sfu/update-config` | `sfu:manage` | 更新配置 |
| POST | `/api/v1/sfu/switch-provider` | `sfu:manage` | 切换 Provider |
| POST | `/api/v1/sfu/providers` | `sfu:manage` | Provider 列表 |
| **Bot** | | | |
| POST | `/api/v1/bot/create` | `bot:manage` | 创建 Bot Key |
| POST | `/api/v1/bot/list` | `bot:manage` | Bot Key 列表 |
| POST | `/api/v1/bot/revoke` | `bot:manage` | 吊销 Bot Key |
| **信令** | | | |
| POST | `/api/v1/signal/token` | JWT | 获取加入 Token |
| **存储** | | | |
| POST | `/api/v1/storage/presign` | JWT | 预签名上传 |
| POST | `/api/v1/storage/confirm` | JWT | 确认上传 |
| POST | `/api/v1/storage/upload` | JWT | 直接上传 |
| POST | `/api/v1/storage/delete` | admin | 删除对象 |
| POST | `/api/v1/storage/config` | admin | 获取配置 |
| POST | `/api/v1/storage/update-config` | admin | 更新配置 |
| **邮件配置** | | | |
| POST | `/api/v1/email_config/config` | admin | 获取配置 |
| POST | `/api/v1/email_config/update-config` | admin | 更新配置 |
| **OAuth 管理** | | | |
| GET | `/api/v1/oauth/providers` | `role:manage` | Provider 列表 |
| POST | `/api/v1/oauth/providers` | `role:manage` | 创建 Provider |
| PUT | `/api/v1/oauth/providers` | `role:manage` | 更新 Provider |
| DELETE | `/api/v1/oauth/providers/:id` | `role:manage` | 删除 Provider |

---

## 六、Socket.IO 事件权限

Socket.IO 连接通过 `WSAuth` 中间件鉴权（JWT）。

| 事件 | 权限 | 说明 |
|------|------|------|
| `connection` | JWT | 连接建立 |
| `disconnect` | - | 连接断开 |
| `room:create` | JWT | 创建房间 |
| `room:join` | JWT | 加入房间（含禁言检查） |
| `room:join_livekit` | JWT | LiveKit 连接确认 |
| `room:leave` | JWT | 离开房间 |
| `room:list` | JWT | 房间列表 |
| `room:kick` | `signal:kick` | 踢出房间 |

---

## 七、权限检查流程

```
请求进入
    │
    ▼
BanCheck 中间件
    │
    ├── role == "ban" → 401 USER_BANNED
    │
    ▼
权限中间件 (RequirePermission / RequireRole)
    │
    ├── 无权限 → 403 FORBIDDEN
    │
    ▼
Handler 处理
```

---

## 八、相关文件

| 文件 | 说明 |
|------|------|
| `internal/permcode/permcode.go` | 权限码常量定义 |
| `internal/model/permission.go` | 权限模型和默认权限配置 |
| `internal/middleware/auth.go` | 鉴权中间件实现 |
| `internal/repository/permission_repo.go` | 权限数据访问层 |
| `internal/handler/permission_handler.go` | 权限管理 Handler |
| `internal/handler/role_handler.go` | 角色管理 Handler |
