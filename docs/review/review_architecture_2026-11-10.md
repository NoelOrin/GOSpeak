# GOSpeak 架构审查报告

- **审查日期**：2026-11-10
- **审查范围**：`app/server`（分层/依赖/SFU/signal/ws/bus/authstate）、`app/web` + `packages/*`（状态管理/包边界/类型/测试）、`turbo.json` + `.github/workflows`（工程闭环）
- **方法**：只读静态审查（3 条并行主线）+ 对高危/存疑结论逐文件行号复核 + 与历史 `docs/review/*` 交叉比对
- **严重度**：🔴 严重（数据/安全/一致性或架构根基）｜🟡 中等（可靠性/可维护性/性能）｜🔵 轻微（规范/残留）
- **证据约定**：路径相对仓库根；每条均经 `file:line` 核验，未经核实的猜测不列入

---

## 一句话结论

**分层主线是健康的，真正的问题不在"某层写错了"，而在三处系统性错配**：① 部署模型（单进程内嵌 NATS）与代码/文档假设（多实例共享状态）错配；② 安全与错误策略在 HTTP / WS / OAuth 三条通道间分叉；③ 前端 server state 被 React Query、zustand、组件 useState 三套容器同时接管。叠加工程闭环缺口（~~62 个前端测试文件 0 进 CI~~ → 已由 f698ce0 修复：CI 新增 `test-web`/`test-packages` 并纳入 Gate），当前健康度为 **中等偏弱**。

---

## 执行摘要

### 架构健康度（分维度）

| 维度 | 评价 | 说明 |
|------|------|------|
| 后端分层（Handler→Service→Repo） | 🟢 良好 | 生产 handler 零直连 repo/DB；service 不碰 `gin.Context`；repo 不反向依赖 service |
| 统一响应 / 错误脱敏（JSON API） | 🟢 良好 | `AppError` + `HandleError` + `ClientError` 设计完整，JSON 主通道覆盖率高 |
| SFU / 实时状态层**局部工程** | 🟢 良好 | CAS 合并、lease、typed-hull、ACK recover、时间预算打磨扎实 |
| 部署模型 ↔ 状态架构 | 🔴 错配 | 内嵌临时 NATS 却假设跨实例共享；密钥随重启丢失 |
| 抽象 ↔ 实现契约 | 🔴 脱节 | `ErrSFUNotSupported` 死代码、Capabilities 全零降级、8+ 可选接口 |
| 安全边界一致性 | 🔴 分叉 | WS 绕过 `BanCheck`/`GuestGuard`；OAuth 错误通道绕过脱敏 |
| 前端状态管理 | 🟡 混乱 | server state 三套容器；同端点重复请求；god component |
| 依赖治理 / 可测试性 | 🟡 偏弱 | 包级全局注入；核心 service 绑定具体类型 |
| 工程闭环 | 🟡 改善中 | ~~CI 不跑前端测试；hook 依赖检查关闭~~ → 已由 f698ce0 修复（CI 新增 `test-web`/`test-packages`；hook 依赖改为 warn + 棘轮门禁）；DTO 手工镜像仍在 |

### 三个根本判断

1. **部署模型与架构复杂度错配（元问题，优先级最高）**。项目定位是单二进制/单进程自托管，但 `bus`/`authstate`/`signal` 里到处是为"多实例共享"设计的机制（InstanceID 去重、revision CAS、lease、`mergeRemoteMembers`、`RemoteHook`），而 `bus.Init` 固定启动 **127.0.0.1 随机端口 + `MkdirTemp` 临时目录**的内嵌 NATS，配置层**没有外部 NATS URL 字段**。多副本下每个副本的 JetStream 互相不可见，所有"跨实例"能力退化为单进程内生效。根因可从历史追溯：项目刚从多 SFU（mediasoup/agora/cloudflare）+ cluster + Redis 精简为单 LiveKit + 内嵌 NATS，旧抽象未同步清理。**当前单实例下大部分问题被掩盖、功能可跑；一旦横向扩容或 NATS 抖动，正确性风险集中爆发。**

2. **策略与契约是"传输层相关"而非"身份层统一"的**。同一条封禁/访客/脱敏规则，在 HTTP 组、WS 直连、OAuth 重定向三条通道各实现一次且不一致，形成稳定绕过面。SFU 抽象层的 `ErrSFUNotSupported`、Capabilities、EnforcementLevel 也存在"文档声明 vs 实现行为"的系统性漂移。

3. **工程闭环没有兜住已有的质量投入**。前端 62 个测试文件、三层错误边界、条件编译双目标测试都写得很认真，但审查时 CI 从不运行前端测试，hook 依赖检查被完全关闭，前后端 DTO 靠人工镜像——意味着回归几乎只能靠 typecheck 和人工 review 拦截。**（G3/G4 已由 f698ce0 修复：CI 新增 `test-web`/`test-packages` 并纳入 Gate；`useExhaustiveDependencies` 改为 warn + `scripts/check-hook-deps.mjs` 棘轮门禁。DTO 镜像（G5）仍未解决。）**

### P0 清单（建议立即处理）

| # | 主题 | 一句话 |
|---|------|--------|
| A1/A2 | 状态根基 | JetStream 确实落盘，但 StoreDir 是每次随机、退出即删的临时目录（volume 未覆盖）→ 重启换 key 全员重登、logout 黑名单失效；先定部署模型再收敛 |
| B1 | 安全 | WS 通道绕过 `BanCheck`/`GuestGuard`，域访客封禁可经已建连继续发消息 |
| B2 | 安全 | OAuth 回调错误通道绕过脱敏，可能把上游 `access_token` 写入重定向 URL |
| F5 | 回归 | `ListParticipants` 错误时返回空数组（历史 B05 已修项回归） |
| G3 | 工程 | ~~前端 62 个测试文件 0 进 CI~~ → 已由 f698ce0 修复（CI 新增 `test-web`/`test-packages` 并纳入 Gate） |

---

## 主题 A. 部署模型与共享状态根基 🔴

### A1. "跨实例共享状态"在生产装配下不成立 〔🔴〕
- **问题**：`bus.Init` 固定启动进程内 NATS，绑定 `127.0.0.1`、随机端口，JetStream 用 `os.MkdirTemp` 临时目录；配置层无任何外部 NATS URL 字段。而 membership/auth/EventBus 的设计（InstanceID 去重、revision CAS、lease、`mergeRemoteMembers`、`RemoteHook`）全部以"多实例共享"为前提。
- **证据**：`app/server/internal/bus/embedded.go:37-45`（`Host: "127.0.0.1"`、`Port=-1`、`StoreDir: dir`）、`:27`（`os.MkdirTemp`）、`:78-80`（Shutdown `RemoveAll`）；`app/server/internal/config/config.go:84-88`（仅 prefix/name/timeout/port/WAL，无 URL）；`app/server/internal/bus/membership_store.go:39-41`（注释称 "cross-instance room views always have shared state"）。
- **影响**：多副本下房间视图/踢人/禁言/密钥共享全部退化为单进程内生效；单实例重启即丢失全部 KV。
- **建议**：这是**产品决策**而非纯 bug。要么承认单实例、删除跨实例抽象并在文档明确"不支持水平扩容"；要么增加 `NATS_URL`、持久化 volume、去掉随机端口。不要维持"代码为多实例、部署为单实例"的中间态。

