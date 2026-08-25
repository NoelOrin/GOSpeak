# 访客访问与权限 — 设计规格

日期：2026-08-25
状态：待用户审查

## 1. 背景与目标

GOSpeak 当前所有业务接口都强制 JWT 登录，不存在匿名访客概念。本设计引入「访客访问」能力：用户无需注册即可通过邀请链接或公开 Domain 以访客身份进入语音房间，旁听、发言、发消息的能力由 Domain 管理端独立配置。

系统已有的 Domain `guest` 系统角色（`model/domain_role.go`）目前仅 seed 了只读权限行，没有任何入口会把用户落到该角色。本设计打通「匿名访客 → 真实用户行(is_guest) → DomainMember(role=guest) → 现有鉴权/裁决」链路。

### 已确认的决策（头脑风暴结论）

1. 匿名访客进入后自动落到 Domain 的 `guest` 角色，权限统一走现有体系
2. 能力边界：听 / 说 / 发消息三个能力**独立配置**；开启访客后听、说默认打开，发消息默认关闭
3. 加入方式：邀请码链接必支持；公开 Domain 额外提供「允许访客进入」开关
4. 身份：设备级持久身份——guest 行落 `users` 表，签发标准 JWT，存 localStorage，同一设备下次进入仍是同一人
5. 治理：方案 3——guest UUID 封禁（新 `domain_guest_bans` 表）+ 签发 IP 限流 + 单 Domain 访客在线上限可配
6. 「访客升级正式账号」不纳入本期，仅保留扩展点（guest UUID 与正式用户 UUID 同空间）
7. 前端完整闭环：访客入口页（含登录页「访客登录」按钮）、访客侧受限界面、管理端配置与治理界面
8. 架构方案 A：guest = 带 `is_guest` 标记的真实 users 行，签发标准 JWT，落 `DomainMember(role=guest)`；存量鉴权、Casbin、消息归属、SFU 全链路复用

## 2. 数据模型

### users 表（1 个新字段）

- `is_guest bool`，default false，加索引
- guest 行约定：
  - `name = "guest_" + uuid前12位`（满足全局唯一索引，仅机器使用）
  - `display_name = 用户输入昵称`
  - `password` 留空（guest 不经登录接口）
  - `role = "user"`，`token_version` 照常参与吊销
  - `email` 为空

### domains 表（5 个新字段）

| 字段 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `allow_guest` | bool | false | 访客总开关（邀请链接与公开进入共用） |
| `guest_can_listen` | bool | true | 访客可旁听语音 |
| `guest_can_speak` | bool | true | 访客可发言（发布音频流） |
| `guest_can_message` | bool | false | 访客可发文字消息 |
| `guest_limit` | int | 50 | 同时在线访客上限，0 = 不限 |

### domain_guest_bans 表（新增）

```go
type DomainGuestBan struct {
    ID         uint       // pk
    DomainUUID string     // index
    UserUUID   string     // guest 的 user uuid
    Reason     string
    BannedBy   string     // 操作者 uuid
    CreatedAt  time.Time
    ExpiresAt  *time.Time // nil = 永久
}
```

### 迁移

全部走现有 GORM AutoMigrate 路径，新字段带 default，存量数据零修改。项目当前无版本化迁移文件，跟随现状。

## 3. 身份流转

```
打开邀请链接 / 登录页访客按钮 / 公开 Domain
→ 输入昵称
→ POST /api/v1/auth/guest {nickname, invite_code? | domain_uuid?}
→ 校验：domain.allow_guest && (邀请码匹配 || domain公开) && 未达 guest_limit
       && 该身份未在该 Domain 被封
→ 事务：插 users(is_guest=true) + 插 domain_members(role=guest, nickname)
→ 签发标准 access + refresh JWT（claims 无特殊标记，is_guest 由 DB 行承载）
→ localStorage 存 token；下次打开自动以同一身份进入
```

关键取舍：

- `is_guest` 放 DB 而非 JWT claims：封禁需立即生效，查 DB 不信 token
- guest 也签发 refresh token：设备级持久身份要求长期在线；吊销用现有 `token_version` 机制
- 存量按 `user_uuid` 的消息归属 / 封禁检查 / join 查询零改动

## 4. API

| Method | Path | Auth | 说明 |
|--------|------|------|------|
| POST | `/api/v1/auth/guest` | 公开 | 访客签发：`{nickname, invite_code? 或 domain_uuid?}` → `{access_token, refresh_token, user, domain}` |
| POST | `/api/v1/auth/guest/renew` | JWT(guest) | 已有凭证的 guest 加入另一个 Domain（复用 user 行，不落新 users 行） |
| POST | `/api/v1/domain/guest/config` | JWT (`domain:manage`) | 读写 allow_guest / guest_can_* / guest_limit |
| POST | `/api/v1/domain/guest/ban` | JWT (`domain:kick`) | 封禁：`{domain_uuid, user_uuid, reason, duration}` |
| POST | `/api/v1/domain/guest/unban` | JWT (`domain:kick`) | 解封 |
| POST | `/api/v1/domain/guest/ban-list` | JWT | 封禁列表 |

统一响应格式遵循现有 `pkg.Success` / `pkg.Fail` / `pkg.HandleError` 约定。

### 错误码

全部复用现有码：1013（禁止/被封）、1017（限流/达上限）、401，本期不新增业务码。

## 5. 中间件：Guest 守卫

`middleware.JWTAuth()` 鉴权通过后新增 guest 守卫（约 20 行）：

```
if user.is_guest:
    检查 domain_guest_bans（命中 → 1013）
    检查路由白名单：仅放行 domain:/room:/message:/signal:/user/profile 等访客可用接口
    域外接口（建 Domain、user/list、bot、storage 管理等）→ 1013
```

