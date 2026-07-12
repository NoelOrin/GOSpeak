# MediaSoup Provider 完善设计

日期: 2026-07-06
分支: feature/srs-sfu (沿用当前工作分支)
范围: worker / Go bridge+signal+provider / sfu-client / 部署文档

## 背景

MediaSoup 成熟度文档标记"高",但实际存在功能性缺口:

1. **producer-closed 事件从未广播** — `signal.go` 仅在 produce 时广播 `sfu:producer-ready`,参与者离开时无清理广播,导致其它 peer 的远端音轨泄漏(听不到离开、AudioContext/track 不释放)。
2. **无 participant 概念** — worker 仅以 producerId 索引,无 identity↔producer/transport 映射,`ListParticipants`/`RemoveParticipant` 无法实现,provider 返回 `ErrNotSupported`。
3. **服务端 mute 未实现** — 无 producer pause/resume 端点,`MuteParticipant`/`MuteRoomParticipant` 返回 `ErrNotSupported`。
4. **active speaker FIXME** — sfu-client 回退为列出全部远端 identity,非实际检测(`mediasoup-client.ts:350`)。
5. **无 e2e runbook** — SRS 有 `srs-selfhost-runbook.md` 已自部署验证,mediasoup 缺同等 runbook。

## 目标

- 参与者离开时正确广播 `sfu:producer-closed`,远端 peer 清理音轨。
- worker 维护 identity↔producer/transport 映射,支撑 ListParticipants/RemoveParticipant/Mute。
- provider 实现 ListParticipants/RemoveParticipant/MuteParticipant/MuteRoomParticipant。
- active speaker 真实检测(前端 WebAudio)。
- 自部署 e2e 验证 + runbook。

## 非目标(YAGNI)

- restartIce 端点(保持现状,TODO 注释保留)。
- worker→Go 实时推送通道(active speaker 走前端规避)。
- 视频支持(现有仅音频,保持)。
- producer 级 RTP 转发拓扑改造。

## 架构

### 决策

- **participant 模型放 worker**:单一可信源,ListParticipants/RemoveParticipant/Mute 全从这派生。Go 层不另维护索引,避免双写不一致。
- **离开广播走 signal 层 disconnect hook**:Hub OnDisconnect 现有异步清理段调 `removeParticipantSafe`(对 mediasoup 是 generic no-op 路径)。新增 `ParticipantCleanupHandler` 接口,MediasoupSignal 实现它,在 disconnect 时广播 `sfu:producer-closed` + 调 bridge.CloseParticipant 真清理 worker 状态。
- **active speaker 纯前端 WebAudio AnalyserNode**:每个远端 track 已过 AudioContext,加 AnalyserNode 测 RMS,本地挑最响。零后端改动,实时,无需跨进程推送。

### 变更清单

#### 1. worker — `packages/mediasoup-worker/src/worker.ts` + `api.ts`

`RoomState` 新增字段:
```ts
participants: Map<string, {
  sendTransportId?: string;
  recvTransportId?: string;
  producerIds: Set<string>;
}>
```

- `createTransport(roomId, identity, direction)`:参数加 identity/direction,登记到 participant 索引。现有无参签名改为接受可选 identity(向后兼容 produce 路径)。
- `addProducer`:从 `producer.appData.identity` 解析 identity,登记到对应 participant.producerIds。
- producer.observer `close` 回调:从 participant.producerIds 移除(已有 producer map 清理,补 participant 索引清理)。

新增端点(api.ts):
- `GET /rooms/:id/participants` → `[{identity, producerCount, hasSendTransport, hasRecvTransport}]`
- `POST /rooms/:id/participants/:identity/close` → 关该 identity 的 send/recv transport(级联关 producers)+ 从 participants 删除。返回 `{closedProducerIds: [...]}`(供调试,广播由 Go 触发)。
- `POST /rooms/:id/producers/:producerId/pause` → `producer.pause()`
- `POST /rooms/:id/producers/:producerId/resume` → `producer.resume()`
- `POST /rooms/:id/participants/:identity/pause` → 批量 pause 该 identity 所有 producer
- `POST /rooms/:id/participants/:identity/resume` → 批量 resume

identity 未找到返回 404。transport/producer 不存在返回 404。

