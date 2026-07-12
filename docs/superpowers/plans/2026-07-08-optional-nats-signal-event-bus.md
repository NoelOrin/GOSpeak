# 可选的 NATS 信号事件总线实现计划（精炼版）

> **对 agent 工作者：** 使用 `superpowers:subagent-driven-development` 或 `superpowers:executing-plans` 按任务单元实施。步骤使用 `- [ ]` 语法跟踪。

## 架构概览

```
┌─────────────────────────────────────────────────┐
│                      Hub                        │
│  ┌───────────────────────────────────────────┐  │
│  │  EventBus (interface)                     │  │
│  │  • PublishNamespace(event, payload)       │  │
│  │  • PublishRoom(room, event, payload)      │  │
│  └──────────┬────────────────────────────────┘  │
└─────────────┼────────────────────────────────────┘
              │
   ┌──────────┴──────────┐
   │                     │
   ▼                     ▼
LocalEventBus      NATSEventBus
 (直接 socketio)    (本地投递 + NATS
                     pub/sub + 去重)
```

**目标：** 为 GOSpeak 引入可选 NATS 信号事件总线，单节点不变，多实例跨进程复制 `room:*`、`member:*`、`user:*` 广播。

**不纳入范围：**
- ❌ 房间成员状态 (`Hub.rooms`) — 保持进程内
- ❌ 禁言状态 — 保持进程内
- ❌ SRS 流注册表 (`activeStreams`, `roomStreams`) — 保持进程内
- ❌ SFU 提供者状态协调 — 保持进程内
- ❌ 分布式状态快照/同步

**技术栈：** Go 1.26, Gin, go-socket.io v1.7.0, NATS Go 客户端, Docker Compose。

---

## 降级机制（精炼核心）

### 对齐 Redis 模块设计模式

降级机制完全遵循 `internal/redis/` 模块的既有模式，不做额外创新：

| Redis 模块 | NATS 事件总线 | 说明 |
|------------|--------------|------|
| `var Client *redis.Client` | `var globalBus EventBus` | 全局单例，nil 表示未连接 |
| `InitRedis()` | `InitEventBus(cfg, sio)` | 启动时条件初始化 |
| `IsConnected() bool` | `IsEventBusConnected() bool` | 消费者检查 |
| `RedisStats{Connected}` | `EventBusStats{Mode, NATSConnected}` | 监控面板集成 |
| 操作中 `if Client == nil { return }` | 广播中 `if globalBus == nil { return }` | 调用方无感知降级 |
| 连接失败 `log + return` | 连接失败 `log + 使用 LocalEventBus` | 永不 panic |

### 降级状态机

```
┌──────────┐    InitEventBus     ┌──────────┐
│ 启动     │ ─────────────────→ │ NATS_URL │
└──────────┘                    └────┬─────┘
                                     │
                    ┌────────────────┴────────────────┐
                    │                                 │
                    ▼                                 ▼
           ┌────────────────┐              ┌──────────────────────┐
           │ 为空            │              │ 已设置               │
           │ LocalEventBus   │              │ nats.Connect(url)    │
           │ mode=local      │              └──────────┬───────────┘
           └────────────────┘                          │
                                           ┌──────────┴──────────┐
                                           │                     │
                                           ▼                     ▼
                                  ┌────────────────┐   ┌────────────────────┐
                                  │ 连接成功        │   │ 连接失败           │
                                  │ NATSEventBus    │   │ log.Warn           │
                                  │ mode=nats       │   │ LocalEventBus      │
                                  │ connected=true  │   │ mode=local(fallback│
                                  └────────────────┘   └────────────────────┘
```

### 运行时状态分类

