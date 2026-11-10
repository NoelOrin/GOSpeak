# 邀请码注册与加入审核 — 设计规格

日期：2026-11-10
状态：待用户审查
关联：取代 `2026-08-25-guest-access-permissions-design.md`（访客访问体系下线）

## 1. 背景与目标

### 1.1 问题

当前邀请链接的主路径是「填昵称 → 以访客身份进入」，访客体系存在三个结构性问题：

1. **不可达**：`/guest` 全仓唯一入口是 `pages/invite/d/$code.tsx` 的未登录 redirect；登录页无访客按钮、发现页无访客入口、`/guest?domain=` 无任何 UI 暴露。设计规格承诺的「设备级持久身份」「登录页访客按钮」「公开域一键进入」均未实现。
2. **治理失效**：`GuestService.Join` 不检查 `domain_guest_bans`（`guest_service.go:67-113`），而前端「以 XX 继续」按钮调用的是 `guestJoin`（新建身份）而非 `guestRenew`（复用身份，前端零调用点）。正常使用路径本身每次更换 UUID，封禁无意义。
3. **数据破坏**：`CleanupInactiveGuests` 无条件删除本域全部 guest 成员的 `domain_members` 行（`guest_service.go:356-369`），时间条件仅作用于 `users` 行删除。管理员点一次「清理 30 天未活跃访客」，域内所有访客立即失去成员身份，`users` 行残留成僵尸身份。

同时，产品需要一条「邀请链接 → 注册 → 管理员审核 → 成为正式成员」的增长与治理链路，而访客体系是这条链路的**绕过通道**：邀请码可配置「需审核」，但访客路径（仅校验 `domain.allow_guest`）不经过审核。

### 1.2 目标

| 编号 | 目标 |
|---|---|
| G1 | 下线访客体系，删除其代码、表、字段，消除上述三类结构性问题 |
| G2 | 邀请码升级为独立实体：自包含签名串、可配过期与使用上限、可禁用、可审计 |
| G3 | 邀请链接落地页支持内联注册，注册即可按域策略直接入域或提交审核申请 |
| G4 | 域级「加入需审核」开关 + 管理端审核队列 |
| G5 | 全链路中文错误文案，不再泄漏后端英文 msg |

### 1.3 已确认的产品决策

1. 访客体系**整体删除**（代码 + 表 + 字段），不做降级保留
2. 邀请链接为主路径，公开域发现为辅
3. 邀请码 = **自包含签名串**，过期时间与次数上限写在码内，验签即判过期，不查库
4. 邀请码**不做旧数据迁移**，旧 8 位码与旧链接全部作废
5. 审核开关**仅在域级**（`domains.require_join_approval`），所有码统一遵守，杜绝绕过
6. 审核权限为新增权限码 `domain:member:review`，owner/admin 默认持有
7. 邀请码管理权限为新增权限码 `domain:invite:manage`，owner/admin 默认持有
8. 审核期间用户**进不了域**，落地页展示申请状态卡片，可撤销、被拒后可重新申请
9. 审核结果**被动发现**：用户下次访问时看到状态变化，不做站内通知或邮件
10. 邀请码审计记使用记录与失败尝试，**不记 IP**，**不记码的修改历史**，**管理操作不进 audit 表**
11. 新建域自动生成一个默认邀请码（永不过期、不限次数）
12. 删除现有的「重置邀请码」功能，改用「禁用」语义
13. 邀请码管理页**改造现有 `/link` 页**，不新增路由
14. 不实现待审数量徽标
15. 公开域允许**已登录用户**免码申请加入，但仍受域级审核约束；未登录者看不到发现页（`/discover` 与 `list-public` 均需 JWT）

## 2. 删除范围

### 2.1 后端删除文件

| 文件 | 行数 |
|---|---|
| `app/server/internal/service/guest_service.go` | 420 |
| `app/server/internal/handler/guest_handler.go` | 236 |
| `app/server/internal/middleware/guest.go` | 145 |
| `app/server/internal/model/guest_ban.go` | 20 |
| `app/server/internal/repository/guest_ban_repo.go` | 46 |
| `app/server/internal/router/routes/guest/routes.go` | 26 |
| 访客相关测试文件（`guest_service_test.go`、`guest_handler_test.go`、`guest_test.go`、`guest_ban_repo_test.go`、`hub_guest_guard_test.go` 等） | 3323 |

