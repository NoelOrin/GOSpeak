# SRS 多流隔离与动态订阅设计

日期: 2026-07-05
分支: feature/srs-sfu
前置: `2026-07-05-srs-selfhost-e2e-design.md`(自部署 e2e 已打通)

## 背景

SRS selfhost e2e 已打通单客户端 WHIP/WHEP 通路。但 `joinRoom` 当前只支持单流:

- 后端 `internal/srs/provider.go:42` `whipURL = http://host:1985/rtc/v1/whip/` 无 `?app=&stream=` query → SRS 默认 `app=live, stream=livestream`,**所有客户端共享同一流**
- 前端 `packages/sfu-client/src/srs-client.ts` 单个 `subscribePc`,WHEP 订阅固定流,`onRemoteAudioTrackCb` 上报的 `identity` 是 `event.track.id` 而非真实参与者身份
- `joinRoom(token, url, identity, room?)` 第 4 参 `room` 被忽略,接口无法承载流名

后果:
1. 第二人 WHIP publish 同流 → SRS `code=5020 RtcStreamBusy` → 关连接不回 HTTP → `ERR_EMPTY_RESPONSE`
2. WHEP 订阅的是 `livestream`(可能是自己) → self-subscribe 回声 + 无法定向他人
3. 无远端参与者发现机制(LiveKit 靠 SDK 事件,MediaSoup 靠 `sfu:producer-ready` 信令,SRS 无对应)

前序 turn 已修 L1(joinRoom 失败清理)+ L2(WHEP 前 wait publish ICE connected)。本设计修 L3:多流隔离 + 动态订阅 + 鉴权 + 重订阅。

## 目标

1. **流隔离**: 每客户端 WHIP publish 独立流,URL 带 `?app=live&stream=<streamName>`
2. **动态订阅**: 客户端按远端参与者列表逐个 WHEP 订阅,`onRemoteAudioTrack` 上报真实 `identity`
3. **参与者发现**: 经现有信令 `member:joined`/`member:left` + `room:join:sfu` ack 携带 `stream` 字段
4. **鉴权**: SRS `on_publish`/`on_play` HTTP callback + 无状态 HMAC,防止伪造 stream 抢占/冒充
5. **重订阅**: WHEP 失败或 subscribe PC ICE failed 时退避重试,覆盖 join 竞态与瞬时断连
6. **零跨 provider 影响**: LiveKit/MediaSoup/Daily/Agora 行为不变,新增接口方法均为可选

## 非目标

- SRS REST `ListParticipants`/`Mute` 补齐(另议)
- prod 部署的 SRS auth 强制(本设计 callback 鉴权已覆盖 dev/LAN,prod 同方案)
- 多设备 LAN candidate 配置(已有 runbook 说明)
- subscribe PC ICE restart(重订阅即重建,不实现 restartIce)

## 流名与鉴权

### 流名

```
stream = "gs-" + base36(sha256(room + ":" + identity))[:12]
```

- 确定性: 同 `(room, identity)` 必同 stream
- ASCII-safe: 中文 room/identity 不破坏 SRS stream 路由
- 服务端计算,客户端不经手(防伪造)

### Stream Token (鉴权凭证)

```
streamToken = base36(hmac_sha256(SRS_SECRET, stream))[:16]
```

- 无状态,无 DB,基于现有 `SRS_SECRET`
- 每客户端通过 token API(JWT 已认证)获取自己的 `stream` + `streamToken`
- `member:joined` 广播仅含 `stream`,**不含 `streamToken`** → 攻击者知 stream 无 token 无法 WHIP/WHEP

### SRS HTTP Callback

SRS `on_publish`/`on_play` 触发时 POST 后端 callback,后端从 payload 解析 `stream` + `streamToken`(经 SRS `param` query 透传),重算 HMAC 比对:

- 匹配 → `{"code":0}` 放行
- 不匹配 → `{"code":403}` 拒绝,SRS 关闭该 publish/play session

