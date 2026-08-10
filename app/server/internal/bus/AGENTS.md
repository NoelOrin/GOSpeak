# bus 模块

多实例事件总线与共享状态。

## 能力

| 能力 | 后端 | 用途 |
|------|------|------|
| EventBus | NATS (embedded/external) | WebSocket 跨实例 fanout（room/namespace）+ internal 事件 |
| MembershipStore | nats KV → none | 房间成员 / stream 映射共享 |
| MuteRuleStore | nats KV → memory | Agora kicking-rule id 等降级 mute 缓存 |
| AuthStore | nats KV | JWT 黑名单 + 签名密钥轮换 |
| JobQueue | NATS JetStream | SFU cleanup 等异步任务 |

## 解析顺序

- `STATE_STORE=auto`：nats → 降级
- MuteRule 最终至少 memory（不阻断启动）
- Auth：尝试 NATS KV；不可用则静态 `JWT_KEY`

## 约束

- membership 写入按 `InstanceID` 合并，禁止整房覆盖远端成员
- stream leave/kick/disconnect 必须 `DeleteStream`
- 客户端广播走 `publishRoom` / `publishNamespace`，不要裸 `BroadcastToRoom`