### A2. JWT 密钥与黑名单：JetStream 确实落盘，但 StoreDir 是临时目录，重启即空 〔🔴〕
- **问题**：签名密钥（`jwt.active`）与黑名单 / refresh-family 确实写进 JetStream KV（`auth_store.go` 用 `kv.Create/Put`），JetStream **有磁盘持久化，不是纯内存**。但 `bus.Init` 把 JetStream `StoreDir` 设为 `os.MkdirTemp("", "gospeak-nats-js-*")`（`embedded.go:27,44`）：**每次启动都是新的随机临时目录**，优雅退出 `Shutdown` 还 `os.RemoveAll(e.storeDir)`（`:78-80`）；`docker-compose.yml:49-52` 的 volume 只挂 db/logs/uploads，未覆盖它；`config.go:84-88` 也没有 StoreDir 或外部 NATS URL 字段。⇒ 数据确实落了盘，但盘的生命周期 = 单次进程运行，**下次启动换新目录、不会去读旧目录**（即便容器没重建、旧 `/tmp` 仍在）。
- **后果（精确版）**：
  - 签名密钥：生产模式 `GetSigningKey()` 读不到 KV 就 `randomKey()` 生成**新** key（`jwt_key.go:50-64`）→ 旧 token 全量验签失败 → 全员重新登录。**开发模式用 static `JWT_KEY`（`:37-45`），重启不换 key、不受影响**——这是设计者有意加的保护。
  - 黑名单：logout 吊销的 jti（TTL=剩余有效期，`auth_service.go:452-460`）与 refresh-family 重放保护随 KV 清空 → token 过期前可"复活"。
  - **不受影响**：改密 / 改角色 / 封禁的失效靠 DB 里的 `user.TokenVersion`（`auth_service.go:464-469`、`middleware/auth.go:131-136`），不依赖 KV，**重启仍有效**。所以黑名单丢失的实际面窄于"全部吊销失效"。
- **证据**：`app/server/internal/bus/embedded.go:27,37-45,78-80`；`app/server/server/gin.go:175-182→factory.go:56-71`（固定内嵌）、`:309-316`（传内嵌 `NC`，URL 分支走不到）；`app/server/internal/config/config.go:34,84-88`；`app/server/internal/authstate/jwt_key.go:37-64,149-156`；`app/server/internal/service/auth_service.go:316,321,452-469`；`deploy/docker-compose.yml:49-52,238-248`。
- **校正（2026-11-10 复核）**：初版标题写"生产回退 `staticKey` 会 panic"过重——panic 实为**条件性**：仅当 backend 为空或 KV 完全不可用、且 `JWT_KEY` 未配或仍是 `default-secret` 时才触发（`jwt_key.go:149-156`）；正常部署 `init.sh:241` 会生成随机 `JWT_KEY` 且 backend 正常，不会 panic，而是静默换新 key → 全量重登。另：`NATS_WAL_PATH`（`config.go:88`）是事件总线断线重放的 JSONL WAL（`bus/wal.go`），**不是** JetStream StoreDir，勿与持久化混淆。
- **附带洞察**：生产 + backend 正常时签名用的是 KV 里的随机 `jwt.active`，`JWT_KEY` **只在回退分支被读**——即 `init.sh` 生成、文档反复强调"生产必改"的 `JWT_KEY`，正常生产路径里**并不参与签名**；真正的签名 key 由首个启动的进程随机生成、随重启更换，运维既看不见也控不了。
- **建议**：三选一——① 给内嵌 NATS 配**固定且持久化**的 StoreDir 并在 compose 挂 volume；② 接**外部 NATS**（底层 `OpenAuthStore(cfg.URL)` 等已支持，需给 `bus.Init`/config 加 URL 并让 auth/membership/jobs 共用同一外部连接）；③ 承认单实例，把签名密钥与黑名单落到 **DB / 密钥文件**，让密钥生命周期显式脱离总线。
- **状态（f698ce0）**：方案① 已落地。`config.NATStoreDirPath()` 新增（显式 `NATS_STORE_DIR` 优先，否则回退 DB 同级 `nats/`），`gin.go` 的 `bus.Init` 调用点已传 `StoreDir`；`deploy/docker-compose.yml` 挂 `gospeak-nats:/app/nats` 并以 `${NATS_STORE_DIR:-/app/nats}` 注入。回归测试 `app/server/server/event_bus_test.go` 断言 InitConfig 带 StoreDir，并复刻调用点验证日志 `storeDir=` 为持久化路径而非 `gospeak-nats-js-*` 临时目录。**A1 的多实例假设问题仍未解决**（仍是单进程内嵌 NATS、固定 StoreDir 只支持单实例）。

### A3. membership fail-fast 与 authstate 内存降级，两套容错哲学冲突 〔🟡〕
- **问题**：membership 删除了 `none` 内存后端、缺 NATS 直接启动失败；authstate 在 `SetBackend(nil)` 时静默退化为进程内存黑名单/refresh family。
- **证据**：`app/server/internal/bus/membership_store.go:28-54`（fail-fast，注释自认）、`app/server/internal/authstate/backend.go:42-54`、`app/server/internal/authstate/blacklist.go:14-42`、`app/server/internal/bus/AGENTS.md:18-19`。
- **影响**：NATS 后端为 nil 时，logout/replay 保护只对单进程生效，多副本下已登出 token 仍可用；运维对"NATS 不可用时系统行为"无法形成一致预期。
- **建议**：统一策略——要么共享状态全部 fail-fast，要么全部提供明确标注"仅单实例"的内存后端，禁止静默退化。

---

## 主题 B. 安全边界一致性 🔴

### B1. WS 通道绕过 `BanCheck` / `GuestGuard` 〔🔴〕
- **问题**：HTTP 受保护组强制 `JWTAuth → BanCheck → GuestGuard`，但 `/ws` 由根 `http.Handler` 直接交给 Upgrader，绕过整个 Gin 中间件链；Upgrader 只做 token 校验，分发层无访客/封禁策略。
- **证据**：`app/server/internal/router/router.go:106-109`（protected 组三件套）；`app/server/server/gin.go:503-507`（`/ws` 直连 Upgrader）；`app/server/internal/ws/upgrader.go:112-117`（仅 `VerifyToken`）；`app/server/internal/ws/handler.go:41-77`（`Dispatch` 仅 panic recover）；`app/server/internal/signal/private_bridge.go:23-45`（私信只校验 identity 非空）；`app/server/internal/service/conversation_service.go:199-212`（`SendDirect` 无访客/封禁判定）。
- **关键细节**：域访客封禁**不失效 JWT**——`app/server/internal/service/guest_service.go:168-199` 只写 `domain_guest_bans` 行，无 `token_version` 递增（对比角色封禁会递增，见 `app/server/internal/repository/user_repo.go:132-140`）；HTTP 每请求查封禁（`app/server/internal/middleware/guest.go:52-61`），WS 只在入房时查一次（`app/server/internal/signal/hub_room_join.go:104-114`），房间消息策略只查域配置不查封禁（`app/server/internal/signal/message_bridge.go:106-112`）。
- **影响**：被域封禁的访客只要保持已建立的 WS 连接，仍可 `message:send`/`private:send`/`typing`；普通访客可经 WS `private:send` 使用 HTTP 侧 `GuestGuard` 明确禁止的私信能力。**缺口集中在域访客封禁与 GuestGuard 白名单**（角色 `ban` 因 `token_version` 递增被 `VerifyToken` 间接拦截）。
- **建议**：把 `BanCheck`/`GuestGuard` 抽成传输无关的 `Policy(claims, resource, action)`，在 WS 建连与 `Dispatch` 按事件各执行一次；`message:send`/`private:send`/`typing` 必须调用 `IsGuestBanned`，不能只依赖 `GuestCaps`。