| 状态 | 本地投递 | NATS 发布 | 对等实例事件 |
|------|---------|-----------|-------------|
| `NATS_URL=""` | ✅ | — | — |
| 连接失败/factory | ✅ | — | — |
| 连接成功、运行中 | ✅ | ✅ | ✅ 接收 |
| NATS 断连（运行时） | ✅ | ❌ 跳过 | ❌ |
| NATS 重连（内置） | ✅ | ✅ 恢复 | ✅ 恢复 |

### 行为要点对照 Redis 模式

1. **一次初始化，终身不变** — `InitEventBus` 在启动时决定总线类型，不支持运行时切换（同 Redis）。
2. **连接失败静默降级** — NATS 连接失败或订阅失败时，降级到 `LocalEventBus`，只打 log 不 panic（同 `InitRedis` 连接失败只 return）。
3. **全局状态暴露** — `globalBus` 非 nil 即可安全调用（同 `Client` 非 nil 即可安全操作）。
4. **监控可见性** — `EventBusStats()` 暴露给 `MonitorHandler` 的 SSE 端点（同 `redis.GetStats()`）。
5. **运行时 NATS 断连** — NATS 内置自动重连；断连期间 `Publish` 到 NATS 失败静默忽略，本地投递不受影响；重连后订阅自动恢复。

### 启动日志格式（严格对齐 Redis）

```go
// 未配置
fmt.Println("[EventBus] NATS_URL not set, using local event bus")

// 连接失败
fmt.Printf("[EventBus] NATS connection failed (falling back to local): %v\n", err)

// 订阅失败
fmt.Printf("[EventBus] NATS subscribe failed (falling back to local): %v\n", err)

// 连接成功
fmt.Printf("[EventBus] NATS connected (url=%s), using replicated event bus\n", url)
```

---

## 关键设计决策

### D1. EventBus 接口

```go
type EventBus interface {
    PublishNamespace(event string, payload interface{}) error
    PublishRoom(room, event string, payload interface{}) error
}
```

- 不包含 `ForEach` — 那是 Hub 内部成员遍历操作，事件总线只负责扇出。
- `payload` 序列化由总线自身处理，调用方传入 `map[string]interface{}` 等 JSON 友好类型。
- 在 `PublishRoom` 时，NATSEventBus 会先调用 `local.PublishRoom`（本实例投递），再发布到 NATS 供对等实例接收后重放。

### D2. NATS 信封格式

```go
type EventEnvelope struct {
    InstanceID string      `json:"instanceId"`
    Event      string      `json:"event"`
    Room       string      `json:"room"`         // 非空表示 room 级别事件
    Payload    interface{} `json:"payload"`
}
```

- `room` 空字符串 → 接收方调用 `BroadcastToNamespace`
- `room` 非空 → 接收方调用 `BroadcastToRoom`

### D3. 自身消息去重

订阅回调中校验 `envelope.InstanceID == bus.instanceID`，相等则跳过（已由本地投递分支处理过）。

### D4. NATS Subject 布局

```
{prefix}.signal.namespace       — namespace 级别事件扇出
{prefix}.signal.room.{room}     — 按 room 拆分，减少不必要接收
```

---

## 任务分解

### 任务 1：定义 EventBus 接口 + EventEnvelope 数据结构

**文件：**
- 创建 `app/server/internal/signal/event_bus.go`

```go
package signal

// EventBus 抽象信号事件的扇出目标。
type EventBus interface {
    PublishNamespace(event string, payload interface{}) error
    PublishRoom(room, event string, payload interface{}) error
}

// EventEnvelope 是跨实例 NATS 消息的 JSON 帧格式。
type EventEnvelope struct {
    InstanceID string      `json:"instanceId"`
    Event      string      `json:"event"`
    Room       string      `json:"room"`
    Payload    interface{} `json:"payload"`
}
```

- [ ] **步骤 1**：创建 `event_bus.go`
- [ ] **步骤 2**：`go build ./internal/signal` 确认无编译错误

---

### 任务 2：实现 LocalEventBus + 测试

**文件：**
- 创建 `app/server/internal/signal/local_event_bus.go`
- 创建 `app/server/internal/signal/local_event_bus_test.go`

