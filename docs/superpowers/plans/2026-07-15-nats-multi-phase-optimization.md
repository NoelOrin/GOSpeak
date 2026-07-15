# NATS 多阶段优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Related:**
> - 阶段一详细计划（已有）：`docs/superpowers/plans/2026-07-15-embedded-nats-signal-event-bus.md`
> - 旧可选外部 NATS 计划（已 supersede）：`docs/superpowers/plans/2026-07-08-optional-nats-signal-event-bus.md`
> - 阶段一实现 worktree：`.worktrees/codex/embedded-nats-signal-bus`（分支 `codex/embedded-nats-signal-bus`）

**Goal:** 用 NATS 把 GOSpeak 从「单进程 Socket.IO + 内存 Hub」演进为可水平扩展的信令/事件架构：先跨实例 fanout，再共享在线状态，再可靠异步 webhook/清理，最后 MediaSoup 控制面与缓存失效。

**Architecture:** 分 4 个可独立交付阶段。阶段一默认内嵌 NATS（core pub/sub）做 Socket.IO 广播桥；阶段二用 JetStream KV 共享 membership/stream 视图；阶段三用 JetStream 队列做 webhook 与 SFU cleanup 至少一次投递；阶段四把 MediaSoup worker 与权限缓存失效接到同一总线。媒体面（WebRTC）不进 NATS。

**Tech Stack:** Go 1.26、`nats.go`、`nats-server/v2`（内嵌，阶段一）、JetStream（阶段二+）、go-socket.io v1.7.0、Express MediaSoup worker、Docker Compose。

---

## 当前完成度（2026-07-15）

| 阶段 | 状态 | 位置 |
|------|------|------|
| **阶段一：内嵌/外部 NATS 信令 fanout** | **≈90% 完成于 worktree**，**主工作区未合入** | `.worktrees/codex/embedded-nats-signal-bus` |
| 阶段二：在线状态共享 | 未开始 | — |
| 阶段三：可靠异步（webhook/cleanup） | 未开始 | — |
| 阶段四：MediaSoup 控制面 + 缓存失效 | 未开始 | — |

### 阶段一 worktree 已完成

- `app/server/internal/bus/*`：`EventBus`、`Envelope`、embedded server、probe、factory、NATSBus、SIODeliverer、测试
- `config`：`NATS_URL` / prefix / name / timeout
- `signal.Hub`：`publishRoom` / `publishNamespace` 路由广播
- `server/gin.go`：启动/关闭接线
- `monitor_handler`：`eventbus_*` 字段
- deploy / DEPLOY / ARCHITECTURE 文档与 `nats` profile

### 阶段一剩余（合入前必做）

1. worktree 有未提交改动：`localNamespace`（`room:list:result` / 部分 `room:updated` 不跨实例广播错误成员数）
2. 跑通 `go test ./internal/bus ./internal/signal` 与双实例手工 fanout
3. 合入主线（或继续在 worktree 完成后续阶段）

**明确不做（全阶段通用）：**
- ❌ 用 NATS 传 WebRTC 媒体
- ❌ 替换 JWT / DB / 房间持久化表
- ❌ 替换浏览器 Socket.IO 协议事件名
- ❌ 单机强制依赖外部 NATS 集群（阶段一默认内嵌）

---

## 总架构

```text
Client ⇄ Socket.IO ⇄ gospeak A/B/C
                         │
              ┌──────────┴──────────┐
              │   NATS Event Bus    │
              │  (embedded|external)│
              └──────────┬──────────┘
     Phase1: signal.namespace / signal.room.*
     Phase2: state.kv (JetStream KV) + state.events
     Phase3: jobs.webhook.* / jobs.sfu.cleanup
     Phase4: mediasoup.cmd.* / cache.invalidate.*
```

### Subject 约定（全阶段统一前缀，默认 `gospeak`）

| Subject | 阶段 | 语义 |
|---------|------|------|
| `{p}.signal.namespace` | 1 | 全局 Socket 事件信封 |
| `{p}.signal.room.{room}` | 1 | 房间 Socket 事件信封 |
| `{p}.state.membership` (KV bucket) | 2 | room→members 快照 |
| `{p}.state.stream` (KV bucket) | 2 | stream↔room 映射 |
| `{p}.state.event` | 2 | membership/stream 变更通知 |
| `{p}.jobs.webhook.srs` | 3 | SRS 回调任务 |
| `{p}.jobs.webhook.livekit` | 3 | LiveKit webhook 任务 |
| `{p}.jobs.sfu.cleanup` | 3 | disconnect/kick SFU 清理 |
| `{p}.mediasoup.cmd.{worker}` | 4 | request/reply 控制面（可选） |
| `{p}.mediasoup.event` | 4 | worker → 后端事件 |
| `{p}.cache.invalidate` | 4 | 权限等缓存失效 |

### Envelope（阶段一已定，后续复用）

```go
// app/server/internal/bus/bus.go
type Envelope struct {
	V          int             `json:"v"` // 恒为 1
	InstanceID string          `json:"instance_id"`
	Scope      string          `json:"scope"` // namespace|room|job|state|cache
	Room       string          `json:"room,omitempty"`
	Event      string          `json:"event"`
	Payload    json.RawMessage `json:"payload"`
	TS         int64           `json:"ts"`
}
```

---

## 文件结构（全阶段）