### B2. OAuth 回调错误通道绕过脱敏，可能回显上游 `access_token` 〔🔴〕
- **问题**：`pkg.HandleError` 的脱敏（`shouldHideErrorDetail`）只作用于 JSON 响应；OAuth 错误走 302 重定向，用自写函数把 `AppError.Message` 原样外泄，而 `OAUTH_GET_USER_FAILED` 的上游错误可能包含 userinfo URL 的 query 参数 `access_token`。
- **证据**：`app/server/internal/pkg/oauth/oauth.go:264-269`（`params.Set("access_token", ...)` 后拼进 URL）、`:109-113`（原始 `err` 未包装，Go `url.Error` 带完整 URL）；`app/server/internal/service/oauth_service.go:143-145`（`NewAppError(OAUTH_GET_USER_FAILED, err.Error())`）；`app/server/internal/handler/oauth_handler.go:153-163`（`if appErr.Message != "" { return appErr.Message }`，不经 `shouldHideErrorDetail`）；`app/server/internal/pkg/response.go:107-115`（本应隐藏却未被该通道调用）；路由为公开组（无 JWT）。
- **影响**：未认证的 `GET /api/v1/oauth/callback/:provider` 在把 token 放 URL 的 provider 上失败时，可能把含 `access_token` 的 `url.Error` 写入 `Location: /login?oauth_error=...`，进入浏览器历史、反代日志与前端错误展示。
- **建议**：`oauthErrorMessage` 复用 `pkg.ClientError(err)` 取安全文案，禁止返回 `appErr.Message`；QQ userinfo 改 Bearer Header 或 URL 脱敏后再记录。

### B3. WS Origin 默认放行，与文档/注释矛盾 〔🟡〕
- **问题**：`originAllowed` 对空 `Origin` 直接 `true`，`AllowedOrigins` 含 `*` 放行任意来源；`CORS_ORIGIN` 默认 `*`，且 `wsAllowedOrigins` 在 `WS_ALLOWED_ORIGINS` 为空时回退到 `CORSOrigin` ⇒ WS 默认就是 `"*"`。而配置注释声称"未配置时默认只允许同源握手"。
- **证据**：`app/server/internal/ws/upgrader.go:61-85`；`app/server/internal/config/config.go:60`（`envDefault:"*"`）；`app/server/server/seeds.go:56-61`（回退 `CORSOrigin`）；`app/server/internal/ws/upgrader.go:119-122`（`InsecureSkipVerify: true`）。
- **影响**：实际策略与文档不符；在 `COOKIE_SAMESITE=none` 部署下构成跨站 WebSocket 劫持（CSWSH）面。**注意**：默认 `COOKIE_SAMESITE=lax` 下跨站 WS 握手不带 cookie，风险主要落在显式改 `none` 的部署，不宜夸大为默认漏洞。
- **建议**：默认同源校验，空 Origin 仅对显式白名单客户端放行；`WS_ALLOWED_ORIGINS` 与 `CORS_ORIGIN` 解耦，注释与实现对齐。

### B4. `/metrics` 默认匿名可读，脱离 JWT/RBAC 组 〔🟡〕
- **问题**：Prometheus 端点直接挂根 Engine，仅在配置 `METRICS_TOKEN` 时鉴权，而该配置默认为空。
- **证据**：`app/server/server/gin.go:424-428`；`app/server/internal/metrics/metrics.go:201-203`（`if token == "" { return next }`）；`app/server/internal/config/config.go:90`（默认 `""`）；暴露内容见 `app/server/internal/handler/monitor_handler.go:149-206`（DB 池、磁盘、房间/在线人数、CPU）。
- **影响**：默认部署下匿名访问者可获取运行时容量与业务拓扑，与 `/system/stream`（`RequireRole("admin")`）敏感级别不一致。
- **建议**：未配置 token 时默认不注册或仅绑内网；把 `/metrics` 访问控制声明为与 admin 同级策略。

### B5. RBAC 双轨：`admin` 角色可被改名/删除 + 硬编码角色名 〔🟡〕
- **问题**：多数路由用 `RequirePermission`，但监控/系统更新/平台管理员豁免/防禁言等保护点直接比较字符串 `"admin"`；而 `/role/update`、`/role/delete` 允许修改内置角色且仅要求 `role:manage`。
- **证据**：`app/server/internal/router/routes/system/routes.go:12,17-18`；`app/server/internal/middleware/domain.go:44-52,120-123`；`app/server/internal/service/mute_service.go:291-296`；`app/server/internal/handler/role_handler.go:73-90,102-117`；`app/server/internal/router/routes/role/routes.go:13-15`。
- **影响**：`admin` 被重命名 → `/system/*` 全部 403、平台管理员失去域成员豁免、"禁止禁言管理员"保护失效；反向把某角色改名 `admin` 并赋给用户 → 获得域成员绕过。
- **建议**：内置角色改用稳定 ID/常量并在 `RoleService.Update/Delete` 中禁止改动；`IsPlatformAdmin` 与 handler 内判断统一改为 `RequirePermission`/`permcode`。

### B6. `STORAGE_NOT_CONFIGURED` 未纳入脱敏，DB 错误原文可返回客户端 〔🟡〕
- **问题**：唯一一处"非脱敏业务码 + 原始 `err.Error()`"拼接，且该码不在 `shouldHideErrorDetail` 白名单。
- **证据**：`app/server/internal/service/storage_service.go:178-181`（`NewAppError(STORAGE_NOT_CONFIGURED, err.Error())`，err 来自 repository 原始错误 `:225-233`）；`app/server/internal/pkg/response.go:107-115`（含 `STORAGE_ERROR` 但不含 `STORAGE_NOT_CONFIGURED`）。
- **影响**：持 `storage:read/manage` 权限者可收到底层 DB 错误文本（表名、驱动错误）；错误码语义也错配（DB 异常被标成"未配置"）。
- **建议**：加入脱敏白名单；DB 异常映射为 `INTERNAL_ERROR`。

### B7. WS ACK 未统一走 `pkg.ClientError` 〔🔵〕
- **问题**：JSON API 主通道脱敏完整，但 WS 入房 ACK 直接写 `err.Error()`，缺强制约束。
- **证据**：`app/server/internal/signal/hub_room_join.go:113,120,372,378`（`marshalAck({"error": guardErr.Error()})`）；对比 `app/server/internal/signal/message_bridge.go:138-141` 已统一脱敏。
- **现状**：当前 `resolveIdentity`/`guestJoinGuard` 只返回固定字符串，未实际泄漏细节（`app/server/internal/signal/hub.go:232-240`、`app/server/server/gin.go:243-268`）；风险是架构上没有强制力。
- **建议**：WS ACK 一律经 `pkg.ClientError` 得到 `(code, msg)`；在文档明确"统一响应仅约束 JSON API"的边界。

---

## 主题 C. SFU 抽象契约脱节 🔴

