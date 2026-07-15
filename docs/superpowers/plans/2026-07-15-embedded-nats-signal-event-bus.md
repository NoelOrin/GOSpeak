# 内嵌 NATS 信号事件总线 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Supersedes:** `docs/superpowers/plans/2026-07-08-optional-nats-signal-event-bus.md`  
> 旧计划是「外部 NATS 可选 + MemoryBus 降级」。本计划改为：**默认内嵌 NATS；配置了 `NATS_URL` 时先探测外部可用性，可用才切外部，不可用则 Warn 并回退内嵌。**

**Goal:** 让 GOSpeak 单二进制默认自带可用的 NATS 信令事件总线；单机零外部依赖，多副本通过外部 NATS 做跨实例 fanout。

**Architecture:** 抽出 `EventBus` 接口，Hub 所有对外 Socket.IO 广播改为 `PublishRoom` / `PublishNamespace`。启动时：`NATS_URL` 为空则启动进程内 `nats-server`；`NATS_URL` 非空则先探测外部可用性，可用才连外部且不启内嵌，不可用则 Warn 并回退内嵌。每个实例本地投递 +（NATS 模式）发布信封；订阅方按 `instance_id` 跳过自身消息后转本地 Socket.IO。

**Tech Stack:** Go 1.26、`github.com/nats-io/nats-server/v2`（内嵌）、`github.com/nats-io/nats.go`（客户端，已在 go.mod indirect）、go-socket.io v1.7.0、Docker Compose 可选外部 NATS profile。

---

## 决策锁定

| 项 | 决策 |
|----|------|
| 默认模式 | **内嵌 NATS**（单二进制开箱即用） |
| 外部模式 | `NATS_URL` 非空 **且探测可用** → 连外部、不启内嵌 |
| 外部不可用 | `NATS_URL` 已设但探测失败 → **回退内嵌**，打 Warn，进程继续启动 |
| 是否保留 MemoryBus | **否**（始终走 NATS client；内嵌或外部二选一） |
| 探测方式 | 启动期短超时 `nats.Connect` + `Flush`；失败关闭探测连接再启内嵌 |
| 状态同步 | **不做**（`Hub.rooms` / stream 注册表仍进程内） |
| JetStream | **本阶段不做**（纯 core NATS pub/sub） |
| 打包 | 客户端 + 内嵌 server 库链进二进制；不单独附 `nats-server` 可执行文件 |
| 多副本 | 必须设**同一个**外部 `NATS_URL`；各副本各自内嵌 **不能互通** |

```text
gospeak 二进制（web embed + nats client + nats-server lib）
        │
        ├─ NATS_URL 为空
        │     → 启动内嵌 nats-server → client 连 ClientURL()  mode=embedded
        │
        └─ NATS_URL 非空
              → Probe(NATS_URL, timeout)
                    ├─ 可用   → 不启内嵌 → client 连外部     mode=external
                    └─ 不可用 → Warn 日志 → 启动内嵌回退     mode=embedded(fallback)
```

```text
业务事件
  → EventBus.PublishRoom/Namespace
  → 本地 socketio 立即投递
  → NATS Publish 信封
  → 其他实例 Subscribe
  → instance_id 去重
  → 对端本地 socketio 投递
```

---

## 文件结构

| 文件 | 职责 |
|------|------|
| Create: `app/server/internal/bus/bus.go` | `EventBus` 接口、`Envelope`、subject 常量/构造 |
| Create: `app/server/internal/bus/local_deliver.go` | 把 envelope 投到 socketio 的交付器接口 |
| Create: `app/server/internal/bus/nats_bus.go` | NATS client 实现：publish + subscribe + 去重 |
| Create: `app/server/internal/bus/embedded.go` | 内嵌 `nats-server` 启停 |
| Create: `app/server/internal/bus/probe.go` | `ProbeExternal(url, timeout)` 启动期探测 |
| Create: `app/server/internal/bus/factory.go` | `Init`：探测外部 → external 或 fallback 内嵌 |
| Create: `app/server/internal/bus/bus_test.go` | Envelope/subject/接口契约测试 |
| Create: `app/server/internal/bus/nats_bus_test.go` | 内嵌 NATS 集成测试（测试里启 embedded） |
| Create: `app/server/internal/bus/embedded_test.go` | 内嵌 server 启停与 ClientURL |
| Modify: `app/server/internal/config/config.go` | `NATSURL`、`NATSSubjectPrefix`、`NATSName` |
| Modify: `app/server/internal/signal/hub.go` | 广播出口改走 EventBus |
| Create: `app/server/internal/signal/hub_event_bus_test.go` | Hub + fake bus / real bus 广播测试 |
| Modify: `app/server/server/gin.go` | 启动 Init bus、注入 Hub、优雅关闭 |
| Modify: `app/server/internal/handler/monitor_handler.go` | health 增加 eventbus 字段 |
| Modify: `deploy/docker-compose.yml` | 可选 `nats` profile（外部） |
| Modify: `deploy/docker-compose.example.yml` | 同上示例 |
| Modify: `deploy/DEPLOY.md` | 内嵌默认 / 外部多副本说明 |
| Modify: `app/server/.env.example`、`app/server/.env.dev` | `NATS_URL` 文档 |
| Modify: `ARCHITECTURE.md` | 事件总线章节 |
| Modify: `app/server/go.mod` | 将 `nats.go` 升为 direct；加入 `nats-server/v2` |

**明确不改：** MediaSoup bridge、SRS/LiveKit webhook 业务逻辑、`Hub.rooms` 存储、前端协议事件名。

---

## Subject 与信封

```text
{prefix}.signal.namespace          # 默认 prefix=gospeak
{prefix}.signal.room.{room}
```

```go
type Envelope struct {
    V          int             `json:"v"`           // 恒为 1
    InstanceID string          `json:"instance_id"`
    Scope      string          `json:"scope"`       // "namespace" | "room"
    Room       string          `json:"room,omitempty"`
    Event      string          `json:"event"`       // 原 socket 事件名，如 member:joined
    Payload    json.RawMessage `json:"payload"`
    TS         int64           `json:"ts"`          // unix milli
}
```

---

### Task 1: Config 增加 NATS 字段