SRS `param` 字段限制: SRS `on_publish` payload 的 `param` 即 WHIP/WHEP URL query string。客户端 URL `?app=live&stream=<s>&token=<t>`,SRS 转发 `param="app=live&stream=<s>&token=<t>"` 到 callback,后端解析。

## 数据流

```
1. POST /api/v1/signal/token {room, identity}
   → SRS Service.GenerateStreamName(room, identity) = stream
   → SRS Service.GenerateStreamToken(stream) = streamToken
   → response: {token, serverUrl, room, identity, provider, whipUrl,
                stream, streamToken}            (SRS only)

2. joinRoom(token, url, identity, room, stream?, streamToken?)
   → WHIP POST url + "?app=live&stream=" + stream + "&token=" + streamToken
   → 不再 self-subscribe
   → 注册 socket 听 member:joined / member:left

3. socketStore.joinRoomSFU(room, identity, stream)  ← emit 携带 stream
   → hub.OnRoomJoinSFU: 服务端重算 stream 校验 emit 值(防伪造),
     存 MemberInfo.Stream,广播 member:joined {room, identity, id, stream}
   → ack 返回 members[] 含 {identity, stream}
   → useRoomJoinSession 拿 ack members,调 client.subscribePeers(members)

4. client.subscribePeers([{identity, stream}])
   → 每个 peer: WHEP POST url + "?app=live&stream=" + peerStream
     (WHEP 不带 token,鉴权靠 on_play callback 查 activeStreams,见下「WHEP 鉴权」)

5. socket member:joined {identity, stream}
   → SRS client WHEP 订阅新 peer

6. socket member:left {identity}
   → SRS client 取消订阅(DELETE resource + close PC + onRemoteAudioTrackRemoved)
```

### WHEP 鉴权修正

on_publish 校验发布者持有 stream 的合法 token。on_play 若同样校验「请求方持有 stream 的合法 token」,则订阅者需知 peer 的 streamToken — 但 streamToken 不广播,只自己拿。

两个方案:

**方案 A(推荐): on_play 不校验 token,仅校验 stream 已存在且已 publish**
- on_publish callback 校验 stream+token 绑定(防伪造发布)
- on_play callback 仅校验 stream 在「当前活跃 publish 流」列表中(后端内存维护 `activeStreams map[string]struct{}`)
- 订阅者无需 peer 的 token,只需 stream(广播可得)
- 攻击面: 知 stream 名即可 play。但 play 仅收听,不冒充发布,dev/LAN 可接受

**方案 B: 订阅者持自己 token,on_play 校验请求方是同 room 成员**
- callback 校验「请求方 streamToken 对应的 stream 属于同 room」
- 后端维护 `streamToRoom map`,on_play 查 room 成员关系
- 更严,但需 stream→room 映射 + room 成员查询,复杂度高

选 **方案 A**: 发布侧强鉴权(防冒充),订阅侧弱鉴权(防不了偷听,但偷听需先突破网络边界)。匹配 runbook「安全靠网络边界」定位。

### 活跃流维护

后端 signal Hub 内存维护(已有 room/members 内存态):

```go
type roomStreamRegistry struct {
    active map[string]struct{}  // stream → exists, key room-scoped not needed
}
```

- on_publish callback 成功 → 加 active
- on_unpublish callback → 删 active
- on_play callback → 查 active 存在则放行

Hub 已有 room/members 内存态,stream registry 同生命周期,无新持久化。

## 改动文件

### 后端 Go