### C1. `ErrSFUNotSupported` 契约从未被唯一实现履行，Hub 分支是死代码 〔🔴〕
- **问题**：`Provider` 契约要求"不支持的操作必须返回 `NewErrSFUNotSupported()`"，但 LiveKit 实现**从不**返回该 sentinel（未配置返回 `SFU_NOT_CONFIGURED`，API 错误返回 `SFU_ERROR`）。Hub 中所有 `errors.Is(err, ErrSFUNotSupported)` 分支永不命中。
- **证据**：`app/server/internal/sfu/provider.go:7-9`（契约）；`app/server/internal/sfu/providers/livekit/client.go:74-195`（无 sentinel）；`app/server/internal/sfu/factory/dynamic_provider.go:275`（明确 unsupported 却返回 `SFU_NOT_CONFIGURED`，违反自身契约）；消费点 `app/server/internal/signal/hub_sfu.go:23,41`、`app/server/internal/signal/hub_mute.go:107`、`app/server/internal/signal/hub_room_join.go:337`；契约测试仅用 stub（`app/server/internal/sfu/factory/dynamic_provider_test.go:31,34`）。
- **影响**："不支持"与"真故障"在调用侧无法区分；新 provider 实现者会以违反契约的 LiveKit 为模板；目前靠"所有错误都 soft fallback"的宽泛分支兜底（`hub_sfu.go:27-28`），掩盖契约失效。
- **建议**：修正 LiveKit/DynamicProvider 的 unsupported 路径返回 `NewErrSFUNotSupported()`；增加对每个 Provider 各方法错误分类的契约测试。

### C2. Capabilities 双源 + 配置失败静默降级为 soft 〔🟡〕
- **问题**：能力矩阵同时存在于自由函数 `CapabilitiesFor(provider string)`（硬编码字符串）与 `Provider.Capabilities()` 方法；`DynamicProvider.Capabilities()` 在 `current()` 出错时返回**全零** `Capabilities`，使 `LevelEnabled("")` 为 false，mute/kick 静默降级 soft 并下发前端全 false——与"缺配置应报 `SFU_NOT_CONFIGURED`"的契约相反。
- **证据**：`app/server/internal/sfu/capabilities.go:5-27`（`if provider == "livekit" || provider == ""`）；`app/server/internal/sfu/providers/livekit/client.go:209-211`；`app/server/internal/sfu/factory/dynamic_provider.go:238-244`（已复核：err 时 `return sfu.Capabilities{}`）；`app/server/internal/sfu/types.go:55-72`；消费 `app/server/internal/signal/hub_mute.go:84-87`。
- **影响**：配置解析失败被伪装成"provider 不支持"；`provider_test.go:28-29` 用 `CapabilitiesFor` 验证 `Capabilities()` 属自证，无法发现矩阵与方法行为漂移。
- **建议**：删除硬编码自由函数，能力由 provider 实例自描述并在构造时校验；出错时返回错误/上次已知 good 值，不得返回全零。

### C3. Provider 抽象被 8+ 可选接口 + 跨包 type assertion 拼装 〔🟡〕
- **问题**：核心 `Provider` 之外散布 `StreamProvider`、`ClientInfoProvider`、`PublishControlProvider`、`TimedMuteProvider`、`ownerAwareProvider`（factory 与 service 各定义一次）、`RoomRegistrySetter`/`StreamRoomResolverSetter`（注释明言为绕 import 循环）、`ParticipantCleanupHandler`（注释称"仅 mediasoup 实现"但 factory 只支持 LiveKit）。缺省均静默 fallback。
- **证据**：`app/server/internal/sfu/provider.go:25-44`、`app/server/internal/sfu/types.go:47-52`、`app/server/internal/sfu/factory/dynamic_provider.go:15-25,127-148`、`app/server/internal/pkg/room_registry.go:3-5,20-36`、`app/server/internal/signal/hub.go:61-68`。
- **影响**：实现新 provider 需满足一组无集中声明的隐式接口，遗漏即静默退化；大量"为假想 provider/单测 stub 服务"的死抽象抬高维护成本（呼应主题 A 的精简残留）。
- **建议**：集中可选能力定义、配编译期接口断言；把能力显式建模为 `Capabilities` 字段而非运行时 type assertion；删除 mediasoup 遗留注释/接口。

### C4. Domain 命名空间在 KV 层被有损编码破坏 〔🟡〕
- **问题**：复合键 `domainUUID:roomName` 写入 KV 经 `sanitizeKey` 把 `:`、空格映射为 `_`，`desanitizeKey` 无法逆推（注释自认）；`ListRoomNames` 返回名字丢失分隔符，`splitRoomKey` 按 `:` 切分得到空 domainUUID；不同房间（`d1:room_b` 与 `d1:room b`）会碰撞到同一 key。
- **证据**：`app/server/internal/pkg/room_key.go:7-22`；`app/server/internal/bus/kv_store.go:307-346,380-399`；消费 `app/server/internal/signal/hub_room_list.go:108,138,190`、`app/server/internal/signal/hub_mute.go:166`。
- **影响**：房间列表 Domain 过滤错误、成员元数据被同名房间覆盖，直接违反 Domain 隔离目标。
- **建议**：复合键一律走可逆编码（base64url 或显式域前缀 + 版本号），不再保留有损 legacy 映射；对已存在 key 做迁移。

### C5. 契约小项合辑 〔🟡/🔵〕
- **EnforcementLevel 用裸 string，未知值默认 soft**〔🔵〕：`app/server/internal/sfu/types.go:20-25,35-45`；能力字段拼错会静默降级 soft，安全相关 enforcement 无声失效。建议定义 `type EnforcementLevel string` + 常量，未知值显式报错。
- **TTL 定时禁言对 LiveKit 实际无 TTL 语义**〔🔵〕：`app/server/internal/signal/hub_mute.go:14-21,101-105`、`app/server/internal/sfu/factory/dynamic_provider.go:175-184`；LiveKit 未实现 `MuteParticipantTimed`，`ttlSeconds` 被丢弃，"永久禁言"退化为普通 mute。建议 provider 实现 TTL 或上层周期重放 mute 规则。
- **永久/TTL 与规则层 TTL 不一致**：同上，属于 C1 契约失效的具体表现。

---

## 主题 D. 事务、一致性与可用性 🟡

### D1. 角色改名跨服务无事务/补偿 〔🔴〕
- **问题**：`RoleService.Update` 先提交角色名，再经**包级回调**触发 `PermissionService.RenameRole` 迁移权限行并重载 Casbin/缓存；后段失败无回滚/补偿，返回 500 了事。
- **证据**：`app/server/internal/service/role_service.go:92,100-103`（已写库后才回调）、`app/server/server/gin.go:123-125`（回调绑定 `permSvc.RenameRole`）、`app/server/internal/service/permission_service.go:189-199`（三步任一步失败即返回）；可达路由 `app/server/internal/router/routes/role/routes.go:14`（`role:manage`）。
- **影响**：DB 抖动时角色名已改但 `role_permissions.role_name` 仍是旧名 → 该角色下所有用户权限立即丢失（RBAC 锁死）且无法恢复；集群下其它实例 `model.roleCache` 保留旧角色名（`OnRemoteInvalidate` 未覆盖，见 `gin.go:287-303`）。
- **建议**：纳入同一事务或可重放幂等 Saga，后段失败必须回滚角色名；或产品层禁止重命名内置角色（配合 B5）。