### 2.2 后端接线点

| 文件 | 删除内容 |
|---|---|
| `app/server/server/gin.go` | `NewGuestService`、`SetGuestJoinGuard`、`SetGuestSpeakPolicy`、`SetGuestMessagePolicy`、`SetOnlineCounter`、`SetOnlineGuestResolver`、`GuestChecker` 注入、`NewGuestHandler`（约 27 处） |
| `app/server/internal/signal/hub.go` | `guestJoinGuard` / `guestSpeakPolicy` / `guestMessagePolicy` 字段与 setter（约 15 处） |
| `app/server/internal/signal/hub_room_join.go` | 两处 guard 调用、发言人策略整段（约 20 处） |
| `app/server/internal/signal/message_bridge.go` | `guestMessagePolicy` 分支（3 处） |
| `app/server/internal/service/sfu_service.go` | `SetGuestPolicy`、`guestIs`、`guestCaps`、`canPublish` 分支（11 处） |
| `app/server/internal/handler/message_handler.go` | `SetGuestPolicy` 及其调用（13 处） |
| `app/server/internal/model/domain.go` | `AllowGuest`、`GuestCanListen`、`GuestCanSpeak`、`GuestCanMessage`、`GuestLimit` 五个字段、`DefaultGuestLimit` 常量、`BeforeCreate` 中的相关逻辑 |
| `app/server/internal/model/user.go` | `IsGuest` 字段、`GuestNamePrefix`、`IsGuestName` |
| `app/server/internal/model/domain_role.go` | `DomainRoleGuest` 常量、权限矩阵条目、`IsSystemDomainRole` 分支 |
| `app/server/internal/repository/domain_role_repo.go` | seed 列表中的 guest |
| `app/server/internal/service/domain_service.go` | `domainRoleLevel` 的 guest 分支、`GuestLimit` 初始化 |
| `app/server/internal/router/router.go` | `guestRoutes.Register`、`guestRoutes.RegisterProtected`、`GuestGuard()` 挂载、`Guest` handler 字段 |
| `app/server/internal/repository/db_migrations.go` | AutoMigrate 列表中的 `&model.DomainGuestBan{}` |

### 2.3 前端删除文件

| 文件 | 行数 |
|---|---|
| `app/web/src/pages/guest.tsx` | 211 |
| `app/web/src/stores/guestStore.ts` | 43 |
| `app/web/src/components/domain/GuestAccessSettings.tsx` | 165 |
| `app/web/src/components/domain/GuestManageTable.tsx` | 82 |
| `app/web/src/api/guest.ts` | 152 |
| `app/web/src/stores/guestStore.test.ts`、`app/web/src/api/guest.spec.ts` | — |

### 2.4 前端接线点

| 文件 | 删除内容 |
|---|---|
| `app/web/src/api/apiClient.ts` | 公开路径白名单中的 `/guest` |
| `app/web/src/pages/invite/d/$code.tsx` | redirect 到 `/guest` 的分支（该页整体重写，见 §6.1） |
| `app/web/src/stores/domainStore.ts` | `syncGuestCaps` 函数、`activateDomain` 中的调用、`isGuest` import |
| `app/web/src/components/userBar.tsx` | `isGuest()` / `guestCaps()` 判断 |
| `app/web/src/components/textRoom/MessageInput.tsx` | 同上 |
| `app/web/src/hooks/useGlobalShortcuts.ts` | 同上 |
| `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx` | 访客访问分区、访客封禁管理分区、5 个 state、3 个 handler、清理访客确认弹窗（约 66 处） |
| `app/web/src/components/domain/DomainMemberTable.tsx` | guest 角色相关展示（如有） |

### 2.5 数据层删除

| 对象 | 处置 |
|---|---|
| 表 `domain_guest_bans` | 删除 |
| `domains.allow_guest` | 删列 |
| `domains.guest_can_listen` | 删列 |
| `domains.guest_can_speak` | 删列 |
| `domains.guest_can_message` | 删列 |
| `domains.guest_limit` | 删列 |
| `users.is_guest` | 删列 |
| `domains.invite_code` | 删列（由新的 `domain_invites` 表取代） |