```go
package signal

import socketio "github.com/googollee/go-socket.io"

type LocalEventBus struct {
    server *socketio.Server
}

func NewLocalEventBus(server *socketio.Server) *LocalEventBus {
    return &LocalEventBus{server: server}
}

func (b *LocalEventBus) PublishNamespace(event string, payload interface{}) error {
    if b.server == nil {
        return fmt.Errorf("local event bus: server is nil")
    }
    b.server.BroadcastToNamespace("/", event, payload)
    return nil
}

func (b *LocalEventBus) PublishRoom(room, event string, payload interface{}) error {
    if b.server == nil {
        return fmt.Errorf("local event bus: server is nil")
    }
    b.server.BroadcastToRoom("/", room, event, payload)
    return nil
}
```

**测试（使用 hub_test.go 中已有的 `mockServer`）：**
- `TestLocalEventBus_PublishNamespace` — 验证 BroadcastToNamespace 被调用且 payload 正确
- `TestLocalEventBus_PublishRoom` — 验证 BroadcastToRoom 被调用且 payload 正确
- `TestLocalEventBus_NilServer_ReturnsError` — server 为 nil 时返回错误

- [ ] **步骤 1**：编写测试（先失败）
- [ ] **步骤 2**：实现 `LocalEventBus`
- [ ] **步骤 3**：运行测试确认通过

---

### 任务 3：实现 NATSEventBus + 降级回路

**文件：**
- 创建 `app/server/internal/signal/nats_event_bus.go`
- 创建 `app/server/internal/signal/nats_event_bus_test.go`

**NATSEventBus 结构：**

```go
type NATSEventBus struct {
    local         EventBus       // 实际类型是 *LocalEventBus
    conn          Connector
    instanceID    string
    subjectPrefix string
    subscribed    bool
    mu            sync.Mutex
    stopCh        chan struct{}
}

// Connector 抽象 nats.Conn，方便测试替身注入。
type Connector interface {
    Publish(subject string, data []byte) error
    Subscribe(subject string, cb func(msg []byte)) error
    Close() error
    IsConnected() bool
}
```

**Publish 流程（降级关键路径）：**

```go
func (b *NATSEventBus) PublishNamespace(event string, payload interface{}) error {
    // 1. 总在本地投递
    if err := b.local.PublishNamespace(event, payload); err != nil {
        return err
    }
    // 2. NATS 扇出：断连时静默跳过（降级）
    if !b.conn.IsConnected() {
        return nil
    }
    envelope := EventEnvelope{
        InstanceID: b.instanceID,
        Event:      event,
        Payload:    payload,
    }
    data, _ := json.Marshal(envelope)
    return b.conn.Publish(b.subjectPrefix+".signal.namespace", data)
}

func (b *NATSEventBus) PublishRoom(room, event string, payload interface{}) error {
    if err := b.local.PublishRoom(room, event, payload); err != nil {
        return err
    }
    if !b.conn.IsConnected() {
        return nil
    }
    envelope := EventEnvelope{
        InstanceID: b.instanceID,
        Event:      event,
        Room:       room,
        Payload:    payload,
    }
    data, _ := json.Marshal(envelope)
    return b.conn.Publish(b.subjectPrefix+".signal.room."+room, data)
}
```

**订阅回调（自消息去重）：**

```go
func (b *NATSEventBus) startSubscriptions() error {
    // namespace subject
    if err := b.conn.Subscribe(b.subjectPrefix+".signal.namespace", b.handleMessage); err != nil {
        return err
    }
    // room wildcard: 接收所有 room 级别事件
    if err := b.conn.Subscribe(b.subjectPrefix+".signal.room.>", b.handleMessage); err != nil {
        return err
    }
    return nil
}

func (b *NATSEventBus) handleMessage(data []byte) {
    var envelope EventEnvelope
    if err := json.Unmarshal(data, &envelope); err != nil {
        return // 格式错误静默丢弃
    }
    // 自消息跳过（已由本地投递）
    if envelope.InstanceID == b.instanceID {
        return
    }
    if envelope.Room == "" {
        b.local.PublishNamespace(envelope.Event, envelope.Payload)
    } else {
        b.local.PublishRoom(envelope.Room, envelope.Event, envelope.Payload)
    }
}
```