#### 2. Go bridge — `app/server/internal/mediasoup/bridge.go`

新增类型:
```go
type ParticipantInfo struct {
    Identity         string `json:"identity"`
    ProducerCount    int    `json:"producerCount"`
    HasSendTransport bool   `json:"hasSendTransport"`
    HasRecvTransport bool   `json:"hasRecvTransport"`
}
```

新增方法:
- `ListParticipants(roomID) ([]ParticipantInfo, error)`
- `CloseParticipant(roomID, identity) ([]string, error)` — 返回关闭的 producerIds
- `PauseProducer(roomID, producerID) error`
- `ResumeProducer(roomID, producerID) error`
- `PauseParticipant(roomID, identity) error`
- `ResumeParticipant(roomID, identity) error`

均走现有 `do()` helper。`CreateTransport` 签名扩展:接受 identity+direction,query/body 传给 worker。

#### 3. Go signal — `app/server/internal/mediasoup/signal.go` + `app/server/internal/signal/hub.go`

signal 包新增接口:
```go
type ParticipantCleanupHandler interface {
    OnParticipantLeft(room, identity string)
}
```

MediasoupSignal:
- 加字段 `socketIndex sync.Map` — key socketID, value `{room, identity}`。
- produce handler 成功后:`socketIndex.Store(s.ID(), {req.Room, appData.identity})`。
- 新方法 `OnParticipantLeft(room, identity string)`:
  - 广播 `sfu:producer-closed {room, identity}`(经现有 broadcast fn)。
  - `bridge.CloseParticipant(room, identity)` 异步执行(不阻塞广播)。
- 注册 `sfu:close-transport` 事件(可选,前端 leaveRoom 显式调用):body `{room, identity}` → 同 OnParticipantLeft 逻辑。前端 socket disconnect 兜底走 Hub OnDisconnect 路径。

Hub OnDisconnect 异步清理段(goroutine 内),在现有 `removeParticipantSafe` 之后:
```go
if h, ok := h.sfuSignalHandler.(signal.ParticipantCleanupHandler); ok {
    for _, c := range cleanups {
        h.OnParticipantLeft(c.room, c.identity)
    }
}
```
注:`sfuSignalHandler` 字段类型需暴露给 Hub(已存在 `SetSFUSignalHandler`)。Hub 持有 `participantCleanupHandler` 字段,SetSFUSignalHandler 时类型断言赋值。

#### 4. Go provider — `app/server/internal/mediasoup/provider.go`

替换 notSupported 实现:
- `ListParticipants(room)` → `bridge.ListParticipants(room)`,返回 `[]ParticipantInfo`。
- `RemoveParticipant(room, identity)` → `bridge.CloseParticipant(room, identity)`,忽略返回的 producerIds(广播由 disconnect 路径触发;直接调用时producer close 由 worker observer 触发,但无广播——可接受,RemoveParticipant 是管理端 API,被踢 peer 的 socket 通常已断或会断)。
- `MuteParticipant(room, identity, trackSid, muted)`:
  - trackSid 非空 → 当 producerId:`bridge.Pause/ResumeProducer(room, trackSid)`。
  - trackSid 空 → `bridge.Pause/ResumeParticipant(room, identity)`(批量)。
- `MuteRoomParticipant(room, identity, muted)` → `bridge.Pause/ResumeParticipant(room, identity)`。

#### 5. 前端 active speaker — `packages/sfu-client/src/mediasoup-client.ts`

`MediaSoupRemoteAudioTrack`:
- 构造加 `analyser: AnalyserNode`(从现有 audioContext 创建),source→analyser→gainNode。
- 新增 `getLevel(): number` — 读 `analyser.getByteTimeDomainData`,算 RMS,归一化 0..1。

MediaSoupSFUClient:
- 字段 `activeSpeakerTimer: ReturnType<typeof setInterval> | null`。
- `joinRoom` 末尾启动 `setInterval(500)`:扫 remoteTracks.getLevel(),挑最大(>阈值 0.01)identity,调 `onActiveSpeakersCb([loudest])`。全部低于阈值 → `[]`。
- `leaveRoom` 清 timer。
- `consumeProducer`:删除 line 350 FIXME 回退行,active speaker 改由 timer 驱动。
- `createSendTransport`/`createRecvTransport`:`sfuEmit(CREATE_TRANSPORT, ...)` payload 加 `identity: this.identity`,配合 worker participant 索引登记。