**Files:**
- Modify: `app/server/internal/config/config.go`
- Test: 无独立测试文件；由后续 factory 测试覆盖读取路径

- [ ] **Step 1: 在 `Config` 增加字段**

在 `app/server/internal/config/config.go` 的 `Config` struct 中、`RedisDB` 附近追加：

```go
NATSURL            string
NATSSubjectPrefix  string
NATSName           string
NATSConnectTimeout string
```

- [ ] **Step 2: 在 `Load()` 填充默认值**

```go
NATSURL:            getEnv("NATS_URL", ""),
NATSSubjectPrefix:  getEnv("NATS_SUBJECT_PREFIX", "gospeak"),
NATSName:           getEnv("NATS_NAME", ""),
NATSConnectTimeout: getEnv("NATS_CONNECT_TIMEOUT", "2s"),
```

语义：
- `NATS_URL=""` → 直接内嵌
- `NATS_URL="nats://nats:4222"` → **先探测**；可用则外部，不可用则回退内嵌
- `NATS_CONNECT_TIMEOUT` 默认 `2s`，用于外部探测与连接
- `NATS_SUBJECT_PREFIX` 默认 `gospeak`
- `NATS_NAME` 空时 factory 用 `gospeak-<hostname>-<pid>`

- [ ] **Step 3: 更新 env 示例注释**

在 `app/server/.env.example` 与 `app/server/.env.dev` 的 Redis 段落后追加：

```bash
# NATS 信号事件总线
# 空 = 进程内嵌 nats-server（单机默认，零外部依赖）
# 非空 = 先探测外部可用性；可用则连外部，不可用则 Warn 并回退内嵌
# 多副本必须所有实例都能连上同一个外部 NATS，例如 nats://nats:4222
NATS_URL=""
NATS_SUBJECT_PREFIX="gospeak"
NATS_CONNECT_TIMEOUT="2s"   # 外部探测/连接超时
# NATS_NAME=""   # 可选，连接名；空则自动生成
```

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/config/config.go app/server/.env.example app/server/.env.dev
git commit -m "feat(config): add NATS_URL for embedded/external bus"
```

---

### Task 2: EventBus 接口 + subject/信封

**Files:**
- Create: `app/server/internal/bus/bus.go`
- Create: `app/server/internal/bus/bus_test.go`

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/bus/bus_test.go`：

```go
package bus

import (
	"encoding/json"
	"testing"
)

func TestSubjectHelpers(t *testing.T) {
	if got := NamespaceSubject("gospeak"); got != "gospeak.signal.namespace" {
		t.Fatalf("NamespaceSubject = %q", got)
	}
	if got := RoomSubject("gospeak", "lobby"); got != "gospeak.signal.room.lobby" {
		t.Fatalf("RoomSubject = %q", got)
	}
}

func TestNewEnvelopeRoundTrip(t *testing.T) {
	payload := map[string]any{"identity": "alice"}
	env, err := NewEnvelope("inst-1", "room", "lobby", "member:joined", payload)
	if err != nil {
		t.Fatal(err)
	}
	if env.V != 1 || env.InstanceID != "inst-1" || env.Scope != "room" || env.Room != "lobby" || env.Event != "member:joined" {
		t.Fatalf("envelope fields wrong: %+v", env)
	}
	if env.TS <= 0 {
		t.Fatal("ts should be set")
	}
	var got map[string]any
	if err := json.Unmarshal(env.Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got["identity"] != "alice" {
		t.Fatalf("payload = %#v", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run:

```bash
cd app/server && go test ./internal/bus -run 'TestSubjectHelpers|TestNewEnvelopeRoundTrip' -count=1
```

Expected: FAIL（package 不存在或符号未定义）

- [ ] **Step 3: 最小实现**

创建 `app/server/internal/bus/bus.go`：

```go
package bus

import (
	"context"
	"encoding/json"
	"time"
)

const EnvelopeVersion = 1

type Envelope struct {
	V          int             `json:"v"`
	InstanceID string          `json:"instance_id"`
	Scope      string          `json:"scope"`
	Room       string          `json:"room,omitempty"`
	Event      string          `json:"event"`
	Payload    json.RawMessage `json:"payload"`
	TS         int64           `json:"ts"`
}

// Deliverer 将事件投递到本机 Socket.IO。
type Deliverer interface {
	BroadcastToNamespace(event string, data interface{})
	BroadcastToRoom(room, event string, data interface{})
}

// EventBus 信号事件总线。
type EventBus interface {
	// PublishNamespace 先本地投递，再发布到 NATS（若可用）。
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
	// PublishRoom 先本地投递到 room，再发布到 NATS。
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
	Mode() string // "embedded" | "external"
	IsConnected() bool
	InstanceID() string
	Close() error
}

func NamespaceSubject(prefix string) string {
	return prefix + ".signal.namespace"
}

func RoomSubject(prefix, room string) string {
	return prefix + ".signal.room." + room
}

func NewEnvelope(instanceID, scope, room, event string, payload interface{}) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	if raw == nil {
		raw = []byte("null")
	}
	return Envelope{
		V:          EnvelopeVersion,
		InstanceID: instanceID,
		Scope:      scope,
		Room:       room,
		Event:      event,
		Payload:    raw,
		TS:         time.Now().UnixMilli(),
	}, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd app/server && go test ./internal/bus -run 'TestSubjectHelpers|TestNewEnvelopeRoundTrip' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/bus/bus.go app/server/internal/bus/bus_test.go
git commit -m "feat(bus): add EventBus interface and envelope helpers"
```

---

### Task 3: 内嵌 NATS Server

**Files:**
- Create: `app/server/internal/bus/embedded.go`
- Create: `app/server/internal/bus/embedded_test.go`
- Modify: `app/server/go.mod`（添加 `github.com/nats-io/nats-server/v2`）

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/bus/embedded_test.go`：

```go
package bus

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestStartEmbeddedServer_ClientCanConnect(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatalf("StartEmbeddedServer: %v", err)
	}
	defer es.Shutdown()

	url := es.ClientURL()
	if url == "" {
		t.Fatal("empty ClientURL")
	}
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect embedded: %v", err)
	}
	defer nc.Close()
	if !nc.IsConnected() {
		t.Fatal("expected connected")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd app/server && go test ./internal/bus -run TestStartEmbeddedServer_ClientCanConnect -count=1
```

