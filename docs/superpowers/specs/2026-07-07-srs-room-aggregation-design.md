# SRS Room 聚合设计

**日期**: 2026-07-07
**状态**: 设计
**范围**: `internal/srs` + `internal/signal` + `internal/sfu` + `server/gin.go`

## 背景

SRS SFU 后端无"房间"概念,只有 stream。每个用户推流生成独立 stream `gs-<hash(room:identity)>`。当前 `srs.Service.ListRooms` 把 SRS `/api/v1/streams` 返回的 stream 名当 room 返,导致同一 room 下 N 用户 → 上层看到 N 个假房间;`DeleteRoom(room)` 打 `DELETE /api/v1/streams/{room}` 永远失败(SRS5 实测返 2048 不支持)。

P0 已修:`RemoveParticipant` 端点修正(`/clients/{id}`)、`ListParticipants` 实真、secret 强制、HTTP 超时、srs.conf 回归。本设计解决剩余 P1:room 维度聚合。

## 目标

- `ListRooms` 返真实 room 列表(基于 WS join 的 room,非 stream 名)
- `DeleteRoom(room)` 能真删该 room 下所有 SRS 推流(通过 kick client)
- 不引入循环 import,遵循现有 Hub setter 注入模式(`SetSFU`/`SetStreamResolver`)
- 单测可独立验证聚合逻辑(无需起 SRS)

## 非目标

- `ListParticipants` 已 P0 实真,不再改
- `RemoveParticipant` 已 P0 修正,不再改
- 持久化 room→streams 映射(当前纯内存,重启丢失,可接受)
- Redis 重启恢复(当前 Hub 全内存,一致)

## 架构

### 数据流

```
WS join (hub.go:OnRoomSfuConfirmed)
  └─ streamResolver.StreamName(room, identity) → stream
     ├─ rooms[room].Members[sid].Stream = stream   (已有)
     ├─ streamRoomCache[stream] = room              (新,正向反查)
     └─ roomStreams[room][stream] = {}              (新,聚合索引)

SRS on_publish callback (srs_callback_handler.go)
  └─ stream 反查 streamRoomCache → room
     └─ activeStreams[stream] = {}                  (已有)
     └─ 若 room 存在: roomStreams[room][stream] 确认存活

SRS on_unpublish callback
  └─ activeStreams delete stream                    (已有)
  └─ room: roomStreams[room] delete stream          (新)

WS leave (OnRoomLeave / OnDisconnect)
  └─ rooms[room].Members delete sid
     └─ streamRoomCache delete stream               (新)
     └─ roomStreams[room] delete stream             (新)
     └─ roomStreams[room] 空 → delete room          (新)

ListRooms (SRS Service)
  └─ roomRegistry.Rooms() → Hub.roomStreams keys

DeleteRoom(room) (SRS Service)
  └─ roomRegistry.Streams(room) → stream 集
  └─ client.ListParticipants-like: GET /api/v1/clients/ 过滤 stream ∈ 集 → client_id 集
  └─ 逐条 DELETE /api/v1/clients/{client_id}
  └─ roomRegistry 清 roomStreams[room]
```

### 接口定义

放 `internal/sfu` 包避循环(signal→sfu 已存在,srs→sfu 已存在,无新环):

```go
// RoomRegistry 提供 room→streams 聚合视图,SRS 等无原生 room 维度的 provider 使用。
// Hub 实现此接口。
type RoomRegistry interface {
    Rooms() []string
    Streams(room string) []string
    ClearRoom(room string)
}
```

Hub 实现(纯内存读,无锁泄漏):

```go
func (h *Hub) Rooms() []string
func (h *Hub) Streams(room string) []string
func (h *Hub) ClearRoom(room string)
```

## 组件改动

### 1. `internal/sfu/room_registry.go`(新)

定义 `RoomRegistry` 接口。

### 2. `internal/signal/hub.go`

- Hub 加字段:`roomStreams map[string]map[string]struct{}`、`streamRoomCache map[string]string`
- `NewHub` 初始化两 map
- `OnRoomSfuConfirmed`(join,~line 349):算 stream 后填 `streamRoomCache[stream]=room` + `roomStreams[room][stream]`
- `OnRoomLeave` + `OnDisconnect`:删 `streamRoomCache` + `roomStreams` 对应 stream;room 空删 room
- `RegisterStream`(on_publish,~line 769):stream 反查 `streamRoomCache` → room;若命中,确认 `roomStreams[room][stream]`
- `UnregisterStream`(on_unpublish):删 `activeStreams` + `roomStreams[room][stream]`
- 新增 `Rooms()`/`Streams(room)`/`ClearRoom(room)` 三个公开方法(实现 `sfu.RoomRegistry`)

注意:`roomStreams` 与 `activeStreams` 职责不同 —— 前者跟 WS 成员预期 stream,后者跟 SRS 实际存活 stream。`ListRooms` 用 `roomStreams`(room 维度权威),`DeleteRoom` kick 时用 `activeStreams` 交叉确认存活。

### 3. `internal/srs/provider.go`

- `Service` 加字段 `registry sfu.RoomRegistry`(nil 可,降级到旧 SRS API 行为)
- `NewService` 不变(registry 后注入)
- 新增 `SetRoomRegistry(r sfu.RoomRegistry)`
- `ListRooms`:registry 非 nil → `registry.Rooms()`;nil → 旧 `client.ListRooms()`(降级)
- `DeleteRoom(room)`:registry 非 nil → kick 流程;nil → 旧 `client.DeleteRoom` (降级,已知失败但保留)

kick 流程新增 `client.KickByStreams(streams []string) error`:
```
GET /api/v1/clients/ → 过滤 Stream ∈ streams → 收集 client id
逐条 DELETE /api/v1/clients/{id}
```
返 nil 若全 kick 成功;部分失败返聚合 error(已删数 + 剩余数)。