### D2. `AuthStore` 全部 KV 调用无超时，阻塞 JWT 鉴权热路径 〔🟡〕
- **问题**：membership 路径统一用 `kvTimeoutCtx()`（2s）+ 包装，但 `AuthStore` 的 `Get/Put/Create/Delete` 全部裸调，无 ctx、无超时。
- **证据**：`app/server/internal/bus/auth_store.go:100-131,151-171,216-324`；对比 `app/server/internal/bus/kv_store.go:125-155`、`app/server/internal/signal/state_sync.go:24-30`；热路径 `app/server/internal/middleware/auth.go:123-139` → `app/server/server/gin.go:402` → `app/server/internal/pkg/jwt.go:64-65`（多次 KV Get）。
- **影响**：NATS 卡顿时每个 HTTP/WS 请求的鉴权同步阻塞且无界，可拖垮整个服务。
- **建议**：auth KV 统一接入带 ctx 超时包装，鉴权入口设总超时预算。

### D3. WAL 重放失败仍无条件截断；事件总线是 at-most-once 〔🟡〕
- **问题**：`flushPending` 重放时 publish 失败只记 `droppedPublish`，随后仍 `wal.Truncate()` 清空磁盘记录；发布用 core NATS `nc.Publish`（fire-and-forget）。
- **证据**：`app/server/internal/bus/nats_publish.go:75-105`（失败仍 Truncate）、`:34`（core Publish）、`:14,57-71`（pending 上限 1024、满即丢）；`app/server/internal/bus/nats_bus.go:127-130,186-210`。
- **影响**：禁言/踢人/provider 切换等控制事件无送达保证，WAL 持久化承诺在故障时自我否定。
- **建议**：按记录粒度确认后才截断；控制事件改走 JetStream + Ack，与"尽力广播"的 namespace 事件分级。

### D4. `AuthStore` 密钥轮换是 last-writer-wins / 进程内锁 〔🔵〕
- **证据**：`app/server/internal/bus/auth_store.go:248-259`（LWW）、`:282-309`（`AddHistoryKey` 用进程内 `s.mu` 读改写，注释自认"极端并发可能丢失中间 key"）；`app/server/internal/authstate/jwt_key.go:191-215`（每实例各自 1 分钟 ticker）。
- **影响**：多实例并发轮换会丢失 history key，导致用中间 key 签发的 token 无法验签（A1 修复后才会真正暴露）。
- **建议**：轮换改单点角色或 CAS + 集中 history（JetStream 追加流）。

---

## 主题 E. 实时层并发与可靠性 🟡

### E1. WS read loop 同步执行所有 handler，慢操作阻塞消息并触发心跳误杀 〔🟡〕
- **问题**：`StartReadLoop` 对每条消息**同步**调用 handler；handler 内含同步 DB、最多 3 轮 KV CAS 重试、SFU HTTP 5s 调用。nhooyr 的 `Ping` 依赖 Reader 读 pong，read loop 被占用时 pong 无法消费，`heartbeatLoop` 5s 超时主动 `Close`。
- **证据**：`app/server/internal/ws/client.go:153-166,199-205`；`app/server/internal/ws/handler.go:65-76`；`app/server/internal/signal/hub_room_join.go:555`；`app/server/internal/signal/state_sync.go:229-280`；`app/server/internal/signal/message_bridge.go:167,197,239,269,299`。
- **影响**：单条慢消息阻塞该连接后续全部消息；大房间/慢后端下活跃但处理慢的连接被误判断连，客户端频繁重连。
- **建议**：handler 投入有界 worker pool 异步执行（ACK 按连接串行保序）；心跳存活判定与 read loop 解耦。

### E2. Hub 单锁保护全部共享 map，且锁内调用 fanout 〔🟡〕
- **问题**：`h.mu` 一把锁覆盖 `rooms/activeStreams/roomStreams/.../kickBans/muteCache`；`clearLocalRoomsForSFUSwitch`、`kickIdentityFromDomain` 持 `h.mu` 时调用 `fanout.Leave/ForEach/GetClient`，与 `OnDisconnect` 反向获取形成潜在锁序风险。
- **证据**：`app/server/internal/signal/hub.go:70-115`；`app/server/internal/signal/hub_sfu.go:93-124`；`app/server/internal/signal/hub_lifecycle.go:215-248`；`app/server/internal/signal/hub_disconnect.go:34-78`。
- **影响**：高并发出入口被单点锁串行化，潜伏死锁/锁序反转。
- **建议**：按领域拆锁；严禁持 `h.mu` 调用 fanout，统一锁序。

### E3. 广播路径同步阻塞（sendAll + sendRaw 背压叠加） 〔🟡〕
- **问题**：`PublishNamespace/PublishRoom` 先**同步** `deliverLocal`，`sendAll` 用 `wg.Wait()` 等所有 `sendRaw`，慢客户端队列满时每个阻塞至多 500ms，且 `publishRoom` 用 `context.Background()` 无超时。
- **证据**：`app/server/internal/bus/nats_bus.go:186-202`；`app/server/internal/bus/ws_deliverer.go:16-30`；`app/server/internal/ws/fanout.go:131-168`；`app/server/internal/ws/client.go:223-241`。
- **影响**：一次广播可阻塞调用 goroutine（往往是 WS handler）数百毫秒，与 E1 叠加放大延迟；无全局背压上限（64 队列 + 丢弃）。
- **建议**：广播投递异步化，handler 只负责入队；控制类消息与高频媒体事件分优先级队列。

### E4. 无锁 setter + 启动顺序使后台 goroutine 与注入产生 data race 〔🟡〕
- **问题**：`SetSFU/SetFanout/SetDomainChecker/SetEventBus/...` 均为无锁字段写入；`gin.go` 先 `StartMembershipHeartbeat()` 启后台 goroutine，再逐个 `Set*` 注入。
- **证据**：`app/server/internal/signal/hub.go:171-189,323-338`；`app/server/internal/signal/hub_bus.go:8-22`；`app/server/internal/signal/state_sync.go:50-58,74-87`；装配 `app/server/server/gin.go:228-268`。
- **影响**：心跳/读 goroutine 可能读到未初始化或正在写入的字段，`-race` 下可复现。
- **建议**：依赖全部通过 `NewHubWithOptions` 一次构造完成，移除启动后 setter。

### E5. 事件投递 worker / 后台 goroutine panic recover 缺口 〔🟡〕
- **问题**：`Dispatch` 有 recover，但 NATS 的 4 个 deliver worker 无 recover；`RemoteHook → HandleRemoteEvent → clearLocalRoomsForSFUSwitch` 在 worker 内同步执行复杂 map/fanout，任一 panic 杀死 worker，4 个全挂则 `deliverCh` 永不消费。队列满还静默丢弃。
- **证据**：`app/server/internal/bus/nats_workers.go:14-33,87-102`；`app/server/internal/signal/hub_lifecycle.go:359-383`；对比 `app/server/internal/ws/handler.go:41-49`。此外 `OnClose→OnDisconnect`（`app/server/internal/ws/client.go:144-146`）、心跳 goroutine（`state_sync.go:74-87`）、cleanup goroutine（`hub_disconnect.go:147-157`）、密钥轮换 goroutine（`jwt_key.go:201-215`）均无 recover。
- **影响**：控制事件可能静默丢失或整个跨实例通道失效，无自愈/告警；goroutine panic 可能杀进程。
- **建议**：所有长生命周期 goroutine 与 worker 统一 `defer recover` + 计数告警；hook 执行与 WS 投递隔离。