**不写迁移代码**：部署环境无存量访客数据与存量邀请码。删列与删表通过移除 AutoMigrate 注册并显式 DROP 完成。

### 2.6 随删除一并消除的缺陷

以下问题不需要单独修复，随访客代码删除而消失：

| 缺陷 | 证据 |
|---|---|
| 清理任务删除全部访客的成员关系 | `guest_service.go:356-369` |
| 封禁可被「重新 Join」绕过 | `guest_service.go:67-113` 无 ban 检查 |
| 设备级持久身份未实现 | 设计规格 §3 承诺 vs 无 localStorage |
| 能力开关非响应式，禁说延迟生效 | `userBar.tsx:28`、`MessageInput.tsx:23`、`useGlobalShortcuts.ts:42` 均为渲染期 `getState()` |
| 能力开关 fail-closed 与后端不一致 | `domainStore.ts:179-188` vs `sfu_service.go:63-70` |
| 访客可转发邀请码绕过审核 | `DomainMenu.tsx:119-149` |
| 6 条访客英文错误文案泄漏 | `errorMessages.ts` 无 guest 词表 |
| `/auth/guest/renew` 死接口 | 前端零调用点 |
| 访客白名单 12 条 vs 99 条受保护路由，无测试守护 | `middleware/guest.go` |

### 2.7 保留项

| 项 | 理由 |
|---|---|
| `domain_members` 表结构与 `owner`/`admin`/`member` 角色 | 核心模型 |
| 现有 `member` 角色权限矩阵 | 不受影响 |
| 公开域发现（`is_public` + `ListPublic`） | 独立的发现能力，与访客无关；仅不再提供匿名进入 |

## 3. 邀请码设计

### 3.1 码结构

自包含签名串，全大写、无歧义字符集 `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`（32 字符，正好 base32），无填充。

```
┌─────────┬────────┬──────────┬────────┬──────────┐
│ 域前缀  │ codeID │ 过期时间 │ 次数上限│  HMAC    │
│  4 字节 │ 4 字节 │  4 字节  │ 2 字节 │  8 字节  │
└─────────┴────────┴──────────┴────────┴──────────┘
   22 字节 → base32 → 36 字符
```

| 段 | 内容 | 用途 |
|---|---|---|
| 域前缀 4B | `SHA-256(domainUUID)[:4]` | 验签后定位域，无需查库；不泄露真实 UUID |
| codeID 4B | `domain_invites.id`（uint32） | 查已用次数与禁用状态 |
| 过期时间 4B | Unix 时间戳（uint32），`0` = 永不过期 | 验签即判过期，不查库 |
| 次数上限 2B | uint16，`0` = 不限 | 管理页展示时无需查库 |
| HMAC 8B | `HMAC-SHA256(key, 前 14 字节)[:8]` | 防伪造、防篡改 |

**为什么 HMAC 取 8 字节**：64 位足以使随机碰撞概率可忽略；伪造者无法离线爆破（需要密钥）；同时把码长控制在 36 字符，兼顾可读性。

**字段上限**：`MaxUses` 编码为 uint16，最大 65535；`0` 表示不限。创建接口对 `max_uses > 65535` 返回参数错误。过期时间戳为 uint32 Unix 秒，上限 2106 年，`0` 表示永不过期。

**码长度校验**：新码固定 36 字符，字符集 `[A-HJ-NP-Z2-9]`（32 字符）。现有 `utils/domainInvite.ts:3-4` 的正则 `^[A-Z2-9]{8}$` 与 `app/desktop/deeplink.go:23` 的正则 `([A-Z2-9]{8})` 必须同步改为 `{36}`，且字符集需排除 `I`/`O`（现有正则允许 `I`、`O`，新码不会生成，但校验时应显式排除以免接受伪造形态）。

### 3.2 密钥派生

```go
inviteKey := HMAC-SHA256([]byte(cfg.StorageEncryptKey), []byte("gospeak:invite:v1"))
```

- 复用 `StorageEncryptKey`（生产必设 64 位 hex），不新增环境变量
- 固定 salt 前缀 `gospeak:invite:v1` 与存储加密用途隔离
- 版本后缀 `v1`：将来更换算法时追加 `v2`，旧码按码内版本路由仍可验
- `StorageEncryptKey` 为空（开发环境）时回退 `JWTKey` 并打 warn 日志