| 文件 | 改动 |
|------|------|
| `app/server/internal/srs/stream.go`(新) | `GenerateStreamName(room, identity) string`、`GenerateStreamToken(stream, secret) string`、`ValidateStreamToken(stream, token, secret) bool` |
| `app/server/internal/srs/provider.go` | Service 持 secret;暴露 `GenerateStreamName`/`GenerateStreamToken` 方法供 handler 调用 |
| `app/server/internal/handler/signal_handler.go` | `GetJoinToken`: SRS 时 `data["stream"]`、`data["streamToken"]`;新增 `SrsCallback(c *gin.Context)` 处理 on_publish/on_play/on_unpublish/on_stop |
| `app/server/internal/router/routes/srs/`(新) | `POST /api/v1/srs/callback` 路由(无 JWT,内网 SRS→backend) |
| `app/server/internal/signal/hub.go` | `OnRoomJoinSFU`: 校验 emit 的 stream 与服务端重算一致,存 `MemberInfo.Stream`,`member:joined` 广播 + ack 携带 stream;维护 `activeStreams` 供 callback 查询 |
| `app/server/internal/signal/events.go` 或 `model` | `MemberInfo` 加 `Stream string json:"stream,omitempty"` |
| `app/server/internal/signal/hub.go` 或新 `stream_registry.go` | `activeStreams` 内存 map + `RegisterStream`/`UnregisterStream`/`IsStreamActive` |

### 前端 TS

| 文件 | 改动 |
|------|------|
| `app/web/src/api/sfu.ts` | `JoinTokenResponse` 加 `stream?: string`、`streamToken?: string` |
| `app/web/src/stores/socketStore.ts` | `joinRoomSFU(room, identity, stream?)` emit 带流;`member:joined`/`member:left` 存 stream;`joinRoomSFU` ack 返回 members 给调用方 |
| `app/web/src/components/room/hooks/useRoomJoinSession.ts` | 传 `data.stream` 给 joinRoom;joinRoomSFU 后调 `client.subscribePeers?.(members)`;member:joined/left 转发 client |
| `packages/sfu-client/src/types.ts` | `joinRoom` 加 `stream?`/`streamToken?` 第 5/6 参;加 `subscribePeers?(peers: {identity, stream}[])`、`unsubscribePeer?(identity)` 可选方法 |
| `packages/sfu-client/src/srs-client.ts` | 重构: per-peer subscribe PC `Map<identity, PeerSub>`;WHIP/WHEP URL 带 stream+token;socket 听 member:joined/left;WHEP 退避重试;真实 identity 上报;leaveRoom 清全部 |

### 部署

| 文件 | 改动 |
|------|------|
| `deploy/srs/srs.conf` | 加 `http_callback { enabled on; on_publish/on_unpublish/on_play/on_stop http://host.docker.internal:8998/api/v1/srs/callback; }` |
| `deploy/docker-compose.example.yml` | srs 服务 `CANDIDATE` 已确认 `127.0.0.1`;callback URL 用 `host.docker.internal`(extra_hosts 已有) |

## srs-client.ts 重构细节

### per-peer 订阅状态

```typescript
interface PeerSub {
  identity: string;
  stream: string;
  pc: RTCPeerConnection;
  resourceUrl: string;
  retryCount: number;
  retryTimer: ReturnType<typeof setTimeout> | null;
}
private peerSubs = new Map<string, PeerSub>();
```

### 订阅流程

```
subscribePeer(identity, stream):
  若 peerSubs 已存在 → 跳过
  创建 subscribePc + recvonly transceiver
  exchangeSdp WHEP url + "?stream=" + stream  (token 不带,WHEP 鉴权靠 on_play 查 active)
  成功 → 存 PeerSub, onRemoteAudioTrack({identity, track})
  失败 → scheduleRetry(identity, stream)

scheduleRetry(identity, stream):
  retryCount++
  若 retryCount > 5 → 放弃, onRemoteAudioTrackRemoved(identity)
  否则 delay = 2^(retryCount-1) * 1000ms (1s/2s/4s/8s/16s)
  retryTimer = setTimeout(() => subscribePeer(identity, stream), delay)
```

### ICE failed 处理

subscribePc `iceconnectionstatechange` → `failed`:
- close pc, DELETE resource
- 调 `scheduleRetry` (从 retryCount=0 重计,视为全新订阅)

### unsubscribePeer(identity)

- 清 retryTimer
- close pc, DELETE resource
- 从 peerSubs 删
- `onRemoteAudioTrackRemoved(identity)`

### socket 事件接入

