# TokenType 鉴权收紧设计

日期：2026-08-02

## 目标

修复 `VerifyToken` 未校验 `TokenType` 的鉴权漏洞，避免 refresh token 和 ws-ticket 被当作普通 HTTP/WS 访问凭证使用。

## 现状

- `VerifyToken` 当前只校验签名、过期、黑名单和 token 版本，不校验 `TokenType`。
- HTTP `JWTAuth` 因此会接受 refresh token 和 ws-ticket。
- WebSocket 握手只对 subprotocol 来源额外检查 `IsWSTicket`，通过 Authorization 或 cookie 传入 refresh token 时仍可建立连接。
- 前端与 Bot 客户端已统一通过 `/api/v1/signal/ws-ticket` 获取短时 ticket，再用 `Sec-WebSocket-Protocol` 连接 `/ws`。

## 约束

- HTTP 只放行 `access` / `bot`。
- WebSocket 只接受 subprotocol 传入的 `ws-ticket`。
- 本次只修改 `app/server/internal/middleware/auth.go` 和 `app/server/internal/ws/upgrader.go`。
- `/api/v1/system/stream` 的独立 `ParseToken` 校验不在本次范围内，后续单独处理。

## 设计

### 1. middleware 校验入口

在 `middleware/auth.go` 中抽出一个带类型白名单的底层校验函数：

```go
func verifyToken(tokenStr string, allowedTypes ...string) (*pkg.Claims, pkg.ErrCode)
```

现有校验链保持：签名解析、过期、黑名单、token 版本，并追加 `TokenType` 白名单匹配。

公开入口调整为：

- `VerifyToken(tokenStr)`：调用底层函数，只传 `pkg.AccessTokenType` 和 `pkg.BotTokenType`。
- `VerifyWSTicket(tokenStr)`：调用底层函数，只传 `pkg.WSTicketType`，并在通过后继续校验 `pkg.WSTicketExpired`。

### 2. WebSocket 握手

在 `ws/upgrader.go` 中：

- `extractToken` 改为仅遍历 `Sec-WebSocket-Protocol`，移除 Authorization 和 cookie 提取分支。
- `ServeHTTP` 要求 `fromSubprotocol == true`，否则直接返回 `401`。
- 对 subprotocol 中的 token 调用 `middleware.VerifyWSTicket`，不再调用 `VerifyToken`。

## 数据流

- HTTP 请求进入 `JWTAuth` 后调用 `VerifyToken`。
- 合法 `access` / `bot` token 继续进入请求上下文；refresh / ws-ticket 被拒绝。
- WebSocket 握手从 `Sec-WebSocket-Protocol` 提取 ticket。
- `VerifyWSTicket` 通过后，Upgrader 正常升级连接并创建 `Client`。
- header/cookie 中的任何 token 都不再用于建立 WS 连接。

## 错误处理

- TokenType 不符：返回 `TOKEN_WRONG`，HTTP 响应 `401`。
- Token 过期：返回 `TOKEN_EXPIRED`，HTTP 响应 `401`。
- 黑名单或 token 版本不匹配：返回 `TOKEN_REVOKED`，HTTP 响应 `401`。
- WS 缺少 subprotocol ticket：拒绝升级，返回 `401`。
- WS ticket 超过 45 秒短窗口：拒绝升级，返回 `401`。

## 测试

- `middleware/auth_test.go`：
  - `VerifyToken` 接受 access token。
  - `VerifyToken` 接受 bot token。
  - `VerifyToken` 拒绝 refresh token。
  - `VerifyToken` 拒绝 ws-ticket。
  - `VerifyWSTicket` 接受 ws-ticket。
  - `VerifyWSTicket` 拒绝 access / bot / refresh token。
- `ws/upgrader_test.go`：
  - 更新提取测试，确认 header/cookie 不再作为 WS 凭据。
  - 保留 subprotocol 提取测试。
  - 补充 `ServeHTTP` 对 header/cookie token 返回 `401` 的覆盖。
- 保留并运行现有 `upgrader_e2e_test.go` 的合法 ticket 生命周期测试。
- 验证命令：`cd app/server && go test ./internal/middleware ./internal/ws`。

## 范围外

- 不修改 JWT 签发逻辑和 TokenType 常量。
- 不修改前端或 Bot 客户端连接方式。
- 不处理 `/api/v1/system/stream` 的独立鉴权逻辑。
- 不调整现有业务路由和权限码。