**测试验证（含降级路径）：**

| 测试用例 | 验证点 |
|---------|--------|
| `TestNATSEventBus_EncodeDecode` | 信封 JSON 序列化/反序列化 |
| `TestNATSEventBus_PublishNamespace` | 本地投递 + NATS 发布到 namespace subject |
| `TestNATSEventBus_PublishRoom` | 本地投递 + NATS 发布到 room subject |
| `TestNATSEventBus_PublishNamespace_Disconnected` | **降级**：conn.IsConnected()=false → 仅本地投递，NATS 发布跳过 |
| `TestNATSEventBus_SubscriptionCallback_SkipsSelf` | 自身消息跳过去重 |
| `TestNATSEventBus_SubscriptionCallback_DeliversRemote` | 远程消息投递到本地总线 |
| `TestNATSEventBus_Start_SubscribeFails` | **降级**：Subscribe 返回错误时 Start 返回 error |

**fakeConnector（测试替身）：**

```go
type fakeConnector struct {
    published      []messageRecord
    subscribeCB    func(msg []byte)
    connected      bool
    subscribeErr   error
    publishErr     error
}

type messageRecord struct {
    subject string
    data    []byte
}
```

- [ ] **步骤 1**：编写 `nats_event_bus_test.go`
- [ ] **步骤 2**：实现 `nats_event_bus.go`
- [ ] **步骤 3**：运行测试确认通过，含降级路径

---

### 任务 4：实现初始化 + 工厂函数（对齐 Redis 模式）

**文件：**
- 创建 `app/server/internal/signal/bus.go`（类似 `redis/redis.go`：全局状态 + Init + Stats）
- 创建 `app/server/internal/signal/bus_test.go`
- 创建 `app/server/internal/signal/nats_client.go`（`NATSConn` 真实 NATS 实现）
- 修改 `app/server/internal/config/config.go`

**bus.go — 全局入口（对齐 `redis/redis.go`）：**

```go
package signal

import (
    "encoding/json"
    "fmt"
    "log"

    "GOSpeak/internal/config"
    "github.com/google/uuid"
    "github.com/nats-io/nats.go"
    socketio "github.com/googollee/go-socket.io"
)

// globalBus 全局事件总线实例。NATS 未配置或连接失败时为 LocalEventBus。
var globalBus EventBus

// InitEventBus 根据配置初始化全局事件总线。
// 与 internal/redis.InitRedis 相同的设计模式：条件初始化 + 日志 + 静默降级。
// 在 server.StartGin 的 sioServer 创建之后、hub.SetupRoutes 之前调用。
func InitEventBus(cfg *config.Config, s *socketio.Server) (close func() error) {
    local := NewLocalEventBus(s)
    url := cfg.NATSURL
    if url == "" {
        log.Println("[EventBus] NATS_URL not set, using local event bus")
        globalBus = local
        return func() error { return nil }
    }

    nc, err := nats.Connect(url)
    if err != nil {
        log.Printf("[EventBus] NATS connection failed (falling back to local): %v", err)
        globalBus = local
        return func() error { return nil }
    }

    prefix := cfg.NATSSubjectPrefix
    if prefix == "" {
        prefix = "gospeak"
    }

    bus := NewNATSEventBus(local, &NATSConn{conn: nc}, uuid.NewString(), prefix)
    if err := bus.Start(); err != nil {
        nc.Close()
        log.Printf("[EventBus] NATS subscribe failed (falling back to local): %v", err)
        globalBus = local
        return func() error { return nil }
    }

    log.Printf("[EventBus] NATS connected (url=%s), using replicated event bus", url)
    globalBus = bus
    return func() error {
        bus.Stop()
        if err := nc.Drain(); err != nil {
            log.Printf("[EventBus] NATS drain error: %v", err)
        }
        nc.Close()
        return nil
    }
}

// EventBusInstance 返回当前全局事件总线，供 Hub 获取。
func EventBusInstance() EventBus {
    return globalBus
}

// EventBusStats 事件总线状态（对齐 redis.RedisStats），供 MonitorHandler 集成。
type EventBusStats struct {
    Mode          string `json:"mode"`          // "local" | "nats"
    NATSConnected bool   `json:"natsConnected"` // NATS 当前是否连接
}

// EventBusStats 返回当前总线状态。
func GetEventBusStats() EventBusStats {
    if globalBus == nil {
        return EventBusStats{Mode: "local"}
    }
    if nb, ok := globalBus.(*NATSEventBus); ok {
        return EventBusStats{
            Mode:          "nats",
            NATSConnected: nb.IsConnected(),
        }
    }
    return EventBusStats{Mode: "local"}
}
```