#### 6. e2e runbook + 部署

新建 `docs/mediasoup-selfhost-runbook.md`,仿 `srs-selfhost-runbook.md` 结构:
1. 起 mediasoup-worker(docker compose 取消注释块 + ANNOUNCED_IP 说明)。
2. 后端切 mediasoup(`.env.dev` 设 `SFU_PROVIDER=mediasoup`,`MEDIASOUP_BRIDGE_URL`)。
3. 前端切 mediasoup(`VITE_SFU_PROVIDER=mediasoup`)。
4. 双向音频验证步骤(两浏览器 tab)。
5. mac UDP 异常注意(commit 998df91 已记,runbook 提示)。

`deploy/docker-compose.example.yml`:mediasoup-worker 注释块补 `ANNOUNCED_IP` env 说明(LAN 部署需设宿主 IP)。

## 数据流

### 参与者离开清理
```
peer socket disconnect
  → Hub.OnDisconnect
    → (sync) room.Members 删除,广播 EventMemberLeft
    → (async goroutine)
        removeParticipantSafe (generic,mediasoup 路径 no-op)
        if participantCleanupHandler:
          MediasoupSignal.OnParticipantLeft(room, identity)
            → broadcast sfu:producer-closed {room, identity}
            → bridge.CloseParticipant(room, identity) [async]
                → worker 关 transports/producers + 清 participant 索引
  → 其余 peer 的 sfu-client onProducerClosedBound(info)
    → info.identity 匹配 → track.stop() + remoteTracks.delete
    → onRemoteAudioTrackRemovedCb(identity)
```

### 服务端 mute
```
admin POST /api/v1/.../mute
  → MuteService → sfuProvider.MuteParticipant(room, identity, trackSid, muted)
    → bridge.Pause/ResumeProducer or Pause/ResumeParticipant
      → worker producer.pause()/resume()
  → (mediasoup 路径不广播;远端 consumer 静默,因 producer pause 导致 RTP 停止)
```

注:mediasoup producer pause 后,已 consume 的 consumer 收不到 RTP,前端 track 静默。无需额外广播事件。

## 错误处理

- bridge HTTP 错误沿用 `do()` 现有格式,status 非 2xx 返回 error。
- provider 包装为 `pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, ...)`。
- worker identity 未找到 → 404 → bridge 返回 error → provider 返回 AppError(SFU_ERROR)。RemoveParticipant 对不存在的 identity 静默成功(幂等,与 LiveKit 行为对齐)——bridge CloseParticipant 对 404 转为 nil error。

## 测试

- **Go bridge**:httptest 起 mock worker server,断言各端点请求路径/body/响应解析。
- **Go signal**:socketIndex 登记/清理单测;OnParticipantLeft 广播 + CloseParticipant 调用断言(mock bridge)。
- **Go provider**:各方法委托 bridge 调用断言(mock bridge)。
- **worker**:无单测框架,手动 e2e。
- **前端 active speaker**:手动 e2e(发声 tab 被标 active)。
- **e2e**:runbook 跑通即验收。

## 影响面

- 修改:worker.ts/api.ts,bridge.go,signal.go,provider.go,hub.go,mediasoup-client.ts,docker-compose.example.yml。
- 新增:docs/mediasoup-selfhost-runbook.md,signal.ParticipantCleanupHandler 接口。
- 不影响其它 SFU provider(接口未变,仅 mediasoup 实现补全)。
- 不影响用户禁言层(MuteService/Hub.BroadcastMute 独立,不变)。

## 验收标准

1. 参与者 A 离开,B 的 `onRemoteAudioTrackRemovedCb` 被调用,A 音轨停止播放。
2. `GET /api/v1/sfu/rooms/:id/participants`(mediasoup)返回参与者列表。
3. 服务端 mute A,B 听不到 A;unmute 恢复。
4. 发声的远端 peer 出现在 `onActiveSpeakersCb`,无人发声返回 `[]`。
5. mediasoup-selfhost-runbook 步骤可复现,双向音频通。
