# 会话过期时间下发与主动续期（expires_in + 懒续期）设计

- 日期：2026-08-29
- 状态：已批准（方案 A：expires_in + 懒续期；TTL 顺带提为 env 可配）
- 关联：`2026-08-28-invite-page-standalone-design.md`（同批未提交的路由闪烁修复）

## 背景

access/refresh token 由 HttpOnly Cookie 承载（保持不变，这是安全上的正确选择），前端读不到 token 本体，因此无法本地解码 `exp`。当前 `(app)` 布局路由的 `beforeLoad → userStore.ensureSession()` 每次导航同步阻塞一次 `GET /user/profile` 来探测会话死活——这是路由切换闪烁/卡顿的遗留根因（TanStack Router 的 `shouldSkipLoader` 不跳过客户端重跑，预载结果不作数）。

采用 OAuth2 标准的 `expires_in` 模式：服务端在签发/续期响应中下发过期秒数，客户端据此做**懒续期**——未过期时守卫同步放行，临近过期才无感刷新。

## 目标

1. 已登录状态下切换路由不再产生每次导航一次的串行 `/user/profile` RTT。
2. 会话真实过期时间驱动续期：距过期 > 60s 同步放行；≤ 60s 先 `refresh_token` 再放行。
3. 硬编码 TTL（`pkg/jwt.go:33-37` 的 15min / 7d）提为 env 可配。

## 非目标

- 不做后台定时器、不做多标签 leader 协调（懒续期已覆盖导航与请求两条路径）。
- 不改撤销类失效（封号 / 改密踢下线 / TokenVersion）的兜底架构——`apiClient` 401 拦截器保持现状。
- guest join、bot token 不下发 `expires_in`。
- 跨标签页 refresh 竞态维持现状（refresh token 轮换下两 tab 同时刷新后到者失败；懒续期把触发频率从「每次 401」降到「每 15 分钟级」，概率反而下降）。

## 后端改动

1. **TTL 可配**（`internal/config/config.go`）：
   - 新增 `JWTAccessTTL`（env `JWT_ACCESS_TTL`，默认 `15m`）、`JWTRefreshTTL`（env `JWT_REFRESH_TTL`，默认 `168h`）。沿用 `JWTKeyTTL` 的「string 字段 + `time.ParseDuration` 方法」模式（参考 config.go:47,249），非法值/非正值回落默认值，不阻塞启动。
2. **TTL 注入**（`internal/pkg/jwt.go` + `server/gin.go`）：
   - `AccessTokenTTL` / `RefreshTokenTTL` 从 `const` 改为包级 `var`，`gin.go` 装配时由 config 显式注入。`GenerateToken` 等调用方零改动。
3. **响应体下发**（`internal/service/auth_service.go`）：
   - `AuthResponse`、`RefreshResponse` 增加 `ExpiresIn int64` `json:"expires_in"`（秒）。Login / Register / RefreshFromToken 填充 `int64(pkg.AccessTokenTTL / time.Second)`。
4. **OAuth 回调**（`internal/handler/oauth_handler.go`）：
   - callback 无 JSON body 且既有设计禁止 token 进 URL；`oauthBridgeHTML` 的 postMessage payload（当前 `map[string]bool{"ok": true}`）扩展为携带 `expires_in`。cookie 写入逻辑不变。

## 前端改动

1. **authTransport**（`src/api/authTransport.ts`）：`refreshSession()` 返回值从 `void` 改为 `Promise<number | null>`（透传响应体 `expires_in`）；`pendingRefresh` 去重保留。
2. **userStore**（`src/stores/userStore.ts`）：
   - 新增 `sessionExpiresAt`（localStorage 独立 key `gospeak_session_expires_at`，毫秒时间戳）与记录入口。
   - `loginAction` 签名加 `expiresIn?`；`logout` / `clearAuth` 清除 `sessionExpiresAt`。
   - `ensureSession` 新逻辑：
     1. `user()` 为空 → 现状探测路径（profile → 失败则 refresh → 重试；失败清理并返回 false）。冷启动、localStorage 被清但 cookie 存活的场景由此覆盖。
     2. `user()` 存在且 `now < sessionExpiresAt - 60s` → **同步返回 true，零网络请求**。
     3. 其余（临近过期 / 已过期 / 无 `sessionExpiresAt` 的存量数据）→ `refreshSession()`（记录 `expires_in`）→ `fetchProfileAction()` 收尾。失败处理同现状：1017 且有缓存会话则沿用，否则清理返回 false。
     - 存量桥接：无 `sessionExpiresAt` 时以 `lastVerifiedAt`（最近一次 profile 验证成功时刻，内存态）宽限 10 分钟——窗口内同步放行，窗口过后自动 refresh 一次换取真实 `expires_in`，此后进入零 RTT 节奏。
     - 无 `sessionExpiresAt` 视为已过期，行为不劣于现状。
   - 实现注意：快速路径（同步 return true）会让 async IIFE 同步完成，IIFE 内部 `finally` 清理 in-flight 去重变量会在赋值前执行并被覆盖（一次性永久泄漏）——清理必须挂在外层 promise 的 `.finally` 并加身份守卫。
   - `userStale` 语义不变。
3. **api/auth.ts**：`LoginData` 增加 `expires_in?: number`；login / register 将其透传给 `userStore.login`。
4. **登录页 OAuth 接收**（`src/pages/login/index.tsx:58,89`）：`completeOAuthLogin()` 接收 payload 中的 `expires_in`，先记录再走 `ensureSession()`。
5. **不变**：`(app)/route.tsx` 的 `beforeLoad` 调用点、`apiClient` 401 拦截器、manage 等本地权限 `beforeLoad`。

## 边界与已知限制

- 撤销类失效的感知从「渲染前探测」变为「页面已渲染 → 请求 401 → 拦截器兜底跳登录」（Discord 同款行为）。
- 存量已登录用户首个会话周期仍有一次 profile 探测（本地无 `sessionExpiresAt`），之后进入懒续期节奏。
- 前端时钟偏移：`expires_in` 以响应到达时刻起算，不信任本地绝对时间；60s margin 吸收小偏移与网络延迟。
- WS 长连接不受影响（token 仅在升级时校验）。

## 测试计划

- 后端：login / register / refresh 响应断言含 `expires_in`；TTL env 默认值、自定义值、非法值启动失败；现有 auth handler 测试回归。
- 前端：未过期同步放行且零网络请求；临近过期先 refresh 后 profile；`expires_in` 记录与持久化；登出清理；无 `sessionExpiresAt` 存量路径（含 10 分钟宽限窗口内放行）；`refreshSession` 返回值；OAuth payload 接收。补入 `userStore.test.ts` 等现有测试文件。

## 验收标准

1. 已登录状态连续切换 `(app)` 子路由，Network 面板无每导航一次的 `/user/profile` 请求（稳态下约 15 分钟一次 refresh）。
2. 路由切换闪烁消除（与本批未提交修复叠加后）。
3. `pnpm test:web` 全绿；`go build ./...` 与后端测试通过。