**config.go 新增：**

```go
// Config 新增字段
NATSURL            string
NATSSubjectPrefix  string

// Load 新增
NATSURL:            getEnv("NATS_URL", ""),
NATSSubjectPrefix:  getEnv("NATS_SUBJECT_PREFIX", "gospeak"),
```

**nats_client.go：**

```go
package signal

import "github.com/nats-io/nats.go"

type NATSConn struct {
    conn *nats.Conn
}

func (nc *NATSConn) Publish(subject string, data []byte) error {
    return nc.conn.Publish(subject, data)
}

func (nc *NATSConn) Subscribe(subject string, cb func(msg []byte)) error {
    _, err := nc.conn.Subscribe(subject, func(m *nats.Msg) {
        cb(m.Data)
    })
    return err
}

func (nc *NATSConn) Close() error {
    nc.conn.Close()
    return nil
}

func (nc *NATSConn) IsConnected() bool {
    return nc.conn.IsConnected()
}
```

**测试验证：**

| 测试用例 | 验证点 |
|---------|--------|
| `TestInitEventBus_NoURL` | `NATS_URL=""` → globalBus 为 `*LocalEventBus`，close=nil |
| `TestInitEventBus_ConnectionFails` | `NATS_URL` 设置 + fake borked connector → **降级** log + globalBus 为 `*LocalEventBus` |
| `TestInitEventBus_SubscribeFails` | `NATS_URL` 设置 + fake subscribe 返回 error → **降级** log + globalBus 为 `*LocalEventBus` |
| `TestInitEventBus_Success` | `NATS_URL` 设置 + fake connector 成功 → globalBus 为 `*NATSEventBus` |
| `TestGetEventBusStats_Local` | 未配置时 stats.Mode="local", stats.NATSConnected=false |
| `TestGetEventBusStats_NATS` | NATS 模式下 stats.Mode="nats" |

- [ ] **步骤 1**：编写 `bus_test.go`
- [ ] **步骤 2**：实现 `bus.go`
- [ ] **步骤 3**：实现 `nats_client.go`
- [ ] **步骤 4**：更新 `config.go`
- [ ] **步骤 5**：`go mod tidy`
- [ ] **步骤 6**：运行测试确认通过，含所有降级路径

---

### 任务 5：重构 Hub 接入 EventBus（保留惰性回退）

**文件：**
- 修改 `app/server/internal/signal/hub.go`
- 创建 `app/server/internal/signal/hub_event_bus_test.go`

**Hub 变更：**