白名单以路由 group 前缀判定，集中在 middleware，不在各 handler 分散判断。

### Rate limit（新中间件，内存滑动窗口）

- `auth/guest`：单 IP 每小时 ≤ 10 次签发，超限返 1017
- 进房时检查 `guest_limit`：当前在线 guest 数（WS Hub 内存统计）达上限返 1017

## 6. WS / Signal / SFU

### WS（`internal/ws/upgrader.go`）

零改动。guest 复用同一条 `/ws` + subprotocol token 路径，`VerifyToken` 解析标准 JWT 即可。封禁生效在 API 层，WS 连接本身只承载广播。

### Signal Hub（`internal/signal/hub.go`）

`JoinRoom` 新增 guest 检查：查 `domain_guest_bans`（命中则踢出并关闭 WS）；查 `guest_limit`。`MemberInfo` 不加 guest 标记字段，「访客」徽标由前端 UI 层处理。消息归属天然正确（`author_uuid` = guest 的 user uuid）。

### SFU（`internal/sfu/*`）

零改动。`GenerateToken(room, identity)` 的 `identity = user_uuid` 约定对 guest 同构；`MuteParticipant` / `RemoveParticipant` 按 uuid 寻址直接生效。

### 发言/旁听/发消息的执行点

三项能力读 domain 表开关字段判定（不走 Casbin 角色行）：

- **听**：`signal/token` 签发及订阅流时校验 `guest_can_listen`，关闭则拒进房
- **说**：`signal/token` 签发的 SFU token 设置 `CanPublish=guest_can_speak`（LiveKit grant / SRS 流名 / Agora role 各自适配）；WS 侧发布事件二次校验；运行中被关闭时通过 `MuteParticipant` 立即禁言
- **发消息**：消息发送接口校验 `guest_can_message`，关闭则 1013

## 7. 前端

### A. 访客入口（全新）

- 登录页新增「访客登录」按钮：仅在存在开放访客的公开 Domain 时显示；点击进入访客流程（选择公开 Domain 或输入邀请码 → 输昵称）
- 邀请链接落地页：单卡片表单（昵称 ≤24 字符 + 进入按钮）；localStorage 已有本 Domain 有效 guest token 时显示「以 @昵称 继续」一键进入
- 失败态文案分别覆盖：Domain 未开放访客 / 达访客上限 / 已被禁止进入

### B. 访客侧受限视图

- 房间列表正常展示；`guest_can_listen` 关闭时进房被拒并 toast 提示
- 麦克风按钮按 `guest_can_speak` 禁用（锁形 tooltip），不粗暴隐藏
- 消息区始终可见；输入框按 `guest_can_message` 禁用
- 侧边栏隐藏 Domain 设置、成员管理、Bot、邀请管理（`isGuest` store flag 控制）
- 昵称旁「访客」灰底 tag；头像用昵称首字色块

### C. 管理端

- Domain 设置「访客访问」分区：`allow_guest` 总开关 + 三个子开关（总开关关闭时子开关灰显）+ `guest_limit` 数字输入
- 「访客管理」标签：在线访客列表（踢出按钮）+ 封禁列表（解封按钮）；复用 `DomainMemberTable` 行结构

### 技术要点

- 新增 `guestStore`（Zustand）：`isGuest` / `guestInfo` / `domainGuestConfig` + `init/leave`
- Axios 拦截器复用同一份 token 注入（`Authorization: Bearer`），不分叉
- `PermissionGate` 类组件新增 guest 短路：`isGuest && !public → null`

### 刻意不做

访客私聊窗口、访客个人资料页、访客跨 Domain 切换 UI（后端留 `guest/renew`，前端本期不做入口）。

## 8. 治理闭环

| 操作 | 后端动作 | 生效时效 |
|------|---------|---------|
| 关闭 `allow_guest` | 新访客签发被拒；存量在线 guest 不踢，自然流失 | 立即阻止新进 |
| 关闭 `guest_can_speak` | 新 SFU token 降级只听；在线发言者 `MuteParticipant` | ≤一次重连 |
| 踢出访客 | SFU `RemoveParticipant` + WS 断开 + 广播 member_left | 立即 |
| 封禁访客 | 写 `domain_guest_bans` + 踢出全套；之后该 uuid 请求被 guest 守卫拦 1013 | 立即 |
| 解封 | 删封禁行；guest 凭原 token 可重新进入 | 立即 |

### 并发与一致性

- `guest_limit` 为软上限，接受 TOCTOU 近似限流，不引入分布式锁
- 封禁只在请求入口检查一次，不做全链路轮询；WS 长连接内恶意行为由 kick 兜底
- 清理：本期仅管理端手动「清理 30 天未活跃访客」（删 users + domain_members 行；消息保留，author_uuid 悬空显示「已注销访客」），不做 cron

## 9. 测试策略

- Handler 集成测试（Go，沿用现有风格）：
  - `auth/guest` 签发：正常 / 未开放 / 昵称超长 / IP 限流 / 达上限
  - guest 守卫：调域外接口 → 1013
  - 封禁链路：ban → 1013；unban → 恢复
  - 三个能力开关各自的开/关行为
- Service 单测：guest 角色权限矩阵（默认只读）回归；`domainRoleLevel` 不变式
- SFU：不动，guest 身份对 SFU 透明
- 前端：`guestStore` 单测；入口页表单组件测试；`PermissionGate` guest 短路测试

## 10. 范围外（明确不做）

- 访客升级正式账号（保留扩展点：UUID 同空间、字段预留）
- 访客自动清理 cron
- IP 级封禁与风控
- 访客 OAuth 绑定
