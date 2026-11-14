# 权限说明

GOSpeak 的权限分两层，这两层经常被混淆：

- **平台角色**：管理整个实例的后台，例如查看用户列表、修改 OAuth 配置
- **域角色**：只管理某一个语音域内部，例如在该域内踢人

两层互不影响。在 A 域拥有管理员身份，不代表能进入 B 域，也不代表能访问平台后台。

## 平台角色

启动时自动写入三个角色：

| 角色 | 权限范围 | 说明 |
|------|---------|------|
| `admin` | 全部权限 | 平台管理员 |
| `user` | 建房间、进房间、发消息、建域 | 普通用户，占绝大多数 |
| `ban` | 无任何权限 | 被封禁，中间件直接拦截 |

首次启动会自动创建 `admin` 账号（密码 `admin123`），部署后应立即修改该密码。

### 普通用户的权限列表

普通用户只有这几项：

```
room:create         建房间
room:read           看房间
domain:create       建域
message:send        发消息
message:read        看消息
announcement:read   看公告
```

特别注意**没有** `user:read` 和 `role:read`。这是刻意的设计——一旦授予这两项，所有用户的侧栏都会出现「管理」入口。

### 全部权限码

| 权限码 | 管理范围 |
|--------|--------|
| `room:create` / `room:read` / `room:update` / `room:delete` | 房间增删改查 |
| `user:read` / `user:update` / `user:delete` | 用户查询、改角色、删号 |
| `role:read` / `role:manage` | 角色与权限关联的读写 |
| `message:read` / `message:send` / `message:delete_others` | 消息读取、发送、删除他人消息 |
| `mute:manage` | 用户级禁言 |
| `signal:kick` | 从信令层踢人下线 |
| `sfu:manage` | 修改 SFU 配置 |
| `storage:read` / `storage:manage` / `storage:delete` | 对象存储配置与删除 |
| `email_config:read` / `email_config:manage` | 邮件配置 |
| `oauth:read` / `oauth:manage` | 第三方登录配置 |
| `domain:create` / `domain:read` / `domain:manage` / `domain:delete` | 域增删改查 |
| `domain:invite` | 生成邀请 |
| `domain:invite:manage` | 管理邀请码（重置/停用） |
| `domain:member:review` | 审批加入申请 |
| `domain:kick` | 从域内移除成员 |
| `domain:role:manage` | 管理域内角色 |
| `announcement:read` / `announcement:manage` | 系统公告读取 / 管理 |
| `audit:read` | 查看审计日志（默认未授予任何角色，需手动授权） |

### 修改角色权限

通过接口修改，需要 `role:manage` 权限：

```bash
POST /api/v1/permission/sync
```

前端在「管理 → 角色权限」页面勾选也能完成相同修改，效果一致。

## 域角色

每个域内部维护一套独立角色，与平台角色无关：

| 域角色 | 说明 |
|--------|------|
| `owner` | 建域的人，拥有最高权限，不能转让给其他成员，也无法被踢出域 |
| `admin` | 域管理员，可管理频道和成员 |
| `member` | 普通成员 |
| `guest` | 访客，可用功能由域的开关决定 |

一个人在不同域可以是不同角色。在自建域中是 owner，在其他人的域中可能是 member。

### 域内的管理范围

- 频道：建、改、删
- 成员：踢出、禁言
- 角色：为成员分配域内角色
- 邀请：生成邀请链接

具体操作对应哪个域角色，取决于域角色的权限配置。

## 访客

访客是「无需注册即可进入收听」的机制，适合公开社区。

域的设置里有三个开关，默认值如下：

| 开关 | 默认 | 说明 |
|------|------|------|
| 允许访客收听 | 开 | 关闭后访客无法进入 |
| 允许访客发言 | 开 | 关闭后访客只能收听 |
| 允许访客发消息 | 关 | 默认不允许访客在文字频道发言 |

访客是通过邀请码进入的临时账号（`is_guest=true`），有独立的封禁列表——封禁访客不影响其后续注册正式账号。

## 鉴权流程

每个请求依次经过以下几层：

```
请求
 ├─ JWTAuth()          验签、验过期、验令牌版本、验黑名单
 ├─ BanCheck()         ban 角色直接 403
 ├─ RequirePermission  按 role → 权限 映射查有没有
 └─ Handler
```

有些接口不需要权限码，但要求「是资源本人或持有某权限」，使用的是 `RequireOwnerOrPermission`——例如修改自己的资料任何用户都可以，修改他人的资料则需要 `user:update`。

## 改密码与旧令牌的关系

改密码会使旧令牌失效。`users.token_version` 存在于 JWT 中，改密码或重置密码会将该值加一，之前签发的所有令牌随之立即作废。

登出是另一套机制：将该次令牌的 JTI 写入内嵌 NATS 的 KV 黑名单，到期自动清理。

## 排查权限问题

**看不到「管理」入口**
平台角色不是 admin，或者角色缺少 `user:read` / `role:read`。

**接口返回 403 但角色配置无误**
查看返回的 code：`1013` 表示权限不足；`1001`-`1014` 范围内的认证类错误说明令牌本身有问题（过期、被吊销、版本不匹配）。

**域内管理操作全部不可用**
域角色是 `member` 或 `guest`，未被授予管理权限。

**被封禁账号的登录行为**
可以登录，但所有受保护接口返回 403。这是预期行为，登录环节本身不校验 `ban` 角色。

## 下一步

- [API 参考](/reference/api) — 每个接口的权限要求
- [OAuth 登录](/guide/oauth) — 第三方登录的配置方法