```go
type Hub struct {
    // 保留原有字段
    eventBus EventBus  // 新增
    // ...
}

func (h *Hub) SetEventBus(bus EventBus) {
    h.eventBus = bus
}

// EventBus 返回当前事件总线，未显式设置时从全局实例获取（惰性回退）。
func (h *Hub) EventBus() EventBus {
    if h.eventBus != nil {
        return h.eventBus
    }
    if bus := EventBusInstance(); bus != nil {
        return bus
    }
    if h.server != nil {
        return NewLocalEventBus(h.server)
    }
    return nil
}

func (h *Hub) EventBus() EventBus {
    if h.eventBus != nil {
        return h.eventBus
    }
    // 未显式设置时 fallback 到全局实例或默认 LocalEventBus
    if bus := EventBusInstance(); bus != nil {
        return bus
    }
    if h.server != nil {
        return NewLocalEventBus(h.server)
    }
    return nil
}
```

**广播替换（18 处）：**

```go
// 替换前
h.server.BroadcastToNamespace("/", EventMemberLeft, payload)

// 替换后
if eb := h.EventBus(); eb != nil {
    eb.PublishNamespace(EventMemberLeft, payload)
}
```

- `h.server.BroadcastToNamespace("/", ...)` → `h.EventBus().PublishNamespace(...)` (15处)
- `h.server.BroadcastToRoom("/", ...)` → `h.EventBus().PublishRoom(...)` (1处)
- `Hub.BroadcastToRoom`、`BroadcastMute`、`BroadcastUnmute` 内部改为 `h.EventBus().Publish*` (3处)
- `h.server` 保留用于 `ForEach` 操作（OnRoomKick 中遍历房间成员），不替换

**测试验证：**
- `TestHub_DefaultEventBus` — 未调用 SetEventBus 时 EventBus() 返回有效总线
- `TestHub_CustomEventBus` — 调用 SetEventBus 后广播委托到指定总线
- 全部现有 hub_test.go 测试继续通过（惰性回退保证向后兼容）

- [ ] **步骤 1**：编写 `hub_event_bus_test.go`
- [ ] **步骤 2**：修改 `hub.go`
- [ ] **步骤 3**：`go build ./...` 编译通过
- [ ] **步骤 4**：运行全部 hub 测试（原有 + 新增）
- [ ] **步骤 5**：`git commit`

---

### 任务 6：接入服务器启动 + 监控面板 + 部署文档

**文件：**
- 修改 `app/server/server/gin.go`
- 修改 `app/server/internal/handler/monitor_handler.go`
- 添加 `deploy/docker-compose.example.yml` 中的 NATS 服务
- 修改 `ARCHITECTURE.md`

**gin.go 变更：**

```go
import (
    "GOSpeak/internal/signal"
)

// 在 sioServer := socketio.NewServer(...) 之后
closeEventBus := signal.InitEventBus(cfg, sioServer)

// 移出：
// signalHub := signal.NewHub(roomSvc, muteSvc, userSvc, permSvc)
// signalHub.SetSFU(sfuProvider)
// ...
// 修改为：
signalHub := signal.NewHub(roomSvc, muteSvc, userSvc, permSvc)
signalHub.SetSFU(sfuProvider)
signalHub.SetEventBus(signal.EventBusInstance())
// ... 其余不变
```

**优雅关闭追加：**

```go
if err := sioServer.Close(); err != nil {
    log.Printf("[Socket.IO] close error: %v", err)
}
log.Println("[Socket.IO] connections closed")

if err := closeEventBus(); err != nil {
    log.Printf("[EventBus] close error: %v", err)
}
```

**monitor_handler.go 变更：**

在 `healthSnapshot` 结构体中添加：

```go
// EventBus
EventBusMode          string `json:"eventbus_mode"`
EventBusNATSConnected bool   `json:"eventbus_nats_connected"`
```

在 `collect()` 中添加：

```go
ebs := gpsignal.GetEventBusStats()
snap.EventBusMode = ebs.Mode
snap.EventBusNATSConnected = ebs.NATSConnected
```