### 4. `internal/sfu/dynamic_provider.go`

- `DynamicProvider` 加 `roomRegistry sfu.RoomRegistry` 字段
- 新增 `SetRoomRegistry(r)`:存字段
- 各方法调 `current()` 得 provider 后,若 provider 实现可选接口 `interface{ SetRoomRegistry(sfu.RoomRegistry) }`,转发

实际上 DynamicProvider 每次 `current()` 重新 `NewProvider(cfg)`(见 `dynamic_provider.go:138`),所以 SetRoomRegistry 必须在每次重建后重放。改 `current()` 或在 setter 后缓存。

**问题**:`DynamicProvider.current()` 每次 new,setter 注入的 registry 会丢。需 DynamicProvider 持 registry,在 `current()` 返回 provider 后立即转发。改 `current()`:
```go
func (p *DynamicProvider) current() (Provider, error) {
    cfg, err := p.resolve()
    ...
    provider, err := NewProvider(cfg)
    ...
    if p.roomRegistry != nil {
        if rs, ok := provider.(interface{ SetRoomRegistry(sfu.RoomRegistry) }); ok {
            rs.SetRoomRegistry(p.roomRegistry)
        }
    }
    return provider, nil
}
```
注:`current()` 现每次调都 new provider,性能本就差(每次 SFU 调用 new 一次)。registry 转发随之,无额外开销量级。

### 5. `server/gin.go`

`signalHub` 建(line 115)后:
```go
if rs, ok := sfuProvider.(interface{ SetRoomRegistry(sfu.RoomRegistry) }); ok {
    rs.SetRoomRegistry(signalHub)
}
```
注入点跟 `SetStreamResolver`(line 117-119)对称。

## 错误处理

- `registry` nil(provider 未注入或非 SRS)→ ListRooms/DeleteRoom 降级旧行为,不崩
- `streamRoomCache` 反查 miss(SRS 流先于 WS join 到达,理论不该发生但防御)→ on_publish 仅登记 `activeStreams`,不动 `roomStreams`(等 WS join 补)
- kick 部分失败 → 返 error 含 `kicked=%d remaining=%d`,DeleteRoom 不静默成功
- `client.ListParticipants` 过滤 stream 时 stream 为空(SRS 老版本字段缺失)→ 返空,DeleteRoom 报 `no participants to kick`

## 测试

### `internal/signal/hub_test.go` 扩展

- `TestHub_Rooms_JoinLeave`:join 后 `Rooms()` 含 room,`Streams(room)` 含 stream;leave 后空删
- `TestHub_RegisterStream_ReverseLookup`:join 设 cache → `RegisterStream(stream)` 后 `roomStreams[room]` 含 stream
- `TestHub_UnregisterStream_Clears`:on_unpublish 删 roomStreams 条目
- `TestHub_Disconnect_UpdatesMapping`:OnDisconnect 清成员 stream 映射
- `TestHub_Rooms_NoRegistry`:空 Hub `Rooms()` 返空切片非 nil

### `internal/srs/provider_test.go`(新)

用 `httptest.Server` mock SRS:
- `TestListRooms_WithRegistry`:registry 返 `["room-1"]`,Service.ListRooms 返之,不打 HTTP
- `TestListRooms_NoRegistry_Fallback`:registry nil → 走 httptest `/api/v1/streams`
- `TestDeleteRoom_KickByStreams`:httptest 返 clients 含匹配 stream → DELETE /clients/{id} 被调
- `TestDeleteRoom_PartialFailure`:一条 kick 失败 → error 含 `remaining=1`
- `TestDeleteRoom_NoParticipants`:clients 空 → error `no participants`

### `internal/srs/client_test.go`(新或并入)

- `TestClient_KickByStreams`:httptest mock,验 client id 收集 + DELETE 路径

## 风险

1. **WS 与 SRS 不同步**:WS leave 后 SRS 仍推流(on_unpublish 滞后)→ `roomStreams` 已删但 `activeStreams` 还在。`DeleteRoom` 用 `Streams(room)` 可能漏杀滞留流。**缓解**:`DeleteRoom` kick 时同时扫 `activeStreams` 中以 room 前缀...无前缀。改:`DeleteRoom` kick 前先 `GET /api/v1/clients/` 全量,过滤 stream ∈ `Streams(room)` ∪ `activeStreams` 中反查命中的。简单版:只用 `Streams(room)`,滞留流留下一轮 on_unpublish 清。接受。

2. **DynamicProvider.current() 每次 new**:registry 转发每次,但已是现状,无新增量级问题。

3. **并发**:`roomStreams`/`streamRoomCache` 与 `activeStreams` 同 `h.mu` 保护,复用现有锁。

4. **stream→room 缓存 miss 时 on_publish**:见错误处理,防御性跳过 roomStreams。

## 实现顺序

1. `sfu/room_registry.go` 接口
2. `signal/hub.go` 两 map + join/leave/callback 同步 + 三公开方法
3. `hub_test.go` 扩展
4. `srs/client.go` `KickByStreams`
5. `srs/provider.go` registry 字段 + ListRooms/DeleteRoom 分支 + SetRoomRegistry
6. `srs/provider_test.go` httptest
7. `sfu/dynamic_provider.go` SetRoomRegistry 转发
8. `server/gin.go` 注入
9. 全量 `go build` + `go test` + docker SRS e2e 手验

## 验收

- `go test ./internal/srs/... ./internal/signal/... ./internal/handler/...` 全过
- docker SRS 起,WS join 两用户同 room → `GET /signal/rooms` 返 1 个 room 非 2 个 stream
- `DELETE` room → SRS clients 清空
- SRS 模式下 ListRooms 返真实 room 数