### 3.3 校验流程

```
1. base32 解码 + 长度校验        → 失败：码格式错误
2. HMAC 验签                     → 失败：码无效（不查库）
3. 过期时间 vs now               → 过期：链接已过期（不查库）
4. 按 codeID 查库
   ├─ 不存在                     → 码无效
   ├─ DisabledAt != nil          → 码已停用
   └─ UsedCount >= MaxUses       → 已达使用上限（上限来自码内，已用来自库）
5. 域是否存在、申请人是否已是成员
```

仅第 4-6 步需要查库，且为一次查询。

### 3.4 次数消耗的并发保护

成功入域或提交申请时，在同一事务内执行条件更新：

```sql
UPDATE domain_invites
   SET used_count = used_count + 1
 WHERE id = ? AND (max_uses = 0 OR used_count < max_uses) AND disabled_at IS NULL
```

`RowsAffected == 0` 表示次数已耗尽或码已被禁用，事务回滚并返回明确错误。该条件更新本身即原子并发保护，不引入分布式锁。

## 4. 数据模型

### 4.1 新表 `domain_invites`

```go
type DomainInvite struct {
    ID               uint       // pk，同时作为码内 codeID
    DomainUUID       string     // index
    Label            string     // 备注名，size:100
    Code             string     // 完整码串，uniqueIndex，size:64
    ExpiresAt        *time.Time // nil = 永不过期；创建时写入码内，之后不可改
    MaxUses          int        // 0 = 不限；创建时写入码内，之后不可改；上限 65535
    UsedCount        int        // 已成功使用次数
    DisabledAt       *time.Time // 非 nil = 已禁用
    CreatedBy        string     // 创建者 userUUID
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

索引：`code` 唯一；`(domain_uuid, disabled_at)` 复合。

### 4.2 新表 `domain_invite_redemptions`

```go
type DomainInviteRedemption struct {
    ID         uint
    InviteID   uint      // 关联 domain_invites.id；验签失败时为 0
    DomainUUID string    // 冗余，便于按域聚合
    UserUUID   string    // 成功时为用户；失败时可能为空
    Code       string    // 冗余存码串（截断 64 字符），码被删后审计仍可读
    Success    bool
    FailReason string    // "expired" | "exhausted" | "invalid" | "disabled" | "rejected" | "already_member"
    CreatedAt  time.Time
}
```

按已确认决策：不存 IP，不存码的修改历史。失败尝试在独立事务中写入，避免随主流程回滚。

### 4.3 新表 `domain_join_requests`

```go
type DomainJoinRequest struct {
    ID         uint
    DomainUUID string     // uniqueIndex: uniq_join_req_domain_user
    UserUUID   string     // uniqueIndex: uniq_join_req_domain_user
    InviteID   uint       // 申请使用的码（0 = 公开域直接申请）
    Status     string     // "pending" | "approved" | "rejected"
    ReviewedBy string     // 审核者 userUUID
    ReviewedAt *time.Time
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

唯一索引 `(domain_uuid, user_uuid)`：一个用户对一个域仅一条记录。

状态流转：

| 当前状态 | 触发动作 | 结果 |
|---|---|---|
| 无记录 | 申请 | INSERT，`status=pending` |
| `pending` | 再次申请 | 幂等返回，不报错，不新建记录 |
| `pending` | 撤销申请 | **物理删除该记录**，状态回到「无记录」 |
| `pending` | 审核通过 | `status=approved` + 插入 `domain_members` |
| `pending` | 审核拒绝 | `status=rejected` |
| `rejected` | 重新申请 | UPDATE 回 `pending`，刷新 `InviteID` |
| `approved` | 再次申请 | 校验 `domain_members` 是否仍存在：存在则幂等返回「已是成员」；不存在（曾离开域）则 UPDATE 回 `pending` |

撤销申请采用物理删除而非新增 `cancelled` 状态，避免状态机无谓膨胀：撤销后的记录没有审计价值（已确认管理操作不进 audit）。

### 4.4 `domains` 表变更

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `require_join_approval` | bool | false | 域级加入审核开关，所有邀请码统一遵守 |

删除字段见 §2.5。

## 5. 后端 API 与权限

### 5.1 权限码

| 权限码 | 含义 | owner | admin | member | 可分配 |
|---|---|---|---|---|---|
| `domain:invite:manage` | 创建/编辑/禁用邀请码、查看审计 | 有 | 有 | 无 | 是 |
| `domain:member:review` | 审核加入申请 | 有 | 有 | 无 | 是 |
| `domain:invite`（已有） | 查看/分享邀请码 | 有 | 有 | 无 | 是 |

改动点：`internal/permcode/domain_permcode.go` 新增 2 个常量；`model/domain_role.go` 的 `AssignableDomainPermissions` 与 `DefaultDomainRolePermissions[admin]` 各新增 2 项。owner 不存权限行，沿用现有约定。

### 5.2 公开路由

| Method | Path | 说明 |
|---|---|---|
| POST | `/api/v1/invite/preview` | 验码 → 返回域信息 + 是否需审核 + 剩余次数；失败一律不露域信息 |
| POST | `/api/v1/invite/register` | 内联注册：`{code, username, password, nickname?, email?, email_code?}` |

限流：各 `RateLimit(20, time.Hour)`，key 为 `ClientIP`。

### 5.3 认证路由

| Method | Path | 说明 |
|---|---|---|
| POST | `/api/v1/invite/join` | 已登录用户加入域。`{code}` 走邀请码；`{domain_uuid}` 走公开域直接申请。按域策略直接入域或提交申请 |
| POST | `/api/v1/invite/my-request` | 查询自己在该域的申请状态 |
| POST | `/api/v1/invite/cancel-request` | 撤销自己的待审申请 |

### 5.4 管理路由

| Method | Path | 权限 |
|---|---|---|
| POST | `/api/v1/domain/invites/list` | `domain:invite:manage` |
| POST | `/api/v1/domain/invites/create` | `domain:invite:manage` |
| POST | `/api/v1/domain/invites/disable` | `domain:invite:manage` |
| POST | `/api/v1/domain/invites/redemptions` | `domain:invite:manage` |
| POST | `/api/v1/domain/join-requests/list` | `domain:member:review` |
| POST | `/api/v1/domain/join-requests/review` | `domain:member:review` |
| POST | `/api/v1/domain/join-approval/config` | `domain:manage` |

全部使用 POST，沿用项目现有风格。

### 5.5 注册并加入流程

```
POST /api/v1/invite/register
  ├─ 1. 验码（§3.3 全流程）
  ├─ 2. 校验（复用 AuthService.Register 逻辑）
  │     ├─ username 不以 guest_ / bot_ 开头
  │     ├─ 密码强度 validateNewPassword
  │     ├─ 邮箱验证（跟随服务端 IsVerificationAvailable 开关）
  │     └─ username / email 唯一性
  ├─ 3. 事务：
  │     ├─ INSERT users
  │     ├─ 条件 UPDATE 消耗邀请码次数
  │     ├─ 写 redemption (success=1)
  │     └─ 分支：
  │         ├─ 域需审核 → INSERT domain_join_requests (status=pending)
  │         └─ 免审核   → INSERT domain_members (role=member)
  ├─ 4. 签发 JWT + 设 Cookie
  └─ 5. 返回 {user, domain, status: "joined" | "pending"}
```

免审核路径把 `users` + `domain_members` + 次数消耗放同一事务，避免「注册成功但入域失败」留下孤儿用户。

### 5.6 审核流程

```
POST /api/v1/domain/join-requests/review {domain_uuid, request_id, approve}
  ├─ 权限：domain:member:review
  ├─ 事务：
  │   ├─ 条件 UPDATE ... WHERE status='pending'（防重复审核）
  │   ├─ approve=true  → INSERT domain_members (role=member) + status=approved
  │   └─ approve=false → status=rejected
  └─ 返回
```

`RowsAffected == 0` 表示已被他人处理，返回明确错误而非静默成功。

### 5.7 错误码映射

| 场景 | 业务码 | 前端文案 |
|---|---|---|
| 码格式错误 / 验签失败 / 已过期 / 已停用 / 次数耗尽 | 2001 | 邀请链接无效、已过期、已停用、已达使用上限（各自独立文案） |
| 用户名已存在 | 1012 | 用户名已存在 |
| 密码强度不足 | 2001 | 密码至少 8 位 |
| 邮箱已被注册 | 8001 | 该邮箱已被注册 |
| 验证码错误 | 8002-8008 | 验证码无效或已过期 |
| 限流 | 1017 | 操作过于频繁，请稍后再试 |
| 无审核权限 | 1013 | 你没有审核权限 |
| 已是成员 | 不报错 | 正常跳转进域 |
| 已有待审申请 | 不报错 | 展示状态卡片 |

所有邀请码失败统一归 `2001`，不用 `1013`，避免被误读为账号封禁。

### 5.8 审计边界

| 内容 | 是否记录 | 位置 |
|---|---|---|
| 邀请码使用成功 | 是 | `domain_invite_redemptions` |
| 邀请码使用失败（无效/过期/耗尽/停用/被拒） | 是 | `domain_invite_redemptions` |
| IP 地址 | 否 | — |
| 邀请码修改历史 | 否 | — |
| 管理操作（建码/禁码/审核通过/审核拒绝） | 否 | — |

## 6. 前端设计

### 6.1 邀请落地页重写（`/invite/d/$code`）

现有页面「已登录才看域详情，未登录 redirect 到 `/guest`」重写为单页处理五种身份：

```
/invite/d/<code>
  ├─ POST /invite/preview（公开）
  │    ├─ 失败 → 单一错误卡片，不露域信息
  │    └─ 成功 → {domain, requires_approval, remaining_uses}
  └─ 身份分流：
       ├─ 未登录              → 内联注册表单（主）+「已有账号？登录」链接
       ├─ 已登录、已是成员     → 「进入域」
       ├─ 已登录、有 pending   → 申请审核中状态卡片
       ├─ 已登录、被拒过       → 申请未通过 + 「重新申请」
       └─ 已登录、非成员       → 「加入」按钮（按域策略）
```

成功后：`status=joined` 直接 navigate 到 `/domain/$domainUUID`；`status=pending` 切到状态卡片，不跳转。

内联注册表单字段：

| 字段 | 必填 | 说明 |
|---|---|---|
| 用户名 | 是 | 失焦时校验唯一性 |
| 密码 | 是 | 最少 8 位，复用 `PasswordInput` |
| 昵称 | 否 | 不填则用用户名，写入 `domain_members.nickname` |
| 邮箱 + 验证码 | 服务端开启邮箱验证时必填 | 复用 `sendEmailCode` |

主按钮文案区分：域免审核 →「注册并加入」；域需审核 →「注册并提交申请」。

### 6.2 申请状态卡片

审核期间进不了域。状态卡片展示域名与图标、当前状态、提交时间，以及「撤销申请」/「重新申请」和「返回首页」。

### 6.3 邀请码管理页（改造 `/link`）

```
/link
├─ 域选择器（复用现有）
└─ 选中域后：
   ├─ 邀请码列表
   │   列：备注名 | 码（截断，可展开） | 过期时间 | 已用/上限 | 状态 | 操作
   │   状态：生效中 / 已过期 / 已用完 / 已禁用
   │   操作：复制链接 | 二维码 | 查看使用记录 | 禁用
   ├─ 「新建邀请码」弹窗
   │   字段：备注名 | 有效期（1h/24h/7d/30d/永久/自定义） | 最大使用次数（不限/自定义）
   └─ 「查看使用记录」侧滑面板
       使用记录：用户 | 时间 | 结果（成功 / 失败原因），分页
```

删除现有「重置邀请码」按钮，改为「禁用」单个码。

### 6.4 域管理页改造

| 现有分区 | 处置 |
|---|---|
| 域设置 | 保留，新增「加入需审核」开关 |
| 成员管理 | 保留 |
| 访客访问 | 删除 |
| 访客封禁管理 | 删除 |
| 加入申请 | **新增**：待审/已通过/已拒绝分页 + 搜索 + 通过/拒绝操作 |
| 角色与权限 | 保留 |
| 房间管理 | 保留 |

### 6.5 公开域发现页

`/discover` 位于 `(app)` 路由下，保持现有登录门槛：未登录访问会跳 `/login`。`/api/v1/domain/list-public` 同样挂在 `protected` 组（`router.go:127`），需要 JWT。

**因此「能看到发现页的人必然已经注册」**——公开域免码加入不存在「未注册者绕过邀请码」的风险。

| 用户状态 | 路径 |
|---|---|
| 未登录 | `/discover` → `/login` → 登录后回跳 |
| 已登录、非成员 | `/discover` 点「加入」→ `/invite/join {domain_uuid}` → 免码，按域策略直接入域或提交申请 |
| 已登录、有 pending | 显示「申请审核中」，不可重复申请 |
| 已登录、已是成员 | 显示「进入域」 |

**接口分工**：

| 接口 | 入参 | 用途 |
|---|---|---|
| `/invite/register` | `code`（必填） | 新用户凭邀请链接注册 |
| `/invite/join` | `code` 或 `domain_uuid`（二选一） | 已登录用户入域 |

`/invite/register` **只接受 `code`**：注册是邀请链接的专属动作，公开域不是免码注册入口。新用户要么凭邀请链接注册，要么走 `/register` 普通注册后再从发现页加入。

**免码加入仍受域级审核约束**：`require_join_approval=true` 的公开域，免码加入同样走 `domain_join_requests` 待审流程，不存在绕过。

### 6.6 前端改动清单

| 文件 | 改动 |
|---|---|
| `pages/invite/d/$code.tsx` | 重写 |
| `pages/(app)/link.tsx` | 改造为邀请码管理中心 |
| `pages/(app)/manage/domains/$domainUUID/index.tsx` | 删访客 2 分区，加加入申请分区 |
| `api/invite.ts` | 新建 |
| `api/domain.ts` | 删 `resetDomainInviteCode`，`joinDomain` 适配 |
| `utils/domainInvite.ts` | 码正则与解析适配新长度 |
| `components/domain/InviteShareModal.tsx` | 适配新码 |
| `components/domain/DomainMenu.tsx` | 适配新码，删重置入口 |
| `api/apiClient.ts` | 公开白名单删 `/guest` |
| `pages/(app)/discover.tsx` | 加入流程适配 |
| `utils/errorMessages.ts` | 补邀请码与审核相关中文文案 |

### 6.7 桌面端同步

| 文件 | 改动 |
|---|---|
| `app/desktop/deeplink.go` | 邀请路径正则由 `[A-Z2-9]{8}` 改为新码长度 |
| `app/desktop/README.md` | 深链示例更新 |

## 7. 边界与一致性

### 7.1 并发场景

| 场景 | 处理 |
|---|---|
| 两人同时用最后 1 次额度 | 条件 UPDATE；`RowsAffected=0` 的一方回滚报「已达上限」 |
| 两个管理员同时审核同一申请 | 条件 `UPDATE ... WHERE status='pending'` |
| 同一用户用两个码同时申请 | `(domain_uuid, user_uuid)` 唯一索引，冲突转 UPDATE 回 pending |
| 注册成功但入域失败 | 同事务，整体回滚 |
| 审核通过时用户已被删除 | 校验 `users` 存在性，不存在则拒绝并标记申请 `rejected` |

### 7.2 安全边界

| 威胁 | 防护 |
|---|---|
| 伪造邀请码 | HMAC 验签，密钥从 `StorageEncryptKey` 派生 |
| 篡改码内过期时间或次数上限 | HMAC 覆盖前 14 字节 |
| 暴力枚举 | 36 字符 base32 + 64 位 HMAC，离线不可爆破；在线限流 20/h/IP |
| 通过无效码探测域存在性 | 失败一律不返回域信息 |
| 审核绕过 | 审核开关仅在域级，所有加入路径检查同一字段 |
| 越权管理邀请码 / 审核 | `domain:invite:manage` / `domain:member:review` 权限码 |

`/invite/preview` 验签通过后返回域名、图标、描述，用于落地页展示；这是有意为之。

## 8. 测试策略

### 8.1 后端单测

| 文件 | 覆盖 |
|---|---|
| `service/invite_code_test.go` | 生成、base32 编解码、验签、篡改检测、过期判断、各段边界值 |
| `service/invite_service_test.go` | preview 全分支、注册并加入、注册并待审、加入、撤销、禁用码、次数耗尽、并发超发（10 并发） |
| `service/join_request_service_test.go` | 提交、重复提交幂等、通过、拒绝、重复审核、被拒后重新申请 |
| `handler/invite_handler_test.go` | 限流、参数校验、错误码映射、Cookie 签发 |
| `handler/join_request_handler_test.go` | 权限校验、分页、搜索 |

### 8.2 后端集成测试

| 场景 | 断言 |
|---|---|
| 建码 → 注册 → 审核 → 进域 | `domain_members` 存在且 `role=member` |
| 免审核域：建码 → 注册 | 直接是成员，无 `domain_join_requests` 记录 |
| 过期码注册 | 2001，`users` 表无新行 |
| 次数用完 | 第 N 次成功、第 N+1 次失败 |
| 跨域隔离 | A 域的码不能加入 B 域 |
| 公开域免码加入（免审核） | `/invite/join {domain_uuid}` → 直接成为 `member`，无 `domain_join_requests` 记录 |
| 公开域免码加入（需审核） | `/invite/join {domain_uuid}` → 生成 `pending` 申请，不是成员 |
| 私有域免码加入 | `/invite/join {domain_uuid}` → 拒绝，提示需要邀请码 |
| 未登录调 `list-public` | 401，确认发现页无法匿名访问 |

### 8.3 前端单测

| 文件 | 覆盖 |
|---|---|
| `utils/inviteCode.spec.ts` | 新码正则、`extractDomainInviteCode`、链接解析 |
| `api/invite.spec.ts` | 各接口请求体与错误处理 |
| `pages/invite/__tests__/` | 五种身份分支的渲染 |

### 8.4 手工验收清单

1. 新建域 → 自动生成默认邀请码 → 复制链接
2. 无痕窗口打开链接 → 看到域信息 → 内联注册 → 直接进域（免审核域）
3. 打开「加入需审核」→ 无痕窗口注册 → 看到待审卡片
4. 管理员进域管理 → 加入申请 → 通过 → 用户刷新 → 进域
5. 邀请码设 1 次上限 → 第二人注册 → 报「已达使用上限」
6. 禁用码 → 打开链接 → 报「已停用」
7. 桌面端打开 `gospeak://invite/d/<新码>` → 唤起并落到邀请页
8. 已登录用户从发现页加入公开域（免审核）→ 直接进域
9. 已登录用户从发现页加入公开域（需审核）→ 看到「申请审核中」
10. 已登录用户从发现页加入私有域 → 提示需要邀请码

## 9. 实施顺序

| PR | 内容 | 依赖 |
|---|---|---|
| 1 | 删除访客全链路（后端 + 前端 + 数据层） | 无 |
| 2 | 邀请码实体：模型、仓库、签名、生成、校验、单测 | 无 |
| 3 | 邀请码 API 与 `/link` 管理页 | PR 2 |
| 4 | 内联注册与域级审核（后端 API） | PR 2、3 |
| 5 | 邀请落地页重写、审核 UI、桌面端正则 | PR 4 |

PR 1 可独立合并，并能立刻消除清理任务的数据破坏缺陷。

## 10. 风险

| 风险 | 说明 | 缓解 |
|---|---|---|
| 无迁移代码导致不可回滚 | 删列删表后只能靠 git revert + 数据库备份恢复 | 实施前必须备份数据库 |
| 旧邀请链接全部失效 | 已分享的 8 位码链接作废 | 已确认接受；上线后需重新分发邀请链接 |
| 公开域不再支持匿名进入 | 未注册用户无法旁听，也无法看到公开域列表（`list-public` 需 JWT） | 已确认接受；注册即为进入门槛 |
| 签名密钥轮换使全部旧码失效 | 密钥从 `StorageEncryptKey` 派生，轮换该密钥会波及邀请码 | 码有时效，影响可控；版本后缀预留算法升级路径 |

## 11. 范围外

- 访客体系的任何形式保留或降级
- 邀请码的旧数据迁移
- 审核结果站内通知或邮件通知
- 待审数量徽标
- 邀请码修改历史与 IP 审计
- 邀请码自定义易记字符串
- 访客升级正式账号