### E6. 其他并发小项 〔🔵〕
- **CAS 重试无退避、超次静默放弃；plain 后端非原子读改写**：`app/server/internal/signal/state_sync.go:229-280`（3 次立即 `continue`，`:279` 仅 log）、`:178-225`。建议指数退避 + jitter。
- **`RoomSubject` 未净化**：`app/server/internal/bus/bus.go:45-47` 直接拼原始房间键（可含空格/`:`/中文/`>`/`*`），与 KV 层 `sanitizeKey` 不一致；含非法字符的房间名会导致 publish 失败。建议 subject 段用稳定哈希/编码。
- **`BroadcastMute` 判空不一致**：`app/server/internal/signal/hub_mute.go:14` 判 `info != nil` 但 `:29-30` 直接解引用；生产调用恒传非 nil，其他调用方传 nil 会 panic。
- **`OnDomainDelete` 缺 fanout nil 守卫**：`app/server/internal/signal/hub_lifecycle.go:48-63`（对比 `hub_disconnect.go:80` 有守卫）。

---

## 主题 F. 依赖治理、事务边界与可测试性 🟡

### F1. 组合根用包级全局变量注入，无法并行 DI 且测试互相污染 〔🟡〕
- **证据**：`app/server/internal/middleware/auth.go:42-57`（全局 `defaultAuth`/`Configure`）、`:67-88`（全局 `tokenVersionCache` + `init()` 常驻 goroutine）、`:90-102`；`app/server/internal/service/role_service.go:19-33`（`var onRoleChanged`/`onRoleRenamed`）；`app/server/internal/pkg/jwt.go:33-36`；`app/server/internal/storage/crypto.go:24-26`；`app/server/internal/model/role.go:27-28`；`app/server/internal/config/config.go:115-125`。
- **影响**：未走 bootstrap 的测试/嵌入路径 `model.roleCache` 为空，而 `BanCheck` 依赖 `model.HasBanRole`（`middleware/auth.go:298` → `model/role.go:50-56`），此时封禁角色**fail-open 放行**；`Configure`/role 回调跨测试不复位，用例顺序敏感。
- **建议**：把 `Auth`/`tokenVersionCache`/role callbacks/TTL/Cipher/`roleCache` 收进一个 `Dependencies` 对象显式传入；删除 `init()` 里的全局 goroutine。

### F2. service / repository 普遍依赖具体类型，mock 困难 〔🟡〕
- **证据**：`app/server/internal/service/auth_service.go:59-66`（三个具体指针）、`app/server/internal/service/guest_service.go:37-58`、`app/server/internal/service/email_verification_service.go:27-34`、`app/server/internal/service/domain_service.go:38-40,55`；对比正面 `app/server/internal/service/message_service.go:31-56,73-94`（用接口）。测试已受影响：`auth_service_test.go:30` 只能传 `nil` 绕过邮件分支。
- **建议**：在 service 包为 repository 与相邻 service 定义窄接口，构造函数只收接口，由 `gin.go` 统一绑定。

### F3. service 直接持有 `*gorm.DB` 跨过 repository 写库 〔🟡〕
- **证据**：`app/server/internal/service/guest_service.go:38,50-58,90,184,328,352`；`app/server/internal/audit/audit.go:113-114,141,191,224`；组合根直接传裸 DB（`app/server/server/gin.go:133,367`）。对照 `app/server/internal/service/message_service.go:73-94`。
- **影响**：service 与 GORM 强耦合，无法用内存替身测试（`guest_service_test.go` 必须启 SQLite）；事务边界一半在 service 一半在 repository。
- **建议**：把访客加入/清理事务封装为 `GuestRepository.JoinWithTx`；审计抽 `AuditRepository` 接口。

### F4. repository 承载业务策略，且与 service 重复实现 〔🟡〕
- **证据**：`app/server/internal/repository/domain_repo.go:39-50`（`TransferOwnership` 把"旧 owner 降级 admin"固化在 SQL 事务）、`:23-36`；`app/server/internal/service/domain_service.go:136-139` 创建域后又调 `SeedDefaults`，与 `domain_repo.go:23-36` 内 `SeedDefaultDomainRoles` 重复播种；`app/server/internal/repository/permission_repo.go:72-98`（"默认权限只增不删"策略）。
- **影响**：RBAC 默认策略分散在 repository、`model.DefaultRolePermissions`、`service.DomainService` 三处，策略变更须改数据访问层。
- **建议**：策略收敛到独立 policy 层，repository 只做幂等 upsert 与事务原语；删除重复播种调用。

### F5. `ListParticipants` 错误时返回空数组（历史 B05 回归） 〔🔴〕
- **问题**：SFU 查询失败被转译成 `code:0` 成功响应，监控与前端无法区分"房间无人"与"上游故障"。**累计摘要 `review_cumulative_summary.md:24` 将 B05 标记为已修复，本次复核发现当前代码又回到原样，属回归。**
- **证据**：`app/server/internal/handler/signal_handler.go:246-252`（`if err != nil { pkg.Success(c, []interface{}{}); return }`）。
- **影响**：SFU 不可用/鉴权失败时返回空数组，与错误契约冲突，故障不可观测。
- **建议**：上游错误走 `pkg.HandleError(c, err)`；若业务需要降级，至少带 `degraded: true` 标识并记日志。**建议为 B01–B09 建立回归测试，防止历史修复再次回退。**

### F6. 基础设施层 `InitDB` 返回 `error` 却内部 `panic` 〔🟡〕
- **证据**：`app/server/internal/repository/db.go:82-101`（`panic(fmt.Sprintf(...))`）；调用方 `app/server/server/gin.go:78-81` 按错误链设计。
- **影响**：DB 不可用时无法优雅降级/重试/释放已初始化资源；`%v` 拼接的连接错误可能带 DSN 片段入日志。
- **建议**：改为 `return nil, fmt.Errorf("...: %w", ...)`，由 `StartGin` 统一处理。

---

## 主题 G. 前端状态管理与工程闭环 🟡

### G1. server state 被三套容器同时接管，且设计规范自相矛盾 〔🔴〕
- **问题**：`AGENTS-pages.md` 规定"服务端数据用 `useQuery`"，`AGENTS-stores.md` 又把"域数据"划给 domainStore 并禁止"另建一套缓存"，两份规范冲突；实现里域/成员/消息/权限/角色/房间/访客配置大量进 zustand 与组件 `useState`。
- **证据**：`app/web/docs/design/AGENTS-pages.md:49,51` vs `app/web/docs/design/AGENTS-stores.md:49,64`；`app/web/src/stores/domainStore.ts:19-32,64-73`（手写 `domainCache`/`loadGenerations`/`myDomainsVersion`）；`app/web/src/stores/chatStore.ts:42-66`；`app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx:103-117,380-402,462-473`（页面 `useState` 持 roles/rooms/guestConfig，手写 fetch）；`app/web/src/utils/optimistic.ts:11-12`（注释承认三套并存）。
- **影响**：手写缓存层重复造 React Query 的 stale/invalidate/dedup/竞态能力；跨容器失效割裂（`CreateDomainModal.tsx:43` 只写 zustand，`public-domains` 缓存全仓无 `invalidateQueries`）。
- **建议**：写死分层契约——server state 一律 React Query，zustand 仅放 UI 态/会话态/WS 实时聚合态；聊天消息流若因实时性保留 zustand，作为**显式文档化例外**。