| 文件 | 阶段 | 职责 |
|------|------|------|
| `app/server/internal/bus/bus.go` | 1 | EventBus 接口、Envelope、subject helpers |
| `app/server/internal/bus/embedded.go` | 1 | 内嵌 nats-server |
| `app/server/internal/bus/probe.go` | 1 | 外部可用性探测 |
| `app/server/internal/bus/nats_bus.go` | 1 | pub/sub + 本地投递 + instance 去重 |
| `app/server/internal/bus/sio_deliverer.go` | 1 | Socket.IO Deliverer |
| `app/server/internal/bus/factory.go` | 1 | Init：embed / external / fallback |
| `app/server/internal/bus/jetstream.go` | 2 | JetStream context 获取与 stream 确保 |
| `app/server/internal/bus/kv_store.go` | 2 | membership/stream KV 封装 |
| `app/server/internal/bus/queue.go` | 3 | JetStream 队列 publish/consume helpers |
| `app/server/internal/signal/hub.go` | 1–3 | 广播改 bus；状态读写 KV；cleanup 入队 |
| `app/server/internal/signal/state_sync.go` | 2 | 远程状态事件应用 |
| `app/server/internal/handler/srs_callback_handler.go` | 3 | 鉴权后入队 |
| `app/server/internal/handler/signal_handler.go` | 3 | LiveKit webhook 入队并真正处理 |
| `app/server/internal/service/permission_service.go` | 4 | 失效订阅 + 广播失效 |
| `app/server/internal/mediasoup/bridge.go` | 4 | 可选 NATS transport（保留 HTTP 默认） |
| `packages/mediasoup-worker/src/*` | 4 | 可选 NATS sidecar 事件 |
| `deploy/docker-compose.yml` | 1–4 | nats profile；后续 jetstream 配置 |
| `docs/...` | 各阶段 | DEPLOY / ARCHITECTURE 更新 |

---

# 阶段一：内嵌 NATS 信令事件总线（P0）

> 详细逐步代码以 `docs/superpowers/plans/2026-07-15-embedded-nats-signal-event-bus.md` 为准。  
> 本阶段任务改为 **「完成剩余 + 合入」**，避免重复实现。

### Task 1.1: 盘点 worktree 与主线差异

**Files:**
- Inspect: `.worktrees/codex/embedded-nats-signal-bus/app/server/internal/bus/`
- Inspect: `/Users/noelorin/GOSpeak/app/server/internal/signal/hub.go`

- [ ] **Step 1: 列出 worktree 相对主线新增文件**

```bash
cd /Users/noelorin/GOSpeak
diff -rq app/server .worktrees/codex/embedded-nats-signal-bus/app/server | head -80
git -C .worktrees/codex/embedded-nats-signal-bus status -sb
git -C .worktrees/codex/embedded-nats-signal-bus log --oneline main..HEAD | head -20
```

Expected: `internal/bus/` 仅在 worktree；hub/gin/config/monitor/deploy 有差异；分支上有 `feat(bus|signal|server|config|deploy)` 提交。

- [ ] **Step 2: 记录未提交补丁意图**

worktree 未提交改动应保留：

```go
// localNamespace 仅本机广播，不经 EventBus。
// 用于 room:list:result / 部分 room:updated（携带本机内存成员数快照）。
func (h *Hub) localNamespace(event string, data interface{}) {
	if h.server != nil {
		h.server.BroadcastToNamespace("/", event, data)
	}
}
```

规则：
- **跨实例 fanout**：`member:joined/left/updated`、`room:kicked`、`user:muted/unmuted`、`sfu:provider-changed`、`room:active-speakers`
- **仅本机**：`room:list:result`、带本机 `MemberCount` 的 `room:updated`（阶段二前避免错误计数）

