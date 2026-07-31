# WS 迁移待办清单

> 2026-07-31，基于迁移恢复后的工作区审查整理。

## 当前状态

- 已完成：Socket.IO → WebSocket 迁移主体（后端信号层、Web 前端、SFU client、bot、部署配置、文档）。
- 已通过：`go build ./...`、`go test ./internal/ws ./internal/signal`、web/bot/sfu-client 三个 `tsc --noEmit`。
- 本清单记录审查发现的遗留问题，按严重度排序。

## 🔴 必须修复

1. `packages/bot/src/runtime/socketClient.ts:150`
   - 问题：连接在 `onopen` 前被服务端关闭时，`connect()` Promise 永远不结束，bot 启动可能卡死。
   - 修复：`onclose` 中若 `connectReject` 仍存在则 reject，并清空回调。

2. `packages/bot/src/runtime/socketClient.ts:99`
   - 问题：异常断线后 `this.socket` 未置 null，后续再次 `connect()` 直接 resolve，实际不会重连。
   - 修复：`onclose` 中把 `this.socket` 置 null，由调用方或重连逻辑重新建连。

## 🟡 待优化

3. `app/web/src/socket/wsClient.ts:104`
   - 问题：坏帧被静默吞掉，ACK 会等 10 秒超时，服务端推送事件也可能无声丢失。
   - 修复：区分解析失败与协议错误，至少 `console.warn` 记录。

4. `app/web/src/socket/wsClient.ts:143`
   - 问题：固定 3 秒重连，无退避/抖动，服务端重启时容易形成重连风暴。
   - 修复：指数退避（如 `min(3s * 2^n, 30s)`）+ 随机 jitter。

5. `app/web/src/socket/wsClient.ts:97`、`packages/bot/src/runtime/socketClient.ts:106`
   - 问题：JWT 拼入 WS URL query，会进入访问日志、代理日志和浏览器历史。
   - 修复：改用短时一次性 ticket，或 cookie + 服务端白名单。

6. `app/server/internal/ws/upgrader.go:67`
   - 问题：`InsecureSkipVerify: true` 且依赖 Gin CORS 拦截跨站 WS，但 Gin CORS 不覆盖 WS 握手。
   - 修复：在 Upgrader 内做 Origin 白名单校验，删除错误注释。

7. `app/web/src/stores/chatStore.ts` 发送路径
   - 问题：`emitAck().catch(removePending)` 在断线/ACK 超时时直接移除乐观消息，用户输入消失且无提示。
   - 修复：失败时保留草稿或显示错误，允许重试。

8. `packages/bot/src/runtime/socketClient.ts:123`
   - 问题：bot 侧坏帧同样静默吞掉，ACK 超时无日志，排障困难。
   - 修复：解析失败时记录 `logger.warn`。

## 🔵 顺手清理

9. `app/server/server/gin.go:344`
   - 问题：`OnConnect` 返回的 error 被 `_` 吞掉，未来失败时无法拒绝连接。
   - 修复：回调签名改为可返回 error，Upgrader 失败时关闭客户端。

10. `app/web/src/socket/wsClient.ts:87`
    - 问题：手动 `connect()` 不会先清掉已排队的 `reconnectTimer`，CONNECTING 期间可能重复建连。
    - 修复：进入 `connect()` 时先 `clearTimeout(reconnectTimer)`。

11. `app/web/vite.config.ts:114`
    - 问题：文件保持 CRLF，`git diff --check` 将新增 `/ws` 行报为 trailing whitespace。
    - 修复：统一 LF，或仓库配置 `core.whitespace=cr-at-eol`。

## 验证命令

- `cd app/server && go build ./... && go test ./internal/ws ./internal/signal -count=1`
- `pnpm --filter @gospeak/web exec tsc --noEmit`
- `pnpm --filter @gospeak/sfu-client exec tsc --noEmit`
- `pnpm --filter @gospeak/bot exec tsc --noEmit`