### G2. 同一端点被三套容器重复缓存，首屏请求放大 + 缓存键抖动 〔🟡〕
- **问题**：`/api/v1/domain/my-domains` 同时被 domainStore 命令式加载、sidebar 的 `["my-domains-detailed", ...]`、link 页的 `["my-domains-detailed-link"]` 缓存；`loadMyDomains` 无 in-flight 去重（只有 version 竞态保护），sidebar `queryKey` 直接由 zustand 派生的 `myDomainUUIDs.join(",")` 组成——store 一更新就换 key，再触发全量请求。
- **证据**：`app/web/src/layouts/common/sidebar.tsx:26-38`；`app/web/src/pages/(app)/discover.tsx:140-141`；`app/web/src/pages/(app)/link.tsx:38-50`；`app/web/src/stores/domainStore.ts:89-94`；`app/web/src/stores/domainStore.test.ts:39-50`（测试名自认并发调用都会发请求，仅防旧响应写入）。
- **影响**：进入 `/discover`/`/link` 首屏同一端点可发 3–4 次请求，三份缓存互不同步。
- **建议**：确立单一数据源（React Query 承载 UUID + 详情，zustand 只留 `currentDomainUUID`）；`loadMyDomains` 复用 `queryClient.fetchQuery` 并加 in-flight 去重；移除 `queryKey` 里的 UUID join。

### G3. CI 完全不执行前端测试 〔🔴〕 → 已由 f698ce0 修复
- **问题**：仓库有 62 个前端测试文件（web 50 + packages 12）与整套 `test/` 后端 E2E，但所有 GitHub Actions 工作流均无 `vitest`/`pnpm test`/`turbo run test`。
- **证据**：`.github/workflows/ci.yml:216-237`（Gate `needs: [lint, typecheck, build-js, build-server, test-server, version-check]`）、`:183-203`（唯一 test job 是 `go test ./... -race`）；全 workflows grep `vitest|pnpm test|turbo run test` 零命中。今日工作日志亦印证前端 vitest 为本地手跑（且有 4 个既有 flaky 超时）。
- **影响**：store/socket/条件编译/桌面能力的回归无法在 PR 拦截，`pnpm test` 形同本地脚本。
- **建议**：新增 `test-web`/`test-packages` job 并加入 Gate；`@gospeak/test` E2E 按 `needs.changes` 触发；同时顺手治理那 4 个 flaky（否则接入 CI 会红）。
- **状态（f698ce0）**：已修复。`ci.yml` 新增 `test-web`（`pnpm --filter @gospeak/web test`）与 `test-packages`（components/desktop/sfu-client）两个 job，均按 `needs.changes` 条件触发，并已加入 `gate` 的 `needs` 与结果校验列表。

### G4. React hook 依赖检查关闭，14 条无效 `eslint-disable` 〔🟡〕 → 已由 f698ce0 修复
- **证据**：`biome.json:44-57`（`useExhaustiveDependencies: "off"`、`noExplicitAny: "off"`）；`.github/workflows/ci.yml:84-88`（只跑 `biome ci`）；14 处 `// eslint-disable-next-line react-hooks/exhaustive-deps`（如 `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx:472,486`、`app/web/src/components/room/hooks/useVoiceSession.ts:447`）；全仓无 eslint 配置/依赖。
- **影响**：Biome 不识别 eslint 指令且对应规则本就关闭，这些注释给人"已处理"错觉，实际陈旧 effect 依赖无人把关。
- **建议**：启用 `useExhaustiveDependencies`（warn/error）；14 处改写为 `// biome-ignore lint/correctness/useExhaustiveDependencies: <理由>`。
- **状态（f698ce0）**：已修复。`biome.json:53` 改为 `useExhaustiveDependencies: "warn"`，并新增 `scripts/check-hook-deps.mjs` 棘轮门禁：当前 11 处历史告警为基线，新增即失败；同时双向校验（低于基线也失败），避免 Biome reporter 格式漂移时静默放行。14 条 `eslint-disable` 也已全部改写为带理由的 `// biome-ignore lint/correctness/useExhaustiveDependencies: ...`（当前 0 条 eslint-disable）。剩余 11 处历史告警待逐步清理。

### G5. API/WS 边界类型不安全 + 前后端 DTO 手工镜像且已漂移 〔🟡〕
- **证据**：`app/web/src/api/apiClient.ts:10-14,188-217`（`Result<T = any>`、`resolve(res.data?.data as T)`）；`app/web/src/socket/wsClient.ts:287`（`emitAck(): Promise<any>`）；`app/web/src/stores/chatStore.ts:444,509,896`（ACK 全 `any`）；`app/web/tsconfig.json:32`（`useUnknownInCatchVariables: false`）。DTO 漂移实例：Go `model/domain.go:18-33`（有 `updated_at`、非空 `AllowGuest`、必填 `GuestLimit`）vs TS `app/web/src/api/domain.ts:3-18`（无 `updated_at`、可选字段）；消息模型 Go `status: string` vs TS `deleted: boolean`（`app/web/src/types/message.ts:1-22`）；共享常量 `DefaultGuestLimit` 在前端写死 `?? 5`（`app/web/src/components/domain/GuestAccessSettings.tsx:86`）。
- **影响**：后端字段改名/类型变更不会在编译期暴露，`as T` 让所有调用方"看起来有类型"，契约一致性全靠人工记忆。
- **建议**：在 interceptor 用已装的 `zod` 按 schema parse `data`；去掉默认 `any`；WS ACK 定义 discriminated union；信封与错误码表做成生成物。

### G6. God component / hook / store 与悬空的 loader 架构 〔🟡〕
- **证据**：`app/web/src/stores/chatStore.ts`（1016 行）、`app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`（959 行、约 35 个 `useState`）、`app/web/src/stores/socketStore.ts`（675 行）、`app/web/src/components/room/hooks/useVoiceSession.ts`（530 行、9 ref + 7 effect），均超自订 `<350` 行目标（`app/web/docs/design/AGENTS-pages.md:51`）。loader 悬空：`app/web/src/main.tsx:22-31`（`context: { queryClient }`、`defaultPreload: "intent"`），全 `web/src` 有大量 `beforeLoad` 但**均为守卫/重定向，无一个数据 `loader`/`ensureQueryData`**，preload 配置与 context plumbing 空转。
- **影响**：单文件认知负荷高、闭包陈旧风险不可见、首屏多一跳 RTT。
- **建议**：按领域切 feature 目录，每块独立 `useQuery` + 局部 store；要么把关键列表前置到 `beforeLoad + ensureQueryData`，要么删掉 router context 与 `defaultPreload`。

### G7. Turbo `build` 编排与实际产物脱节 〔🟡〕
- **证据**：`turbo.json:16-18`（`build.dependsOn:["^build"]`、`outputs:["dist/**"]`）；但 `packages/components/tsconfig.json:10`、`packages/desktop/tsconfig.json:10` 均 `noEmit: true`，其 `package.json` 的 `build` 却是 `tsc` ⇒ dist 不存在；`packages/sfu-client` exports 全指向 `./src/*`（`packages/sfu-client/package.json:9-21`），web 也源码直引（`app/web/vite.config.ts:122-129`），`dist` 还混进 6 个测试产物。
- **影响**：`^build` 强制串行但产物无人消费；components/desktop 的 build 实为重复 typecheck；CI 构建时间空耗。
- **建议**：两包 build 改 `tsc --noEmit` 并归入 typecheck；sfu-client tsconfig `exclude:["src/**/*.test.ts"]`；统一"走 dist"或"源码直引"二选一。

