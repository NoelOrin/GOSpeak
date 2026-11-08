# 权限说明

GOSpeak 的权限分两层，很多人一开始会搞混：

- **平台角色**：管整个实例的后台，比如看用户列表、改 OAuth 配置
- **域角色**：只管某一个语音域内部，比如在这个域里踢人

两层互不影响。你在 A 域是管理员，不代表你能进 B 域，也不代表你能看平台后台。

## 平台角色

启动时自动写入三个角色：

| 角色 | 能做什么 | 说明 |
|------|---------|------|
| `admin` | 全部权限 | 平台管理员 |
| `user` | 建房间、进房间、发消息、建域 | 普通用户，绝大多数人都是这个 |
| `ban` | 什么都做不了 | 被封禁，中间件直接拦截 |

第一次启动会自动创建 `admin` 账号（密码 `admin123`），记得改。

### 普通用户有哪些权限

普通用户只有这几项：

```
room:create    建房间
room:read      看房间
domain:create  建域
message:send   发消息
message:read   看消息
```

特别注意**没有** `user:read` 和 `role:read`。这是故意的——给了这两项，所有人在侧栏都能看到「管理」入口。

### 全部权限码

| 权限码 | 管什么 |
|--------|--------|
| `room:create` / `room:read` / `room:update` / `room:delete` | 房间增删改查 |
| `user:read` / `user:update` / `user:delete` | 用户查询、改角色、删号 |
| `role:read` / `role:manage` | 角色与权限关联的读写 |
| `message:read` / `message:send` / `message:delete_others` | 消息读取、发送、删别人的 |
| `mute:manage` | 用户级禁言 |
| `signal:kick` | 从信令层踢人下线 |
| `sfu:manage` | 改 SFU 配置 |
| `storage:read` / `storage:manage` / `storage:delete` | 对象存储配置与删除 |
| `email_config:read` / `email_config:manage` | 邮件配置 |
| `oauth:read` / `oauth:manage` | 第三方登录配置 |
| `domain:create` / `domain:read` / `domain:manage` / `domain:delete` | 域增删改查 |
| `domain:invite` | 生成邀请 |
| `domain:kick` | 从域里踢人 |
| `domain:role:manage` | 管域内角色 |
| `audit:read` | 看审计日志（默认没给任何人，需要手动授） |

### 改某个角色的权限

走接口，需要 `role:manage`：

```bash
POST /api/v1/permission/sync
```

前端在「管理 → 角色权限」里点勾选也能改，效果一样。

## 域角色

每个域内部独立一套，跟平台角色没关系：

| 域角色 | 说明 |
|--------|------|
| `owner` | 建这个域的人，最高权限，不能转让给别人踢掉 |
| `admin` | 域管理员，能管频道、管成员 |
| `member` | 普通成员 |
| `guest` | 访客，能做什么由域的开关决定 |

一个人在不同域可以是不同角色。你在自己的域是 owner，去朋友的域就是 member。

### 域里能管什么

- 频道：建、改、删
- 成员：踢出、禁言
- 角色：给成员分配域内角色
- 邀请：生成邀请链接

具体谁能做，看域角色的权限配置。

## 访客

访客是「不用注册也能进来听」的机制，适合公开社区。

域的设置里有三个开关，默认值如下：

| 开关 | 默认 | 说明 |
|------|------|------|
| 允许访客收听 | 开 | 关掉后访客进不来 |
| 允许访客发言 | 开 | 关掉后访客只能听 |
| 允许访客发消息 | 关 | 默认不让访客在文字频道说话 |

访客是通过邀请码进来的临时账号（`is_guest=true`），有独立的封禁列表——封了访客不影响他注册正式账号。

## 鉴权是怎么走的

每个请求进来会依次过这几道：

```
请求
 ├─ JWTAuth()          验签、验过期、验令牌版本、验黑名单
 ├─ BanCheck()         ban 角色直接 403
 ├─ RequirePermission  按 role → 权限 映射查有没有
 └─ Handler
```

有些接口不需要权限码，但要求「是资源本人或持有某权限」，用的是 `RequireOwnerOrPermission`——比如改自己的资料谁都可以，改别人的就要 `user:update`。

## 改密码会让旧令牌失效吗

会。`users.token_version` 存在 JWT 里，改密码或重置密码会把它加一，之前签发的所有令牌立刻作废。

登出是另一套机制：把这次令牌的 JTI 写进 NATS KV 黑名单，到期自动清理。

## 排查权限问题

**看不到「管理」入口**
平台角色不是 admin，或者角色缺 `user:read` / `role:read`。

**接口返回 403 但角色看着没问题**
看返回的 code：`1013` 是权限不足，`1001`-`1014` 里的认证类错误说明令牌本身有问题（过期、被吊销、版本不匹配）。

**域里什么都点不了**
域角色是 `member` 或 `guest`，没给你管理权限。

**被封了还能登录？**
能登录但所有受保护接口返回 403。这是预期行为，登录本身不校验 `ban` 角色。

## 下一步

- [API 参考](/reference/api) — 每个接口要求什么权限
- [OAuth 登录](/guide/oauth) — 第三方登录怎么配