Expected: FAIL（`StartEmbeddedServer` 未定义）

- [ ] **Step 3: 实现内嵌 server**

```bash
cd app/server && go get github.com/nats-io/nats-server/v2@latest
cd app/server && go get github.com/nats-io/nats.go@v1.48.0
# 确保 nats.go 为 direct require
```

创建 `app/server/internal/bus/embedded.go`：

```go
package bus

import (
	"fmt"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

// EmbeddedServer 进程内 nats-server 句柄。
type EmbeddedServer struct {
	ns *natsserver.Server
}

// StartEmbeddedServer 启动仅监听本机随机端口的内嵌 NATS。
// Port=-1 避免与宿主机/其他副本抢 4222。
func StartEmbeddedServer() (*EmbeddedServer, error) {
	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		MaxPayload: 1 << 20, // 1MiB，信令 JSON 足够
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("nats embedded: new server: %w", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		return nil, fmt.Errorf("nats embedded: not ready for connections")
	}
	return &EmbeddedServer{ns: ns}, nil
}

func (e *EmbeddedServer) ClientURL() string {
	if e == nil || e.ns == nil {
		return ""
	}
	return e.ns.ClientURL()
}

func (e *EmbeddedServer) Shutdown() {
	if e == nil || e.ns == nil {
		return
	}
	e.ns.Shutdown()
	e.ns.WaitForShutdown()
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd app/server && go test ./internal/bus -run TestStartEmbeddedServer_ClientCanConnect -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/bus/embedded.go app/server/internal/bus/embedded_test.go app/server/go.mod app/server/go.sum
git commit -m "feat(bus): embed nats-server for single-binary default"
```

---

### Task 4: NATS EventBus 实现（本地投递 + pub/sub 去重）

**Files:**
- Create: `app/server/internal/bus/nats_bus.go`
- Create: `app/server/internal/bus/nats_bus_test.go`

- [ ] **Step 1: 写失败测试（双 bus 模拟两实例 fanout）**

创建 `app/server/internal/bus/nats_bus_test.go`：

```go
package bus

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memDeliverer struct {
	mu         sync.Mutex
	namespace  []string
	roomEvents []string
}

func (m *memDeliverer) BroadcastToNamespace(event string, data interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.namespace = append(m.namespace, event)
}

func (m *memDeliverer) BroadcastToRoom(room, event string, data interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roomEvents = append(m.roomEvents, room+":"+event)
}

func TestNATSBus_FanoutToPeerSkipsSelf(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()
	url := es.ClientURL()

	d1 := &memDeliverer{}
	d2 := &memDeliverer{}

	b1, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		Prefix:        "gospeak",
		Name:          "inst-a",
		InstanceID:    "inst-a",
		Mode:          "embedded",
		Deliverer:     d1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b1.Close()

	b2, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		Prefix:        "gospeak",
		Name:          "inst-b",
		InstanceID:    "inst-b",
		Mode:          "embedded",
		Deliverer:     d2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()

	// 等订阅就绪
	time.Sleep(100 * time.Millisecond)

	if err := b1.PublishRoom(context.Background(), "r1", "member:joined", map[string]string{"id": "x"}); err != nil {
		t.Fatal(err)
	}

	// 本机必达
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		d1.mu.Lock()
		n1 := len(d1.roomEvents)
		d1.mu.Unlock()
		d2.mu.Lock()
		n2 := len(d2.roomEvents)
		d2.mu.Unlock()
		if n1 >= 1 && n2 >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	d1.mu.Lock()
	defer d1.mu.Unlock()
	d2.mu.Lock()
	defer d2.mu.Unlock()

	if len(d1.roomEvents) != 1 {
		t.Fatalf("local deliver want 1, got %v", d1.roomEvents)
	}
	if len(d2.roomEvents) != 1 {
		t.Fatalf("peer deliver want 1, got %v", d2.roomEvents)
	}
	// 发布方不应因 NATS 回环再投递一次（instance_id 去重）
	if len(d1.roomEvents) > 1 {
		t.Fatalf("self echo not deduped: %v", d1.roomEvents)
	}
}

func TestNATSBus_PublishNamespace(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	d1 := &memDeliverer{}
	d2 := &memDeliverer{}
	b1, err := NewNATSBus(NATSBusConfig{URL: es.ClientURL(), Prefix: "gospeak", InstanceID: "a", Name: "a", Mode: "external", Deliverer: d1})
	if err != nil {
		t.Fatal(err)
	}
	defer b1.Close()
	b2, err := NewNATSBus(NATSBusConfig{URL: es.ClientURL(), Prefix: "gospeak", InstanceID: "b", Name: "b", Mode: "external", Deliverer: d2})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	time.Sleep(100 * time.Millisecond)

	if err := b1.PublishNamespace(context.Background(), "user:muted", map[string]any{"user_id": 1}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		d2.mu.Lock()
		n := len(d2.namespace)
		d2.mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	d1.mu.Lock()
	d2.mu.Lock()
	defer d1.mu.Unlock()
	defer d2.mu.Unlock()
	if len(d1.namespace) != 1 || d1.namespace[0] != "user:muted" {
		t.Fatalf("local namespace = %v", d1.namespace)
	}
	if len(d2.namespace) != 1 || d2.namespace[0] != "user:muted" {
		t.Fatalf("peer namespace = %v", d2.namespace)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd app/server && go test ./internal/bus -run 'TestNATSBus_' -count=1
```

Expected: FAIL（`NewNATSBus` 未定义）

- [ ] **Step 3: 实现 `nats_bus.go`**

创建 `app/server/internal/bus/nats_bus.go`：