**deploy/docker-compose.example.yml：**

```yaml
  # ===========================================================================
  # NATS — 可选多实例信号事件总线
  # ===========================================================================
  # 启用: .env 设 NATS_URL="nats://nats:4222"
  nats:
    image: nats:2-alpine
    container_name: gospeak-nats
    restart: unless-stopped
    ports:
      - "4222:4222"
      - "8222:8222"
```

**ARCHITECTURE.md 新增：**

`````markdown
### 可选的 NATS 信号事件总线

GOSpeak 支持可选 NATS 事件总线，用于跨实例信号事件广播。

**工作方式：**
1. 事件先投递到当前进程的 Socket.IO 客户端（本地投递）
2. 然后以 JSON 信封格式发布到 NATS
3. 订阅了对等 subject 的其他实例接收后重新投递到各自的 Socket.IO 客户端
4. 发送方实例通过 `instanceId` 跳过自身消息

**降级机制（对齐 Redis 模块模式）：**
- `NATS_URL=""` → 纯本地模式，零外部依赖，行为不变
- NATS 连接失败 → 日志警告，降级到本地模式，不 panic
- NATS 订阅失败 → 关闭连接，降级到本地模式
- 运行时断连 → NATS 内置自动重连，断连期间事件仅本地投递

**环境变量：**

```env
NATS_URL=""                    # 空值禁用（单节点默认）
NATS_SUBJECT_PREFIX="gospeak"  # NATS subject 前缀
```

**Subject 布局：**

- `{prefix}.signal.namespace` — namespace 级别事件
- `{prefix}.signal.room.{room}` — 按 room 拆分的事件

**约束：**
- 仅扇出事件，不分发房间成员状态、禁言状态、SRS 流状态
- 自身消息通过 `instanceId` 去重，避免重复投递
- 运行时不支持从 local 切换到 NATS（重启生效）
`````

- [ ] **步骤 1**：修改 `gin.go`
- [ ] **步骤 2**：修改 `monitor_handler.go`
- [ ] **步骤 3**：修改 `deploy/docker-compose.example.yml`
- [ ] **步骤 4**：修改 `ARCHITECTURE.md`
- [ ] **步骤 5**：`go build ./...` 编译通过
- [ ] **步骤 6**：`git commit`

---

## 自我审查

### Redis 模式对齐清单
| 检查项 | Redis `internal/redis/` | NATS `internal/signal/` |
|--------|------------------------|------------------------|
| 全局单例 | `var Client *redis.Client` | `var globalBus EventBus` |
| 条件初始化 | `InitRedis()` | `InitEventBus(cfg, sio)` |
| 消费前检查 | `if Client == nil { return }` | `if eb := h.EventBus(); eb != nil { }` |
| 监控状态 | `GetStats() RedisStats` | `GetEventBusStats() EventBusStats` |
| 连接失败 | log + 不赋值 Client | log + globalBus = LocalEventBus |
| 运行时断连 | Redis 连接断开后操作跳过 | NATS 断连后 `Publish` 跳过 |
| 重试机制 | 无（启动时一次性） | NATS 内置自动重连，不额外实现 |
| 配置变更 | 重启生效 | 重启生效 |

### 占位符检查
无 `TBD`、`TODO`、`implement later` 残留。

### 类型一致性
`EventBus` → `LocalEventBus` / `NATSEventBus` → `InitEventBus` → `EventBusInstance` → `GetEventBusStats`
`Connector` → `NATSConn` / `fakeConnector`
`EventEnvelope` → `EventBusStats`

### 向后兼容
- `NATS_URL=""` → 行为完全不变
- Hub 未调用 `SetEventBus` → 惰性回退到 `EventBusInstance()` 或默认 `LocalEventBus`
- 全部现有 hub 测试继续通过
- `InitEventBus` 调用位置在 sioServer 创建之后、hub.SetupRoutes 之前