- [ ] **Step 3: Commit worktree 剩余补丁（在 worktree 内）**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/codex/embedded-nats-signal-bus
git add app/server/internal/bus/factory.go app/server/internal/bus/nats_bus.go app/server/internal/signal/hub.go
git commit -m "fix(signal): keep list/updated local until shared membership"
```

---

### Task 1.2: 阶段一回归测试

**Files:**
- Test: `app/server/internal/bus/*_test.go`
- Test: `app/server/internal/signal/hub_event_bus_test.go`

- [ ] **Step 1: 跑 bus + signal 包测试**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/codex/embedded-nats-signal-bus/app/server
go test ./internal/bus ./internal/signal -count=1
```

Expected: PASS

- [ ] **Step 2: 编译**

```bash
go build -o /tmp/gospeak-nats-phase1 .
```

Expected: 成功

- [ ] **Step 3: 单机内嵌冒烟（可选手工）**

```bash
# 不设 NATS_URL
./gospeak-nats-phase1 server
# 日志: [EventBus] embedded nats started: nats://127.0.0.1:xxxxx
# 浏览器进房/禁言与改造前一致
```

---

### Task 1.3: 双实例 fanout 手工验收

**Files:** 无代码；外部 NATS 容器

- [ ] **Step 1: 启动外部 NATS**

```bash
docker run --rm -p 4222:4222 --name gospeak-nats-test nats:2-alpine
```

- [ ] **Step 2: 起两个 gospeak 实例**

```bash
NATS_URL=nats://127.0.0.1:4222 SERVER_PORT=8098 ./gospeak-nats-phase1 server
NATS_URL=nats://127.0.0.1:4222 SERVER_PORT=8099 ./gospeak-nats-phase1 server
```

Expected 日志两侧：
- `external nats probe ok`
- **不应**出现 `embedded nats started`（除非探测失败）

- [ ] **Step 3: 跨实例事件**

客户端 A 连 `:8098`，客户端 B 连 `:8099`，同一房间：
- A 进房 → B 收到 `member:joined`
- A 被禁言 → B 收到 `user:muted`
- A 踢 B → B 收到 `room:kicked` / `member:left`

- [ ] **Step 4: 外部不可用回退**

```bash
NATS_URL=nats://127.0.0.1:59999 SERVER_PORT=8100 ./gospeak-nats-phase1 server
```

Expected：
- `external nats unavailable ... fallback to embedded`
- 进程不退出；health `eventbus_mode=embedded`，`eventbus_fallback_from_external=true`

---

### Task 1.4: 合入主线

**Files:** worktree → 主仓库

- [ ] **Step 1: 在主仓库创建/切换分支并合入**

```bash
cd /Users/noelorin/GOSpeak
git checkout -b codex/nats-multi-phase
git merge --no-ff codex/embedded-nats-signal-bus -m "feat(nats): merge phase-1 embedded signal event bus"
```

若 worktree 分支不在主 git 可见：

```bash
git -C .worktrees/codex/embedded-nats-signal-bus format-patch main --stdout | git am
```

- [ ] **Step 2: 主线再跑测试**

```bash
cd /Users/noelorin/GOSpeak/app/server && go test ./internal/bus ./internal/signal -count=1
```

Expected: PASS

- [ ] **Step 3: 更新阶段一计划 checkbox（可选）**

将 `docs/superpowers/plans/2026-07-15-embedded-nats-signal-event-bus.md` 中已完成 Task 标为 `[x]`，并注明合入 commit。

- [ ] **Step 4: Commit 文档状态**

```bash
git add docs/superpowers/plans/2026-07-15-embedded-nats-signal-event-bus.md docs/superpowers/plans/2026-07-15-nats-multi-phase-optimization.md
git commit -m "docs(plans): mark nats phase-1 status and multi-phase roadmap"
```

**阶段一完成标准：** 单机内嵌可用；外部 NATS 多副本 fanout 可用；监控可见 mode；`room:list:result` 不错误跨实例污染。

---

# 阶段二：在线状态共享（P1）

**Goal:** 多副本下 `room:list` / 参与者视图 / SRS stream 注册表一致。  
**手段：** JetStream KV 存 membership + stream 映射；变更事件通知各实例刷新本地缓存。  
**前提：** 阶段一 EventBus 已合入；外部 NATS 建议开启 JetStream（内嵌阶段二默认也启 JetStream）。

### 设计锁定

| 项 | 决策 |
|----|------|
| 存储 | JetStream KV：`{prefix}_membership`、`{prefix}_stream` |
| 本地缓存 | Hub 仍保留 `rooms` map 作连接本地索引；KV 为跨实例真相源 |
| 写路径 | 本机 join/leave/kick → 更新本地 → Put KV → Publish `state.event` |
| 读路径 | `GetRooms`/`GetRoomMembers` 优先合并本地连接 + KV 远端成员 |
| 冲突 | last-write-wins（KV revision）；identity 级覆盖 |
| 单机内嵌 | embedded nats-server **启用 JetStream**（改 `embedded.go`） |
| 无 JS | 探测失败则 **降级阶段一行为**（仅本地状态），打 Warn |

### membership KV value

```go
// app/server/internal/bus/kv_store.go
type MemberRecord struct {
	Room        string `json:"room"`
	Identity    string `json:"identity"`
	SocketHint  string `json:"socket_hint,omitempty"` // 仅提示，不可跨实例直连
	InstanceID  string `json:"instance_id"`
	Stream      string `json:"stream,omitempty"`
	MicMuted    bool   `json:"mic_muted"`
	Speaking    bool   `json:"speaking"`
	UpdatedAtMS int64  `json:"updated_at_ms"`
}

type RoomMembersSnapshot struct {
	Room      string         `json:"room"`
	Members   []MemberRecord `json:"members"`
	UpdatedAt int64          `json:"updated_at_ms"`
}
```

KV key：
- membership: `room.{roomName}` → `RoomMembersSnapshot`
- stream: `stream.{streamName}` → `{"room":"...","identity":"..."}`

---

### Task 2.1: 内嵌 NATS 启用 JetStream

**Files:**
- Modify: `app/server/internal/bus/embedded.go`
- Modify: `app/server/internal/bus/embedded_test.go`

- [ ] **Step 1: 写失败测试**

```go
// embedded_test.go
func TestStartEmbeddedServer_JetStreamAvailable(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	nc, err := nats.Connect(es.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream context: %v", err)
	}
	if _, err := js.AccountInfo(); err != nil {
		t.Fatalf("JetStream not enabled: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd app/server && go test ./internal/bus -run TestStartEmbeddedServer_JetStreamAvailable -count=1
```

Expected: FAIL（当前 embedded 未开 JS）

- [ ] **Step 3: 实现**

```go
// embedded.go
func StartEmbeddedServer() (*EmbeddedServer, error) {
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // 随机
		JetStream: true,
		StoreDir:  "", // 内存/临时；生产外部 NATS 用持久 store
		NoLog:     true,
		NoSigs:    true,
	}
	// 使用临时目录避免污染 cwd
	dir, err := os.MkdirTemp("", "gospeak-nats-js-*")
	if err != nil {
		return nil, err
	}
	opts.StoreDir = dir

	ns, err := server.NewServer(opts)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("embedded nats not ready")
	}
	return &EmbeddedServer{ns: ns, storeDir: dir}, nil
}

func (e *EmbeddedServer) Shutdown() {
	if e.ns != nil {
		e.ns.Shutdown()
		e.ns.WaitForShutdown()
	}
	if e.storeDir != "" {
		_ = os.RemoveAll(e.storeDir)
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./internal/bus -run TestStartEmbeddedServer_JetStreamAvailable -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/bus/embedded.go app/server/internal/bus/embedded_test.go
git commit -m "feat(bus): enable JetStream on embedded nats-server"
```

---

### Task 2.2: KV Store 封装

**Files:**
- Create: `app/server/internal/bus/kv_store.go`
- Create: `app/server/internal/bus/kv_store_test.go`
- Modify: `app/server/internal/bus/nats_bus.go`（暴露 `JetStream()` 或 `KV()`）

- [ ] **Step 1: 写失败测试**

```go
func TestMembershipKV_PutGet(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenStateStore(StateStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	snap := RoomMembersSnapshot{
		Room: "r1",
		Members: []MemberRecord{{
			Room: "r1", Identity: "alice", InstanceID: "i1", UpdatedAtMS: 1,
		}},
		UpdatedAt: 1,
	}
	if err := store.PutRoomMembers(context.Background(), snap); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetRoomMembers(context.Background(), "r1")
	if err != nil || len(got.Members) != 1 || got.Members[0].Identity != "alice" {
		t.Fatalf("got %+v err=%v", got, err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./internal/bus -run TestMembershipKV_PutGet -count=1
```

Expected: FAIL `OpenStateStore` undefined

- [ ] **Step 3: 最小实现**

```go
// kv_store.go
package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type StateStoreConfig struct {
	URL    string
	Prefix string
	NC     *nats.Conn // 可选：复用已有连接
}

type StateStore struct {
	nc   *nats.Conn
	js   nats.JetStreamContext
	mem  nats.KeyValue
	strm nats.KeyValue
	own  bool // 是否 Close 时关 nc
}

func OpenStateStore(cfg StateStoreConfig) (*StateStore, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	var nc *nats.Conn
	var err error
	own := false
	if cfg.NC != nil {
		nc = cfg.NC
	} else {
		nc, err = nats.Connect(cfg.URL, nats.Name(cfg.Prefix+"-state"))
		if err != nil {
			return nil, err
		}
		own = true
	}
	js, err := nc.JetStream()
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, err
	}
	mem, err := js.CreateOrUpdateKeyValue(&nats.KeyValueConfig{
		Bucket: cfg.Prefix + "_membership",
		TTL:    24 * time.Hour,
	})
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("membership kv: %w", err)
	}
	strm, err := js.CreateOrUpdateKeyValue(&nats.KeyValueConfig{
		Bucket: cfg.Prefix + "_stream",
		TTL:    24 * time.Hour,
	})
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("stream kv: %w", err)
	}
	return &StateStore{nc: nc, js: js, mem: mem, strm: strm, own: own}, nil
}

func (s *StateStore) PutRoomMembers(ctx context.Context, snap RoomMembersSnapshot) error {
	_ = ctx
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = s.mem.Put("room."+sanitizeKey(snap.Room), b)
	return err
}

func (s *StateStore) GetRoomMembers(ctx context.Context, room string) (RoomMembersSnapshot, error) {
	_ = ctx
	entry, err := s.mem.Get("room." + sanitizeKey(room))
	if err != nil {
		return RoomMembersSnapshot{}, err
	}
	var snap RoomMembersSnapshot
	if err := json.Unmarshal(entry.Value(), &snap); err != nil {
		return RoomMembersSnapshot{}, err
	}
	return snap, nil
}

func (s *StateStore) DeleteRoomMembers(ctx context.Context, room string) error {
	_ = ctx
	return s.mem.Delete("room." + sanitizeKey(room))
}

func (s *StateStore) PutStream(ctx context.Context, stream, room, identity string) error {
	_ = ctx
	b, _ := json.Marshal(map[string]string{"room": room, "identity": identity})
	_, err := s.strm.Put("stream."+sanitizeKey(stream), b)
	return err
}

func (s *StateStore) GetStream(ctx context.Context, stream string) (room, identity string, err error) {
	_ = ctx
	entry, err := s.strm.Get("stream." + sanitizeKey(stream))
	if err != nil {
		return "", "", err
	}
	var m map[string]string
	if err := json.Unmarshal(entry.Value(), &m); err != nil {
		return "", "", err
	}
	return m["room"], m["identity"], nil
}

func (s *StateStore) DeleteStream(ctx context.Context, stream string) error {
	_ = ctx
	return s.strm.Delete("stream." + sanitizeKey(stream))
}

func (s *StateStore) Close() error {
	if s.own && s.nc != nil {
		s.nc.Close()
	}
	return nil
}

func sanitizeKey(s string) string {
	// KV key 不允许空格；房间名保守替换
	return strings.ReplaceAll(s, " ", "_")
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./internal/bus -run TestMembershipKV_PutGet -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/bus/kv_store.go app/server/internal/bus/kv_store_test.go
git commit -m "feat(bus): add JetStream KV state store for membership/stream"
```

---

### Task 2.3: Hub 写路径同步 KV

**Files:**
- Modify: `app/server/internal/signal/hub.go`
- Create: `app/server/internal/signal/state_sync.go`
- Create: `app/server/internal/signal/state_sync_test.go`

- [ ] **Step 1: 定义 Hub 依赖接口**

```go
// state_sync.go
type membershipStore interface {
	PutRoomMembers(ctx context.Context, snap bus.RoomMembersSnapshot) error
	GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error)
	DeleteRoomMembers(ctx context.Context, room string) error
	PutStream(ctx context.Context, stream, room, identity string) error
	DeleteStream(ctx context.Context, stream string) error
}
```

- [ ] **Step 2: 写失败测试 — join 后 KV 有成员**

```go
func TestHub_JoinSFU_WritesMembershipKV(t *testing.T) {
	store := &memStateStore{rooms: map[string]bus.RoomMembersSnapshot{}}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store)
	// ... mock server + OnRoomJoinSFU with identity alice room r1
	// assert store has alice in r1
}
```

- [ ] **Step 3: 在 join/leave/kick/disconnect/RegisterStream 后调用 sync**

```go
func (h *Hub) syncRoomToStore(roomName string) {
	if h.membershipStore == nil {
		return
	}
	h.mu.RLock()
	room, ok := h.rooms[roomName]
	if !ok {
		h.mu.RUnlock()
		_ = h.membershipStore.DeleteRoomMembers(context.Background(), roomName)
		return
	}
	snap := bus.RoomMembersSnapshot{Room: roomName, UpdatedAt: time.Now().UnixMilli()}
	for _, m := range room.Members {
		snap.Members = append(snap.Members, bus.MemberRecord{
			Room: roomName, Identity: m.Identity, Stream: m.Stream,
			MicMuted: room.MicMuted[m.Identity], Speaking: room.Speaking[m.Identity],
			InstanceID: h.instanceID, UpdatedAtMS: snap.UpdatedAt,
		})
	}
	h.mu.RUnlock()
	_ = h.membershipStore.PutRoomMembers(context.Background(), snap)
}
```

`RegisterStream` / `UnregisterStream` 同步 `PutStream` / `DeleteStream`。

- [ ] **Step 4: 跑 signal 测试**

```bash
go test ./internal/signal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/signal/hub.go app/server/internal/signal/state_sync.go app/server/internal/signal/state_sync_test.go
git commit -m "feat(signal): sync room membership and streams to JetStream KV"
```

---

### Task 2.4: 读路径合并 + 远端事件刷新

**Files:**
- Modify: `app/server/internal/signal/hub.go`（`GetRooms` / `GetRoomMembers` / `broadcastRoomList`）
- Modify: `app/server/internal/bus/nats_bus.go` 或 factory：订阅 `{p}.state.event`

- [ ] **Step 1: 状态事件信封**

```go
type StateEvent struct {
	Kind string `json:"kind"` // room_members|stream_put|stream_del
	Room string `json:"room,omitempty"`
	// stream fields optional
	Stream   string `json:"stream,omitempty"`
	Identity string `json:"identity,omitempty"`
}
```

Publish subject: `NamespaceSubject` 复用或专用 `{prefix}.state.event`（推荐专用，避免进 Socket.IO）。

- [ ] **Step 2: 写测试 — 仅 KV 有远端成员时 GetRoomMembers 能读到**

```go
func TestHub_GetRoomMembers_MergesKV(t *testing.T) {
	store := &memStateStore{rooms: map[string]bus.RoomMembersSnapshot{
		"r1": {Room: "r1", Members: []bus.MemberRecord{{Identity: "bob", InstanceID: "other"}}},
	}}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store)
	// 本地无 bob 连接
	members := hub.GetRoomMembersMerged("r1")
	if len(members) != 1 || members[0].Identity != "bob" {
		t.Fatalf("%+v", members)
	}
}
```

- [ ] **Step 3: `room:updated` / `room:list:result` 改用合并视图后，可重新走 `publishNamespace`**

阶段一的 `localNamespace` 限制在阶段二解除：

```go
func (h *Hub) broadcastRoomList() {
	rooms := h.GetSFURoomsMerged() // 本地活跃 + KV
	h.publishNamespace(EventRoomListResult, map[string]interface{}{"rooms": rooms})
}
```

- [ ] **Step 4: 订阅 state.event，远端变更时刷新本地 shadow cache（非 socket 成员）**

```go
// shadow 仅用于列表展示；真正的 socket 连接仍只在本机 Members
type shadowMember struct {
	Identity   string
	InstanceID string
	Stream     string
}
```

- [ ] **Step 5: 测试 + Commit**

```bash
go test ./internal/bus ./internal/signal -count=1
git add app/server/internal/signal app/server/internal/bus
git commit -m "feat(signal): merge KV membership into room list and members API"
```

---

### Task 2.5: 接线 gin + 部署说明

**Files:**
- Modify: `app/server/server/gin.go`
- Modify: `deploy/DEPLOY.md`、`ARCHITECTURE.md`
- Modify: `deploy/docker-compose.yml`（外部 nats 加 `-js`）

- [ ] **Step 1: compose 外部 NATS 开 JetStream**

```yaml
nats:
  profiles: ["nats"]
  image: nats:2-alpine
  command: ["-js", "-m", "8222"]
  ports:
    - "4222:4222"
    - "8222:8222"
```

- [ ] **Step 2: gin 在 bus.Init 后 OpenStateStore，注入 Hub**

```go
if nb, ok := eventBus.(*bus.NATSBus); ok {
	// 复用 NATSBus 连接打开 KV；若 API 未暴露 nc，则用 URL 再连或给 NATSBus 增加 JS() 方法
	store, err := bus.OpenStateStore(bus.StateStoreConfig{URL: /* from bus */, Prefix: cfg.NATSSubjectPrefix})
	if err != nil {
		log.Printf("[EventBus] state store unavailable: %v", err)
	} else {
		signalHub.SetMembershipStore(store)
	}
}
```

推荐在 `NATSBus` 增加：

```go
func (b *NATSBus) Conn() *nats.Conn { return b.nc }
```

- [ ] **Step 3: 双实例验收**

A join room → B 的 `room:list` 显示成员数 ≥1（即使 B 无本地 socket）。

- [ ] **Step 4: Commit**

```bash
git add app/server/server/gin.go deploy docs
git commit -m "feat(deploy): enable JetStream and wire membership state store"
```

**阶段二完成标准：** 两实例房间列表成员一致；SRS stream 注册跨实例可查；无 JS 时 Warn 降级单机。

---

# 阶段三：可靠异步 Webhook / SFU Cleanup（P2）

**Goal:** webhook 快速 ACK + 至少一次处理；disconnect SFU 清理可重试，不死挂在 `go func`。

### 设计锁定

| 项 | 决策 |
|----|------|
| 队列 | JetStream WorkQueue 或 interest + durable consumer |
| Stream | `{prefix}_jobs`，subjects：`{p}.jobs.>` |
| 幂等 | 消息带 `dedupe_id`（stream 名 / participant key）；handler 可重入 |
| SRS | HTTP 鉴权通过后 Publish，再 200；consumer 调 `RegisterStream` |
| LiveKit | 入队后 consumer 解析 event，至少处理 `participant_left` → 对齐信令 |
| Cleanup | `OnDisconnect`/`kick` 发 `jobs.sfu.cleanup`，worker 调 `removeParticipantSafe` |
| 降级 | 无 JS：保持现有同步/goroutine 路径 |

### Job payload

```go
type JobEnvelope struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"` // srs_publish|srs_unpublish|livekit|sfu_cleanup
	Payload json.RawMessage `json:"payload"`
	TS     int64           `json:"ts"`
}