```go
package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSBusConfig struct {
	URL        string
	Prefix     string
	Name       string
	InstanceID string
	Mode       string // "embedded" | "external"
	Deliverer  Deliverer
}

type NATSBus struct {
	nc                   *nats.Conn
	prefix               string
	instanceID           string
	mode                 string
	deliverer            Deliverer
	subs                 []*nats.Subscription
	mu                   sync.Mutex
	closed               bool
	fallbackFromExternal bool // Init 在外部探测失败回退内嵌时置 true
}

func NewNATSBus(cfg NATSBusConfig) (*NATSBus, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("nats bus: empty URL")
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	if cfg.InstanceID == "" {
		return nil, fmt.Errorf("nats bus: empty InstanceID")
	}
	if cfg.Deliverer == nil {
		return nil, fmt.Errorf("nats bus: nil Deliverer")
	}
	if cfg.Mode == "" {
		cfg.Mode = "external"
	}

	nc, err := nats.Connect(cfg.URL,
		nats.Name(cfg.Name),
		nats.Timeout(3*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Printf("[EventBus] nats disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Printf("[EventBus] nats reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats bus: connect %s: %w", cfg.URL, err)
	}

	b := &NATSBus{
		nc:         nc,
		prefix:     cfg.Prefix,
		instanceID: cfg.InstanceID,
		mode:       cfg.Mode,
		deliverer:  cfg.Deliverer,
	}
	if err := b.subscribeAll(); err != nil {
		nc.Close()
		return nil, err
	}
	return b, nil
}

func (b *NATSBus) subscribeAll() error {
	// 房间事件用通配；namespace 用固定 subject
	nsSub, err := b.nc.Subscribe(NamespaceSubject(b.prefix), b.onMessage)
	if err != nil {
		return fmt.Errorf("nats bus: subscribe namespace: %w", err)
	}
	roomSub, err := b.nc.Subscribe(b.prefix+".signal.room.*", b.onMessage)
	if err != nil {
		_ = nsSub.Unsubscribe()
		return fmt.Errorf("nats bus: subscribe room: %w", err)
	}
	if err := b.nc.Flush(); err != nil {
		_ = nsSub.Unsubscribe()
		_ = roomSub.Unsubscribe()
		return fmt.Errorf("nats bus: flush: %w", err)
	}
	b.subs = []*nats.Subscription{nsSub, roomSub}
	return nil
}

func (b *NATSBus) onMessage(msg *nats.Msg) {
	var env Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		log.Printf("[EventBus] bad envelope: %v", err)
		return
	}
	if env.InstanceID == b.instanceID {
		return // 跳过自身
	}
	var payload interface{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			payload = json.RawMessage(env.Payload)
		}
	}
	switch env.Scope {
	case "room":
		b.deliverer.BroadcastToRoom(env.Room, env.Event, payload)
	default:
		b.deliverer.BroadcastToNamespace(env.Event, payload)
	}
}

func (b *NATSBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	_ = ctx
	b.deliverer.BroadcastToNamespace(event, payload)
	return b.publish(NamespaceSubject(b.prefix), "namespace", "", event, payload)
}

func (b *NATSBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	_ = ctx
	b.deliverer.BroadcastToRoom(room, event, payload)
	return b.publish(RoomSubject(b.prefix, room), "room", room, event, payload)
}

func (b *NATSBus) publish(subject, scope, room, event string, payload interface{}) error {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed || b.nc == nil || !b.nc.IsConnected() {
		// 本地已投递；NATS 不可用时只记日志，避免广播路径 panic
		log.Printf("[EventBus] skip nats publish (disconnected): %s %s", scope, event)
		return nil
	}
	env, err := NewEnvelope(b.instanceID, scope, room, event, payload)
	if err != nil {
		return err
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if err := b.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("nats publish %s: %w", subject, err)
	}
	return nil
}

func (b *NATSBus) Mode() string        { return b.mode }
func (b *NATSBus) IsConnected() bool   { return b.nc != nil && b.nc.IsConnected() }
func (b *NATSBus) InstanceID() string  { return b.instanceID }

func (b *NATSBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, s := range b.subs {
		_ = s.Unsubscribe()
	}
	b.subs = nil
	if b.nc != nil {
		b.nc.Close()
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd app/server && go test ./internal/bus -run 'TestNATSBus_' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/bus/nats_bus.go app/server/internal/bus/nats_bus_test.go
git commit -m "feat(bus): nats EventBus with local deliver and peer fanout"
```

---

### Task 5: Factory — 探测外部可用性后再决定内嵌/外部

**Files:**
- Create: `app/server/internal/bus/factory.go`
- Create: `app/server/internal/bus/factory_test.go`
- Create: `app/server/internal/bus/sio_deliverer.go`
- Create: `app/server/internal/bus/probe.go`

决策伪代码：

```text
if NATS_URL == "":
    startEmbedded()
    connect(ClientURL) → mode=embedded
else:
    if probe(NATS_URL, timeout) OK:
        connect(NATS_URL) → mode=external   // 不启内嵌
    else:
        log.Warn("external nats unavailable, fallback to embedded: ...")
        startEmbedded()
        connect(ClientURL) → mode=embedded  // 可选: mode 仍报 embedded；Stats 可加 FallbackFromExternal=true
```

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/bus/factory_test.go`：

```go
package bus

import (
	"strings"
	"testing"
	"time"
)

type nopDeliverer struct{}

func (nopDeliverer) BroadcastToNamespace(string, interface{})    {}
func (nopDeliverer) BroadcastToRoom(string, string, interface{}) {}