srs-client 在 `joinRoom` 拿 `options.socket`(MediaSoup 已用此通道),注册:
- `socket.on("member:joined", ({identity, stream}) => this.subscribePeer(identity, stream))`
- `socket.on("member:left", ({identity}) => this.unsubscribePeer(identity))`

初始成员: `joinRoomSFU` ack 返回 members,`useRoomJoinSession` 调 `client.subscribePeers(members)`。但 srs-client 自己也听 `member:joined`,需去重(peerSubs 已存在则跳过)。

### leaveRoom

遍历 `peerSubs` 全部 `unsubscribePeer`,再清 publish PC。

## 信令字段

### `room:join:sfu` emit (C→S)

```json
{"room": "R", "identity": "alice", "stream": "gs-abc123def456"}
```

### `room:join:sfu` ack (S→C)

```json
{"ok": true, "room": "R", "identity": "alice",
 "members": [{"identity": "bob", "stream": "gs-xyz789..."}]}
```

### `member:joined` (S→C broadcast)

```json
{"room": "R", "identity": "bob", "id": "socket-xyz", "stream": "gs-xyz789..."}
```

`stream` 字段 `omitempty`,非 SRS provider 不带,旧客户端忽略。

## 错误处理

| 场景 | 处理 |
|------|------|
| SRS callback 不可达 | SRS fail-closed 拒 publish。部署文档警告:保证 SRS→backend 网络 |
| on_publish token 不匹配 | callback 返 403,SRS 关 session,客户端 WHIP fetch 失败 → joinRoom 抛错(走 L1 清理) |
| WHEP 对未 active 流 | on_play callback 返 403,SRS 关 session。客户端 scheduleRetry 等 peer publish |
| 重试 5 次全失败 | 放弃订阅该 peer,onRemoteAudioTrackRemoved |
| member:left 到达时有 pending retry | 取消 timer,立即 cleanup |
| client 重连后 stream 变 | stream 由 (room,identity) 确定,重连同身份同 stream,SRS 旧 session 已 unpublish,可重新 publish |

## 测试

### 后端单测

- `stream_test.go`: `GenerateStreamName` 确定性、ASCII-safe、不同输入不同输出;`GenerateStreamToken`/`ValidateStreamToken` 正例反例
- `hub_test.go` 或 `stream_registry_test.go`: activeStreams 注册/注销/查询并发安全
- callback handler: on_publish 合法/非法 token、on_play active/inactive stream

### 前端

- srs-client 单测(vitest, mock RTCPeerConnection + fetch + socket):
  - subscribePeer 成功 → onRemoteAudioTrack 上报正确 identity
  - WHEP 失败 → 退避重试调用次数
  - unsubscribePeer → pc close + resource DELETE + onRemoteAudioTrackRemoved
  - member:joined/left → 订阅/取消订阅
  - leaveRoom → 全部 peer 清理

### e2e (手动浏览器)

按 `docs/srs-selfhost-runbook.md` §4:
- 双标签加入同房 → 互听到对方
- 第三人加入 → 前两人各订阅第三人
- 一人离开 → 其他人收到 track removed
- 自身不出现在订阅列表(无回声)

## 残留风险

- **SRS callback 不可达 → 全拒**: fail-closed 设计,部署需保证网络
- **activeStreams 内存态不跨实例**: 多 backend 实例时 callback 路由到不同实例,active 查询不准。单实例 dev/LAN 可接受,多实例需共享状态(Redis)。out of scope
- **stream 泄露给同 room 成员**: member:joined 广播 stream 给同房所有人。同房本应互见,可接受
- **WHEP 偷听**: 知 stream 名 + 突破网络边界即可 play。dev/LAN 网络边界即安全边界,可接受

## 不改

- `Provider` 接口(`token/adminToken/rooms/participants/mute/remove/delete/getHost`)不动
- 现有 JWT token 生成逻辑不动(SRS 仍不校验 JWT,鉴权靠 streamToken + callback)
- LiveKit/MediaSoup/Daily/Agora client 不实现新可选方法
- token API 对非 SRS provider 响应不变(经 interface check,SRS 专属字段仅 SRS provider 注入)