### G8. 测试覆盖结构失衡 〔🟡〕
- **证据**：web 50 个测试分布 `utils` 10、`stores` 10、`api` 8、`components/room` 6、`socket` 5，而 `pages` 仅 1、`layouts` 0；`packages/components/package.json:21` 为 `--passWithNoTests`，src 下 0 个测试（15 个导出组件）；`packages/desktop` 9 个能力模块仅 `server.test.ts` 有测试，`bridge.ts`/`runtime.ts` 无。
- **影响**：959 行域管理页、522 行 layout、共享 UI 库、MyGo 桥接无自动化回归。
- **建议**：给 `packages/components` 补组件测试、`bridge.ts` 补 mock 测试、3 个 god page 加冒烟测试，并接入 CI（G3）。

---

## 主题 H. 文档与实现漂移 🔵

> 这一节是本次审查相对历史 review 的**增量视角**：代码在演进，文档与旧 review 结论未同步。

1. **`_solid-legacy` 已删除但文档仍在描述它存在**〔🔵〕：`app/web/README.md:5`、`app/web/docs/AGENTS.md:7` 声明已删除（本次 `Glob` 复核确认目录不存在、无 `solid-js` 依赖、构建配置无残留 exclude），但根 `AGENTS.md:24` 仍写 `app/web/src/_solid-legacy/ # 迁移暂存的 SolidJS 旧代码，不参与构建`，`ARCHITECTURE.md` 亦提及。另有用户可见 404 文案仍写"或该页面尚未从 Solid 迁移到 React"（`app/web/src/main.tsx:38`），误导用户。**建议**同步根 AGENTS.md/ARCHITECTURE.md 与 404 文案。
2. **WS Origin 文档 vs 实现**：配置注释称未配置时默认同源，实现默认 `*` 放行（详见 B3）。
3. **SFU 契约文档 vs 实现**：`AGENTS.md` 声明"不支持的操作 MUST 返回 `NewErrSFUNotSupported()`"，LiveKit 实现从不返回（详见 C1）；`AGENTS.md` 的多 provider/动态切换叙事与"SFU_PROVIDER 固定为 livekit"并存。
4. **cluster/mediasoup 残留**：`app/server/internal/signal/hub.go:61-68`、`app/server/internal/bus/AGENTS.md` 仍有 mediasoup/cluster 注释，但 factory 只支持 LiveKit；反映从多 SFU + Redis 精简的历史（呼应 A1）。
5. **历史 review B05 状态失真**：`docs/review/review_cumulative_summary.md:24` 标"已修复"，实际回归（详见 F5）。**建议**为 P0 级历史修复补回归测试或状态复检机制。

---

## 已验证非问题（避免误报 / 肯定做得好的部分）

- **后端分层主线干净**：`internal/handler` 全部生产 `.go` 未 import `repository`/`gorm`（12 个命中均为 `_test.go` fixture）；`internal/service` 无 `gin.Context`/`c.JSON`/`pkg.Success`；`internal/repository` 无 `internal/service` 反向依赖、无 `gin.Context`。
- **无 service 间循环依赖**：`roleSvc → permSvc` 单向（`gin.go:118-125`），`permSvc` 不回调 `roleSvc`。
- **JSON API 错误脱敏基础设施完整**：`pkg.HandleError`/`ClientError` 对 `INTERNAL_ERROR`/`SFU_ERROR`/`STORAGE_ERROR`/`OAUTH_*`/`EMAIL_SEND_FAILED` 正确隐藏细节；约 70 处 `pkg.Fail(c, INVALID_PARAMS, err.Error())` 均为 `ShouldBindJSON` 输入回显，属正常。
- **黑名单 fail-closed**：生产 HTTP/WS 均用 `authstate.IsBlacklistedErr`（`gin.go:402`、`middleware/auth.go:124-129`）；fail-open 的 `IsBlacklisted`（`authstate/blacklist.go:72-84`）仅测试使用。
- **前端包边界清晰**：web 无 `livekit-client` 直连（唯一引用在 `packages/sfu-client/src/livekit-client.ts:16`）、无 `window.mygo` 直连（全经 `@gospeak/desktop`）、三包互不 import、无反向依赖；store 依赖为 DAG（`socketStore.ts:22-35` 用回调注册规避与 `chatStore` 的环）。
- **条件编译与命令校验有 CI 保障**：token 由 `app/web/vite.config.ts:37-39` 统一注入；CI 校验 Go 绑定与 TS command 一致性（`.github/workflows/ci.yml:94-100`）。
- **`_solid-legacy` 清理彻底**（详见 H1），无死代码。

---

## 优先级路线图

### P0（先定方向 + 堵安全/数据面）
1. **A1 决策部署模型**：单实例 or 多实例。这是 A2/A3/D4 等一系列问题的总开关——不先定，后续改动都可能推倒重来。
2. **A2 密钥持久化独立于总线**：无论单/多实例都必做——给内嵌 NATS 固定并持久化 StoreDir（且挂 volume）、或接外部 NATS、或把密钥/黑名单落 DB，消除"重启换 key 全员重登 + logout 吊销失效"。（注：正常部署不会因重启 panic；panic 仅在 backend 为空/KV 全挂且 JWT_KEY 缺失时触发）
3. **B1 WS 统一策略入口**：把封禁/访客判定抽成传输无关 Policy，建连 + 按事件各执行一次。
4. **B2 OAuth 错误脱敏**：复用 `pkg.ClientError`，token 改 Header/脱敏。
5. **F5 + 历史回归防护**：修 `ListParticipants`，为 B01–B09 补回归测试。
6. ~~**G3 前端测试接入 CI**（先治理 4 个 flaky）。~~ 已由 f698ce0 完成：`test-web`/`test-packages` 已进 CI 与 Gate。

### P1（契约收敛 + 一致性 + 并发可靠性）
- C1/C2：修正 `ErrSFUNotSupported` 与 Capabilities 契约，加契约测试。
- D1：角色改名事务化 / 禁止改内置角色（与 B5 联动）。
- D2/E1/E3：KV 超时、read loop 异步化、广播异步化。
- B3/B4/B5/B6：安全边界与小项脱敏收口。
- G1/G2：统一 server state 数据源，消除重复请求。
- E5/F1：panic recover 补全、全局状态收进 `Dependencies`。

### P2（复杂度清理 + 长期演进）
- A1 后续：按部署模型决定是删除还是坐实跨实例抽象；清理 cluster/mediasoup 残留。
- C3/C4/C5、D3、E2/E4/E6、F2/F3/F4/F6：接口化、拆锁、KV 编码、事务边界归一。
- ~~G4~~（已由 f698ce0 完成：warn + 棘轮门禁）/G5/G6/G7/G8：DTO 生成、god component 拆分、turbo 编排、测试结构。
- H：文档与实现漂移系统性对齐。

---

## 审查方法与边界

- 本报告为**静态只读审查**，未运行服务、未做动态压测或 `-race` 实测；E 类并发问题为代码路径推断，建议后续用 `go test -race` 与压测验证。
- 结论基于 2026-11-10 工作区快照；与 `docs/review/review_cumulative_summary.md`（2026-08-05，Round 1–6）交叉比对，标注了回归与演进残留。
- 严重度是**架构风险**评估，不等于修复工单优先级；实际排期请结合版本节奏与业务影响。