func TestInit_EmptyURL_StartsEmbedded(t *testing.T) {
	b, cleanup, err := Init(InitConfig{
		URL:       "",
		Prefix:    "gospeak",
		Deliverer: nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if b.Mode() != "embedded" {
		t.Fatalf("mode = %q, want embedded", b.Mode())
	}
	if !b.IsConnected() {
		t.Fatal("expected connected")
	}
	if b.InstanceID() == "" {
		t.Fatal("empty instance id")
	}
}

func TestInit_ExternalURL_Available_UsesExternalNoEmbed(t *testing.T) {
	// 用内嵌 server 模拟“已存在的外部 NATS”
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	b, cleanup, err := Init(InitConfig{
		URL:            es.ClientURL(),
		Prefix:         "gospeak",
		Name:           "ext-test",
		ConnectTimeout: 2 * time.Second,
		Deliverer:      nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if b.Mode() != "external" {
		t.Fatalf("mode = %q, want external", b.Mode())
	}
	if !b.IsConnected() {
		t.Fatal("expected connected")
	}
}

func TestInit_ExternalURL_Unavailable_FallsBackEmbedded(t *testing.T) {
	b, cleanup, err := Init(InitConfig{
		URL:            "nats://127.0.0.1:1", // 必不可达
		Prefix:         "gospeak",
		ConnectTimeout: 300 * time.Millisecond,
		Deliverer:      nopDeliverer{},
	})
	if err != nil {
		t.Fatalf("should fallback embedded, not fail: %v", err)
	}
	defer cleanup()
	if b.Mode() != "embedded" {
		t.Fatalf("mode = %q, want embedded fallback", b.Mode())
	}
	if !b.IsConnected() {
		t.Fatal("expected connected to embedded")
	}
	st := GetStats(b)
	if !st.FallbackFromExternal {
		t.Fatal("expected FallbackFromExternal=true")
	}
}

func TestProbeExternal_OKAndFail(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	if err := ProbeExternal(es.ClientURL(), time.Second); err != nil {
		t.Fatalf("probe ok url: %v", err)
	}
	if err := ProbeExternal("nats://127.0.0.1:1", 200*time.Millisecond); err == nil {
		t.Fatal("probe bad url should fail")
	}
}

func TestInit_ExternalBadURL_MessageContainsProbe(t *testing.T) {
	// 仅文档/日志语义：不可达时不返回 error；此测试确保 fallback 路径可启动
	b, cleanup, err := Init(InitConfig{
		URL:            "nats://127.0.0.1:1",
		Prefix:         "gospeak",
		ConnectTimeout: 200 * time.Millisecond,
		Deliverer:      nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !strings.Contains(b.Mode(), "embedded") {
		t.Fatalf("mode=%s", b.Mode())
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd app/server && go test ./internal/bus -run 'TestInit_|TestProbeExternal' -count=1
```

Expected: FAIL

- [ ] **Step 3: 实现 probe + factory + socketio deliverer**

创建 `app/server/internal/bus/probe.go`：

```go
package bus

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// ProbeExternal 校验外部 NATS 是否可连接。
// 成功时立即关闭探测连接（正式连接由 NewNATSBus 建立）。
func ProbeExternal(url string, timeout time.Duration) error {
	if url == "" {
		return fmt.Errorf("nats probe: empty url")
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	nc, err := nats.Connect(url,
		nats.Name("gospeak-probe"),
		nats.Timeout(timeout),
		nats.MaxReconnects(0),
		nats.DontRandomize(),
	)
	if err != nil {
		return fmt.Errorf("nats probe connect: %w", err)
	}
	defer nc.Close()
	if err := nc.FlushTimeout(timeout); err != nil {
		return fmt.Errorf("nats probe flush: %w", err)
	}
	if !nc.IsConnected() {
		return fmt.Errorf("nats probe: not connected")
	}
	return nil
}
```

创建 `app/server/internal/bus/factory.go`：

```go
package bus

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type InitConfig struct {
	URL            string // 空 = 直接内嵌；非空 = 先探测
	Prefix         string
	Name           string
	ConnectTimeout time.Duration
	Deliverer      Deliverer
}

// Init 按配置创建 EventBus。
// 1) URL 空 → 内嵌
// 2) URL 非空且 Probe 成功 → 外部（不启内嵌）
// 3) URL 非空但 Probe 失败 → Warn + 回退内嵌（进程不失败）
// cleanup：先 Close bus，再 Shutdown 内嵌（若有）。
func Init(cfg InitConfig) (EventBus, func(), error) {
	if cfg.Deliverer == nil {
		return nil, func() {}, fmt.Errorf("bus init: nil Deliverer")
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 2 * time.Second
	}

	instanceID := cfg.Name
	if instanceID == "" {
		host, _ := os.Hostname()
		instanceID = fmt.Sprintf("gospeak-%s-%d", sanitize(host), os.Getpid())
	}
	name := cfg.Name
	if name == "" {
		name = instanceID
	}

	url := strings.TrimSpace(cfg.URL)
	mode := "embedded"
	fallbackFromExternal := false
	var embedded *EmbeddedServer

	if url != "" {
		if err := ProbeExternal(url, cfg.ConnectTimeout); err != nil {
			log.Printf("[EventBus] external nats unavailable (%s): %v; fallback to embedded", url, err)
			fallbackFromExternal = true
			url = "" // 走内嵌分支
		} else {
			mode = "external"
			log.Printf("[EventBus] external nats probe ok: %s instance=%s", url, instanceID)
		}
	}

	if mode == "embedded" {
		es, err := StartEmbeddedServer()
		if err != nil {
			return nil, func() {}, fmt.Errorf("bus init embedded: %w", err)
		}
		embedded = es
		url = es.ClientURL()
		if fallbackFromExternal {
			log.Printf("[EventBus] embedded nats started (fallback): %s instance=%s", url, instanceID)
		} else {
			log.Printf("[EventBus] embedded nats started: %s instance=%s", url, instanceID)
		}
	}

	nb, err := NewNATSBus(NATSBusConfig{
		URL:        url,
		Prefix:     cfg.Prefix,
		Name:       name,
		InstanceID: instanceID,
		Mode:       mode,
		Deliverer:  cfg.Deliverer,
	})
	if err != nil {
		if embedded != nil {
			embedded.Shutdown()
		}
		return nil, func() {}, err
	}
	nb.fallbackFromExternal = fallbackFromExternal

	cleanup := func() {
		_ = nb.Close()
		if embedded != nil {
			embedded.Shutdown()
			log.Printf("[EventBus] embedded nats stopped")
		}
	}
	return nb, cleanup, nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	if s == "" {
		return "unknown"
	}
	return s
}

// Stats 供监控面板。
type Stats struct {
	Mode                 string `json:"mode"`
	Connected            bool   `json:"connected"`
	InstanceID           string `json:"instance_id"`
	FallbackFromExternal bool   `json:"fallback_from_external"`
}

func GetStats(b EventBus) Stats {
	if b == nil {
		return Stats{Mode: "none", Connected: false}
	}
	st := Stats{
		Mode:       b.Mode(),
		Connected:  b.IsConnected(),
		InstanceID: b.InstanceID(),
	}
	if nb, ok := b.(*NATSBus); ok {
		st.FallbackFromExternal = nb.fallbackFromExternal
	}
	return st
}
```

在 `nats_bus.go` 的 `NATSBus` 结构体增加字段（Task 4 实现时一并带上，或本 Task 补丁）：

```go
fallbackFromExternal bool
```

创建 `app/server/internal/bus/sio_deliverer.go`：

```go
package bus

// SocketServer 是 go-socket.io Server 的最小广播面。
type SocketServer interface {
	BroadcastToNamespace(namespace string, event string, args ...interface{}) bool
	BroadcastToRoom(namespace string, room string, event string, args ...interface{}) bool
}

type SIODeliverer struct {
	Server SocketServer
}

func NewSIODeliverer(server SocketServer) *SIODeliverer {
	return &SIODeliverer{Server: server}
}

func (d *SIODeliverer) BroadcastToNamespace(event string, data interface{}) {
	if d == nil || d.Server == nil {
		return
	}
	d.Server.BroadcastToNamespace("/", event, data)
}

func (d *SIODeliverer) BroadcastToRoom(room, event string, data interface{}) {
	if d == nil || d.Server == nil {
		return
	}
	d.Server.BroadcastToRoom("/", room, event, data)
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd app/server && go test ./internal/bus -count=1
```

Expected: PASS  
关键断言：
- 空 URL → embedded
- 可达外部 URL → external，且未要求内嵌
- 不可达外部 URL → **不报错**，mode=embedded，`FallbackFromExternal=true`

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/bus/factory.go app/server/internal/bus/factory_test.go app/server/internal/bus/sio_deliverer.go app/server/internal/bus/probe.go app/server/internal/bus/nats_bus.go
git commit -m "feat(bus): probe external NATS before enable, else embed fallback"
```

### Task 6: Hub 广播改走 EventBus

**Files:**
- Modify: `app/server/internal/signal/hub.go`
- Create: `app/server/internal/signal/hub_event_bus_test.go`

**替换范围（仅广播出口，不改业务状态机）：**

| 场景 | 现有 | 改为 |
|------|------|------|
| 房间内成员事件 | `h.server.BroadcastToRoom("/", room, event, data)` | `h.publishRoom(room, event, data)` |
| 全局房间列表/更新/禁言/SFU切换 | `h.server.BroadcastToNamespace("/", event, data)` | `h.publishNamespace(event, data)` |
| `Hub.BroadcastToRoom` | 直接 server | `publishRoom` |
| `BroadcastMute` / `BroadcastUnmute` | namespace | `publishNamespace` |
| `ForceSFUProviderSwitch` 的 `sfu:provider-changed` | namespace | `publishNamespace`（本地清房间逻辑不变） |

**不要经 bus 的：** 针对单个 `socketio.Conn` 的 `Emit` / ack 回包。

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/signal/hub_event_bus_test.go`：

```go
package signal

import (
	"context"
	"sync"
	"testing"
)

type captureBus struct {
	mu    sync.Mutex
	rooms []string
	ns    []string
}

func (c *captureBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ns = append(c.ns, event)
	return nil
}
func (c *captureBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms = append(c.rooms, room+":"+event)
	return nil
}
func (c *captureBus) Mode() string       { return "test" }
func (c *captureBus) IsConnected() bool  { return true }
func (c *captureBus) InstanceID() string { return "test" }
func (c *captureBus) Close() error       { return nil }

func TestHub_BroadcastToRoom_UsesEventBus(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	bus := &captureBus{}
	hub.SetEventBus(bus)
	// 即使 server 为 nil，也应走 bus
	hub.BroadcastToRoom("lobby", EventMemberJoined, map[string]string{"id": "1"})
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if len(bus.rooms) != 1 || bus.rooms[0] != "lobby:"+EventMemberJoined {
		t.Fatalf("bus rooms = %v", bus.rooms)
	}
}

func TestHub_BroadcastMute_UsesEventBus(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	bus := &captureBus{}
	hub.SetEventBus(bus)
	hub.BroadcastMute(9, &MuteInfo{Permanent: true, Reason: "x"})
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if len(bus.ns) != 1 || bus.ns[0] != EventUserMuted {
		t.Fatalf("bus ns = %v", bus.ns)
	}
}
```

注意：`captureBus` 需满足 Hub 依赖的接口；接口定义在 hub 侧为了避免 signal→bus 循环时，可在 `signal` 包内定义窄接口：

```go
// 在 hub.go
type eventBus interface {
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}
```

Hub 字段用 `eventBus`，`SetEventBus` 接受该接口；`*bus.NATSBus` 自然满足。

- [ ] **Step 2: 跑测试确认失败**

```bash
cd app/server && go test ./internal/signal -run 'TestHub_BroadcastToRoom_UsesEventBus|TestHub_BroadcastMute_UsesEventBus' -count=1
```

Expected: FAIL（`SetEventBus` 不存在）

- [ ] **Step 3: 改 `hub.go`**

1) import 增加 `"context"`（若尚未有）。

2) Hub 结构体增加：

```go
eventBus eventBus
```

3) 新增接口与方法：

```go
type eventBus interface {
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

func (h *Hub) SetEventBus(b eventBus) {
	h.eventBus = b
}

func (h *Hub) publishRoom(room, event string, data interface{}) {
	if h.eventBus != nil {
		if err := h.eventBus.PublishRoom(context.Background(), room, event, data); err != nil {
			log.Printf("[Signal] eventbus publish room %s %s: %v", room, event, err)
		}
		return
	}
	// 无 bus 时回退本机（测试兼容）
	if h.server != nil {
		h.server.BroadcastToRoom("/", room, event, data)
	}
}

func (h *Hub) publishNamespace(event string, data interface{}) {
	if h.eventBus != nil {
		if err := h.eventBus.PublishNamespace(context.Background(), event, data); err != nil {
			log.Printf("[Signal] eventbus publish ns %s: %v", event, err)
		}
		return
	}
	if h.server != nil {
		h.server.BroadcastToNamespace("/", event, data)
	}
}
```

4) 替换所有 **广播向客户端** 的 `h.server.BroadcastToRoom` / `BroadcastToNamespace` 为 `publishRoom` / `publishNamespace`。  
   `Hub.BroadcastToRoom` 改为：

```go
func (h *Hub) BroadcastToRoom(room string, event string, data interface{}) {
	h.publishRoom(room, event, data)
}
```

`BroadcastMute` / `BroadcastUnmute` / `ForceSFUProviderSwitch` 中的广播同理。

**保留** `h.server.ForEach` 等连接级操作不变。

- [ ] **Step 4: 跑 signal 测试**

```bash
cd app/server && go test ./internal/signal -count=1
```

Expected: 全部 PASS（含原有 hub 测试；无 bus 时回退 server 广播）

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/signal/hub.go app/server/internal/signal/hub_event_bus_test.go
git commit -m "feat(signal): route hub broadcasts through EventBus"
```

---

### Task 7: 接入 `gin.go` 启动 / 关闭

**Files:**
- Modify: `app/server/server/gin.go`

- [ ] **Step 1: 在 `sioServer` 创建后、Hub SetupRoutes 前初始化 bus**

在 `app/server/server/gin.go` 增加 import：

```go
"GOSpeak/internal/bus"
```

在 `sioServer := socketio.NewServer(...)` 之后：

```go
deliverer := bus.NewSIODeliverer(sioServer)
timeout, err := time.ParseDuration(cfg.NATSConnectTimeout)
if err != nil || timeout <= 0 {
	timeout = 2 * time.Second
}
eventBus, closeEventBus, err := bus.Init(bus.InitConfig{
	URL:            cfg.NATSURL,
	Prefix:         cfg.NATSSubjectPrefix,
	Name:           cfg.NATSName,
	ConnectTimeout: timeout,
	Deliverer:      deliverer,
})
if err != nil {
	// 仅内嵌启动失败等不可恢复错误会到这里；外部探测失败不会
	panic(fmt.Sprintf("failed to init event bus: %v", err))
}
```

在 `signalHub := signal.NewHub(...)` 后：

```go
signalHub.SetEventBus(eventBus)
```

- [ ] **Step 2: 优雅关闭时先关 Socket.IO，再关 EventBus（含内嵌 server）**

在现有 `sioServer.Close()` 成功日志后追加：

```go
if err := closeEventBus(); err != nil {
	log.Printf("[EventBus] close error: %v", err)
} else {
	log.Println("[EventBus] closed")
}
```

> `closeEventBus` 由 `bus.Init` 返回；需注意闭包变量在 goroutine 中可用（同函数作用域即可）。

- [ ] **Step 3: 编译**

```bash
cd app/server && go build -o /tmp/gospeak-test .
```

Expected: 成功，无错误

- [ ] **Step 4: Commit**

```bash
git add app/server/server/gin.go
git commit -m "feat(server): wire embedded/external NATS event bus on startup"
```

---

### Task 8: 监控面板字段

**Files:**
- Modify: `app/server/internal/handler/monitor_handler.go`

- [ ] **Step 1: 扩展 `healthSnapshot`**

```go
// EventBus
EventBusMode                 string `json:"eventbus_mode"`
EventBusConnected            bool   `json:"eventbus_connected"`
EventBusInstanceID           string `json:"eventbus_instance_id"`
EventBusFallbackFromExternal bool   `json:"eventbus_fallback_from_external"`
```

- [ ] **Step 2: `MonitorHandler` 持有 bus 或从全局取 stats**

推荐构造注入，避免全局：

```go
// MonitorHandler 增加：
eventBus bus.EventBus

func NewMonitorHandler(signalHub *gpsignal.Hub, cfg *config.Config, eventBus bus.EventBus) *MonitorHandler {
	// ...
	h.eventBus = eventBus
	return h
}
```

`collect()`：

```go
es := bus.GetStats(h.eventBus)
snap.EventBusMode = es.Mode
snap.EventBusConnected = es.Connected
snap.EventBusInstanceID = es.InstanceID
snap.EventBusFallbackFromExternal = es.FallbackFromExternal
```

同步改 `gin.go`：

```go
monitorH := handler.NewMonitorHandler(signalHub, cfg, eventBus)
```

- [ ] **Step 3: 编译 + 相关测试**

```bash
cd app/server && go test ./internal/handler -count=1
cd app/server && go build -o /tmp/gospeak-test .
```

Expected: PASS / 编译成功

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/handler/monitor_handler.go app/server/server/gin.go
git commit -m "feat(monitor): expose event bus mode and connectivity"
```

---

### Task 9: 部署与文档

**Files:**
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/docker-compose.example.yml`（若与主文件重复则只改主文件+example 注释）
- Modify: `deploy/DEPLOY.md`
- Modify: `ARCHITECTURE.md`
- Modify: `deploy/env/app.srs.env.example`、`deploy/env/app.livekit.env.example`（追加 NATS 注释行）

- [ ] **Step 1: compose 增加可选外部 NATS**

在 `deploy/docker-compose.yml` 增加：

```yaml
  # ── NATS（可选，多副本信号总线；单机默认用 gospeak 内嵌）──
  nats:
    <<: *restart
    profiles: ["nats"]
    image: nats:2-alpine
    container_name: gospeak-nats
    ports:
      - "${NATS_PORT:-4222}:4222"
      - "${NATS_HTTP_PORT:-8222}:8222"
    command: ["-m", "8222"]
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8222/healthz"]
      interval: 10s
      timeout: 3s
      retries: 6
```

`gospeak` 服务 environment 增加（若已有 env_file 则在 example env 写）：

```yaml
NATS_URL: ${NATS_URL:-}
```

文档说明：
- 单副本：不设 `NATS_URL`，用内嵌
- 多副本：

```bash
NATS_URL=nats://nats:4222
docker compose --profile nats --profile app up -d
```

- [ ] **Step 2: `DEPLOY.md` 增加小节**

```markdown
### NATS 信号事件总线

- **默认**：`NATS_URL` 为空时，gospeak 进程内嵌 nats-server（随机本机端口），单二进制零依赖。
- **外部优先**：`NATS_URL` 非空时先探测可用性（`NATS_CONNECT_TIMEOUT`，默认 2s）。
  - 探测成功 → 连外部，不启内嵌（`eventbus_mode=external`）
  - 探测失败 → 打 Warn，回退内嵌（`eventbus_mode=embedded`，`eventbus_fallback_from_external=true`），进程不退出
- **多副本**：所有实例必须实际连上**同一个**外部 NATS。若探测失败回退内嵌，则跨实例 fanout 失效（各嵌各的）。
- **监控**：SSE health 含 `eventbus_mode`、`eventbus_connected`、`eventbus_fallback_from_external`。
```

- [ ] **Step 3: `ARCHITECTURE.md` 在 Infrastructure 增加 NATS 说明**

写明：信令 fanout 总线；状态仍进程内；内嵌 vs 外部选择规则。

- [ ] **Step 4: Commit**

```bash
git add deploy/docker-compose.yml deploy/docker-compose.example.yml deploy/DEPLOY.md ARCHITECTURE.md deploy/env/*.example
git commit -m "docs(deploy): document embedded NATS default and external profile"
```

---

### Task 10: 全量回归与手工验收清单

- [ ] **Step 1: 单元/包测试**

```bash
cd app/server && go test ./internal/bus ./internal/signal ./internal/handler -count=1
```

Expected: PASS

- [ ] **Step 2: 全量 server 测试（允许跳过需外部依赖的）**

```bash
cd app/server && go test ./... -count=1
```

Expected: PASS（或仅预存无关失败；本功能相关包必须绿）

- [ ] **Step 3: 单机手工（内嵌）**

```bash
# 不设 NATS_URL
cd app/server && go run . server
# 日志应出现： [EventBus] embedded nats started: nats://127.0.0.1:xxxxx
# 浏览器进房/禁言行为与改造前一致
```

- [ ] **Step 4: 外部 NATS 手工（探测成功 / 失败回退）**

```bash
# 4a 外部可用
docker run --rm -p 4222:4222 nats:2-alpine
NATS_URL=nats://127.0.0.1:4222 go run . server
# 日志： external nats probe ok
# 不应出现 embedded nats started

# 4b 外部不可用 → 回退内嵌，进程仍启动
NATS_URL=nats://127.0.0.1:1 NATS_CONNECT_TIMEOUT=500ms go run . server
# 日志： external nats unavailable ... fallback to embedded
# 日志： embedded nats started (fallback)
# health: eventbus_mode=embedded, eventbus_fallback_from_external=true
```

- [ ] **Step 5: 双实例 fanout 手工（外部）**

```bash
# terminal 1
NATS_URL=nats://127.0.0.1:4222 SERVER_PORT=8998 go run . server
# terminal 2
NATS_URL=nats://127.0.0.1:4222 SERVER_PORT=8999 go run . server
# 客户端分别连两端口；在 A 禁言/踢人，B 上连接应收到 user:muted / member:left
```

- [ ] **Step 6: 最终 commit（若有修复）**

```bash
git add -A
git commit -m "test: verify embedded/external NATS signal bus"
```

---

## 行为矩阵（验收标准）

| 配置 | 探测 | 启内嵌？ | Client 连谁 | mode | fallback 标记 | 跨进程 fanout |
|------|------|----------|-------------|------|---------------|----------------|
| `NATS_URL=""` | 跳过 | ✅ | `ClientURL()` | `embedded` | false | 否（单进程） |
| `NATS_URL` 可达 | OK | ❌ | 外部 | `external` | false | ✅ |
| `NATS_URL` 不可达 | FAIL | ✅ 回退 | `ClientURL()` | `embedded` | **true** | ❌（应修外部 NATS） |
| 两进程都内嵌 | — | 各启各的 | 各自本地 | `embedded` | — | ❌ |

---

## 二进制与打包影响

| 项 | 结果 |
|----|------|
| 产物数量 | 仍 1 个 `gospeak` |
| `CGO_ENABLED` | 仍 `0` |
| 新增依赖 | `nats-server/v2` + direct `nats.go` |
| 体积 | 预计 +10–20MB 量级（nats-server 库） |
| 运行时默认端口 | 内嵌用 **随机端口**，不占 4222 |
| 前端 embed | 不变 |

---

## 明确不做（防 scope creep）

- ❌ `Hub.rooms` / `roomStreams` 分布式状态
- ❌ JetStream / 持久化队列
- ❌ MediaSoup request/reply 改 NATS
- ❌ webhook 改投 NATS
- ❌ 把 `nats-server` 可执行文件 `go:embed` 再 exec
- ❌ 运行时热切换 embedded↔external（重启生效）
- ❌ 外部探测失败时 fail-fast 拒启（已改为回退内嵌）

---

## Self-Review

### Spec coverage
| 需求 | 对应 Task |
|------|-----------|
| 默认内嵌 NATS | Task 3, 5, 7 |
| 外部可用才启用外部 | Task 5 `ProbeExternal` + `TestInit_ExternalURL_Available_UsesExternalNoEmbed` |
| 外部不可用回退内嵌 | Task 5 `TestInit_ExternalURL_Unavailable_FallsBackEmbedded` |
| 单二进制可打包 | Task 3 依赖库链接；文档 Task 9 |
| Hub 跨实例广播 | Task 4, 6 |
| 监控可见（含 fallback 标记） | Task 8 |
| 部署说明 | Task 9 |
| 不改媒体/状态存储 | 全文 Out of scope |

### Placeholder scan
无 TBD/TODO/“similar to task N” 占位；关键代码均给出完整片段。

### Type consistency
- `EventBus` / `NATSBus` / `Init` / `Deliverer` / `SIODeliverer` / `Envelope` / `Stats` 命名前后一致
- Hub 侧窄接口 `eventBus` 与 `bus.EventBus` 方法集兼容
- mode 字符串仅 `embedded` | `external`

### 与旧计划差异
| 旧 (2026-07-08) | 新 (本计划) |
|-----------------|-------------|
| 无 URL → MemoryBus | 无 URL → **内嵌 NATS** |
| 外连失败 → 降级本地 MemoryBus | 外连探测失败 → **回退内嵌 NATS**（非 MemoryBus） |
| 有 URL 直接外连 | 有 URL → **先 Probe，可用才 external** |
| 包放 `internal/signal` | 包放 **`internal/bus`**（职责分离） |
| Redis 式 nil 全局 | 显式注入 Hub + cleanup 闭包 |

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-15-embedded-nats-signal-event-bus.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — 每个 Task 派生子代理，Task 间审查，迭代快  
2. **Inline Execution** — 本会话按 executing-plans 连续执行，设检查点  

**Which approach?**