type SFUCleanupJob struct {
	Room     string `json:"room"`
	Identity string `json:"identity"`
	DeleteRoom bool `json:"delete_room"`
}
```

---

### Task 3.1: JetStream Queue helper

**Files:**
- Create: `app/server/internal/bus/queue.go`
- Create: `app/server/internal/bus/queue_test.go`

- [ ] **Step 1: 测试 publish + consume ack**

```go
func TestJobQueue_PublishConsume(t *testing.T) {
	es, _ := StartEmbeddedServer()
	t.Cleanup(es.Shutdown)
	q, err := OpenJobQueue(JobQueueConfig{URL: es.ClientURL(), Prefix: "gospeak"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })

	done := make(chan struct{})
	_, err = q.Consume("worker-1", func(job JobEnvelope) error {
		if job.Type != "sfu_cleanup" {
			t.Fatalf("type %s", job.Type)
		}
		close(done)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Publish(context.Background(), JobEnvelope{
		ID: "1", Type: "sfu_cleanup", Payload: json.RawMessage(`{"room":"r","identity":"u"}`),
		TS: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}
```

- [ ] **Step 2: 实现 `OpenJobQueue`**

```go
func OpenJobQueue(cfg JobQueueConfig) (*JobQueue, error) {
	// Connect, JetStream, AddStream:
	// StreamConfig{Name: prefix+"_jobs", Subjects: []string{prefix+".jobs.>"}, Retention: nats.WorkQueuePolicy}
	// Publish to prefix+".jobs."+type
	// QueueSubscribe / Subscribe with durable + ManualAck
}
```

失败重试：`Nak()` 或 `InProgress()`；成功 `Ack()`。最大投递次数字段 `MaxDeliver: 5`。

- [ ] **Step 3: 测试 PASS + Commit**

```bash
go test ./internal/bus -run TestJobQueue -count=1
git add app/server/internal/bus/queue.go app/server/internal/bus/queue_test.go
git commit -m "feat(bus): add JetStream work-queue for async jobs"
```

---

### Task 3.2: SRS callback 入队

**Files:**
- Modify: `app/server/internal/handler/srs_callback_handler.go`
- Create: `app/server/internal/handler/srs_callback_handler_queue_test.go`
- Create: `app/server/internal/jobs/srs_consumer.go`

- [ ] **Step 1: Handler 依赖接口**

```go
type streamJobPublisher interface {
	PublishSRS(ctx context.Context, action, stream string) error
}
```

- [ ] **Step 2: on_publish 鉴权成功后**

```go
case "on_publish":
	// validate token...
	if h.jobs != nil {
		_ = h.jobs.PublishSRS(c.Request.Context(), "on_publish", stream)
	} else {
		h.hub.RegisterStream(stream) // 降级同步
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
```

- [ ] **Step 3: Consumer**

```go
// jobs/srs_consumer.go
func HandleSRSJob(hub *signal.Hub, action, stream string) {
	switch action {
	case "on_publish":
		hub.RegisterStream(stream)
	case "on_unpublish":
		hub.UnregisterStream(stream)
	}
}
```

- [ ] **Step 4: 测试 — publisher 被调用；无 jobs 时同步路径仍绿**

```bash
go test ./internal/handler -run SrsCallback -count=1
```

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/handler/srs_callback_handler.go app/server/internal/jobs
git commit -m "feat(srs): enqueue stream register jobs via JetStream"
```

---

### Task 3.3: LiveKit webhook 真正处理

**Files:**
- Modify: `app/server/internal/handler/signal_handler.go`
- Create: `app/server/internal/jobs/livekit_consumer.go`
- Modify: `app/server/internal/handler/signal_handler_test.go`

- [ ] **Step 1: 入队代替纯 log**

```go
func (h *SignalHandler) LivekitWebhook(c *gin.Context) {
	var event map[string]interface{}
	if err := c.ShouldBindJSON(&event); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if h.jobs != nil {
		b, _ := json.Marshal(event)
		_ = h.jobs.PublishLiveKit(c.Request.Context(), b)
	} else {
		log.Printf("[Webhook] livekit event (sync no-queue): %v", event["event"])
	}
	pkg.Success(c, "ok")
}
```

- [ ] **Step 2: Consumer 最小处理**

```go
// participant_left → 可选：若信令仍显示在线则清理 shadow / 触发 leave 对齐
// room_finished → deleteRoomSafe
func HandleLiveKitEvent(hub *signal.Hub, sfu sfu.Provider, raw []byte) error {
	var event struct {
		Event string `json:"event"`
		// 按 LiveKit webhook schema 扩展
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return err
	}
	switch event.Event {
	case "participant_left", "participant_disconnected":
		// 解析 room + identity 后 hub.ForceRemoveIdentity(room, identity)（新增幂等方法）
	case "room_finished":
		// hub 清理空房
	default:
		// ignore
	}
	return nil
}
```

- [ ] **Step 3: 增加 `Hub.ForceRemoveIdentity` 幂等清理（无 socket 时只清 shadow/KV）**

- [ ] **Step 4: 测试 + Commit**

```bash
go test ./internal/handler -run LivekitWebhook -count=1
git commit -m "feat(livekit): process webhooks via job queue with hub reconcile"
```

---

### Task 3.4: SFU cleanup 入队替换裸 goroutine

**Files:**
- Modify: `app/server/internal/signal/hub.go`（`OnDisconnect` 等）
- Create: `app/server/internal/jobs/sfu_cleanup_consumer.go`

- [ ] **Step 1: 接口**

```go
type sfuCleanupPublisher interface {
	PublishSFUCleanup(ctx context.Context, room, identity string, deleteRoom bool) error
}
```

- [ ] **Step 2: 替换**

```go
// OnDisconnect 原:
// go func(cleanups) { removeParticipantSafe... }()
// 新:
if h.cleanupPub != nil {
	for _, c := range cleanups {
		_ = h.cleanupPub.PublishSFUCleanup(context.Background(), c.room, c.identity, c.deleted)
	}
} else {
	go func(cleanups []disconnectCleanup) { /* 原逻辑降级 */ }(cleanups)
}
```

- [ ] **Step 3: Consumer 调 `removeParticipantSafe` + `participantCleanup` + `deleteRoomSafe`**

- [ ] **Step 4: 测试 — publisher 记录调用；无 publisher 时旧路径仍工作**

```bash
go test ./internal/signal -count=1
git commit -m "feat(signal): enqueue SFU participant cleanup jobs"
```

**阶段三完成标准：** webhook 快速 200；进程重启后未 ack 任务可再投递；cleanup 失败可重试；无 JS 降级不破坏单机。

---

# 阶段四：MediaSoup 事件 + 缓存失效（P3）

**Goal:** worker 状态变化主动推送；权限缓存多副本一致。HTTP bridge 保留为默认。

### Task 4.1: 权限缓存失效总线

**Files:**
- Modify: `app/server/internal/service/permission_service.go`
- Modify: `app/server/internal/handler` 中 sync 权限成功路径
- Create: `app/server/internal/service/permission_service_bus_test.go`

- [ ] **Step 1: PermissionService 增加**

```go
type cacheBus interface {
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
}

const EventPermissionsInvalidated = "cache:permissions-invalidated"

func (s *PermissionService) SetEventBus(b cacheBus) { s.bus = b }

func (s *PermissionService) SyncRolePermissions(roleName string, permCodes []string) error {
	if err := s.permRepo.SyncRolePermissions(roleName, permCodes); err != nil {
		return err
	}
	if err := s.LoadCache(); err != nil {
		return err
	}
	if s.bus != nil {
		_ = s.bus.PublishNamespace(context.Background(), EventPermissionsInvalidated, map[string]string{
			"role": roleName,
		})
	}
	return nil
}

func (s *PermissionService) OnRemoteInvalidate(payload interface{}) {
	_ = s.LoadCache()
}
```

- [ ] **Step 2: gin 启动后**

```go
if nb, ok := eventBus.(*bus.NATSBus); ok {
	nb.SetRemoteHook(func(event string, payload interface{}) {
		if event == service.EventPermissionsInvalidated {
			permSvc.OnRemoteInvalidate(payload)
		}
	})
}
permSvc.SetEventBus(eventBus)
```

注意：`SetRemoteHook` 已在阶段一 worktree 的 `NATSBus` 存在；若合入时缺失需补回。  
`PublishNamespace` 会本地投递到 Socket.IO — **cache 事件不应进前端**。

因此阶段四应扩展 EventBus：

```go
// 内部事件，不经 Deliverer，仅 NATS + RemoteHook
PublishInternal(ctx context.Context, event string, payload interface{}) error
```

- [ ] **Step 3: 实现 `PublishInternal` + 测试两实例 cache 刷新**

```bash
go test ./internal/bus ./internal/service -count=1
git commit -m "feat(rbac): broadcast permission cache invalidation over NATS"
```

---

### Task 4.2: MediaSoup worker 主动事件（可选增强）

**Files:**
- Modify: `packages/mediasoup-worker/src/worker.ts`
- Modify: `packages/mediasoup-worker/src/index.ts`
- Create: `packages/mediasoup-worker/src/nats.ts`（可选依赖 `nats` npm）
- Modify: `app/server/internal/mediasoup/signal.go`

- [ ] **Step 1: worker 环境变量**

```bash
NATS_URL=nats://nats:4222
NATS_SUBJECT_PREFIX=gospeak
```

未设置则不连 NATS（纯 HTTP，与现网一致）。

- [ ] **Step 2: producer close / transport close 时 publish**

```ts
// nats.ts
import { connect, StringCodec } from "nats";

export async function connectBus(url: string, prefix: string) {
  const nc = await connect({ servers: url });
  const sc = StringCodec();
  return {
    publish(event: string, payload: unknown) {
      const env = {
        v: 1,
        instance_id: process.env.HOSTNAME ?? "mediasoup",
        scope: "mediasoup",
        event,
        payload: JSON.stringify(payload),
        ts: Date.now(),
      };
      nc.publish(`${prefix}.mediasoup.event`, sc.encode(JSON.stringify(env)));
    },
    close: () => nc.close(),
  };
}
```

- [ ] **Step 3: Go 侧订阅 `mediasoup.event`，转 `BroadcastToRoom`（producer-closed）**

可减少 leave 路径上冗余 HTTP 轮询（见现有 `MediasoupSignal.recentClose` 注释）。

- [ ] **Step 4: 文档说明 HTTP 仍为控制面默认；NATS 事件为增强**

```bash
# packages
pnpm --filter @gospeak/mediasoup-worker build
git commit -m "feat(mediasoup): optional NATS event publish on producer/transport close"
```

---

### Task 4.3: MediaSoup request/reply 控制面（仅多 worker 需要时）

**Files:**
- Create: `app/server/internal/mediasoup/nats_transport.go`
- Modify: `app/server/internal/mediasoup/bridge.go`（抽 interface）

- [ ] **Step 1: 抽象**

```go
type Transport interface {
	Do(ctx context.Context, method, path string, body io.Reader, out interface{}) error
}
```

HTTP 实现 = 现 `BridgeClient.do`；NATS 实现 = request subject `prefix.mediasoup.cmd`，worker queue group 消费。

- [ ] **Step 2: 默认仍 HTTP；`MEDIASOUP_TRANSPORT=nats` 才切换**

- [ ] **Step 3: 集成测试用 embedded NATS 模拟 worker reply**

```bash
go test ./internal/mediasoup -count=1
git commit -m "feat(mediasoup): optional NATS request/reply transport for multi-worker"
```

**阶段四完成标准：** 权限变更多副本秒级生效；MediaSoup 可选事件推送；控制面 NATS 为可选，默认 HTTP 不回归。

---

# 跨阶段验收矩阵

| 场景 | 阶段一 | 阶段二 | 阶段三 | 阶段四 |
|------|--------|--------|--------|--------|
| 单机无外部依赖 | ✅ 内嵌 | ✅ 内嵌+JS | ✅ | ✅ |
| 双实例 member:joined | ✅ | ✅ | ✅ | ✅ |
| 双实例 room:list 人数 | ❌ 本机偏 | ✅ | ✅ | ✅ |
| SRS 回调打到实例 B | 仅 B 有 stream | KV 全可见 | 队列消费 | ✅ |
| LiveKit webhook | log only | log only | 处理+重试 | ✅ |
| disconnect SFU 清理 | go func | go func | 队列 | ✅ |
| 权限 sync 跨实例 | ❌ | ❌ | ❌ | ✅ |
| 媒体 WebRTC | 不经 NATS | 不经 NATS | 不经 NATS | 不经 NATS |

---

# 推荐执行顺序与分支策略

```text
codex/nats-multi-phase
  ├─ merge phase-1 worktree          (Task 1.x)
  ├─ feat/nats-phase-2-state-kv      (Task 2.x)
  ├─ feat/nats-phase-3-jobs          (Task 3.x)
  └─ feat/nats-phase-4-cache-ms      (Task 4.x)
```

每阶段结束必须：
1. `go test ./internal/bus ./internal/signal ./internal/handler ./internal/service -count=1`
2. 更新 `ARCHITECTURE.md` 一小节
3. 独立可运行（后阶段失败可降级到前阶段行为）

---

# Self-Review

### Spec coverage

| 需求 | 任务 |
|------|------|
| 跨实例信令 fanout | 阶段一 Task 1.x（worktree + 合入） |
| room:list 不错误 fanout | Task 1.1 `localNamespace`；阶段二解除 |
| 在线成员/stream 共享 | 阶段二 Task 2.2–2.4 |
| SRS/LiveKit 可靠处理 | 阶段三 Task 3.2–3.3 |
| SFU cleanup 重试 | 阶段三 Task 3.4 |
| 权限缓存失效 | 阶段四 Task 4.1 |
| MediaSoup 事件/多 worker | 阶段四 Task 4.2–4.3 |
| 单机默认零依赖 | 内嵌 NATS；JS 在内嵌启用；失败降级 |
| 不碰媒体面 | 全文约束 |

### Placeholder scan

无 TBD/TODO；关键类型与 subject 已写死。阶段一细节指向既有完整计划文件，避免双份漂移。

### Type consistency

- `EventBus` / `Envelope` / `NATSBus` / `StateStore` / `JobQueue` / `JobEnvelope` 命名统一
- subject 前缀默认 `gospeak`
- mode：`embedded` | `external`
- 降级路径均保留无 NATS/JS 时的旧行为

### 与阶段一计划关系

- 阶段一 **实现细节** 以 `2026-07-15-embedded-nats-signal-event-bus.md` 为准
- 本文件是 **全阶段路线图 + 阶段一收尾 + 二/三/四可执行任务**
- 主工作区若尚未合入 bus，**必须先完成 Task 1.x**

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-15-nats-multi-phase-optimization.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — 每个 Task 派生子代理，Task 间审查，迭代快  
2. **Inline Execution** — 本会话按 executing-plans 连续执行，设检查点  

**建议起点：** Task 1.1（收尾 worktree 阶段一并合入），再进阶段二。

**Which approach?**
