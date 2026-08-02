# Cluster Control Plane Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 GOSpeak 集群控制面剩余工作：前端按 `workerUrl` 路由到目标 Worker、Agent 独占写面、NATS 控制命令、状态对账、部署/管理页、监控指标，并为 Phase 6 高可用保留扩展点。

**Architecture:** Agent 作为唯一权威控制面，Worker 只承载信令/SFU 数据面。现有 `internal/cluster`、`bus`、`signal.Hub`、`router` 继续复用；控制命令通过 NATS internal 事件投递到目标 Worker，Worker 收到后执行本地 Hub 操作；Agent 启动时对 DB 中的节点、Server 分配和离线节点做对账恢复。

**Tech Stack:** Go 1.26、Gin、GORM、NATS JetStream、nhooyr WebSocket、SolidJS、TypeScript、Vite、Vitest、Docker Compose。

---

## 当前状态与剩余范围

已实现：

- `GOSPEAK_ROLE=agent|worker|all`
- `internal/cluster`、`cluster_nodes`、`server_assignments`
- 节点注册/心跳/离线回收、Server 扩缩容、drain/undrain
- `/api/v1/cluster/*` 控制面 API
- join token 返回 `workerUrl`
- `deploy/docker-compose.yml` 的 `cluster` profile 基础服务

剩余未完成：

- 前端 socket 尚未按 `workerUrl` 连接目标 Worker
- Worker 仍不是严格只读 DB，启动仍会执行迁移/种子
- Agent 尚未向 Worker 下发 kick/mute/room delete/domain delete 控制命令
- Agent 重启后没有统一对账恢复
- 缺少 Nginx 集群路由、管理页、集群健康指标
- Phase 6 主备/自动扩缩/滚动灰度仍只有高层描述

## 文件结构

| 文件 | 职责 |
|---|---|
| `app/web/src/stores/socketStore.ts` | 增加按 worker URL 连接/断开/重连能力 |
| `app/web/src/components/room/session/runVoiceJoin.ts` | 进房时先连接目标 Worker，再执行信令 join |
| `app/web/src/components/room/hooks/useVoiceSession.ts` | 将 token.workerUrl 传入进房编排 |
| `app/server/internal/config/config.go` | 增加 Worker 只读启动配置与校验 |
| `app/server/internal/repository/db.go` | Worker 模式跳过迁移/种子，SQLite 使用只读连接 |
| `app/server/internal/cluster/control.go` | 控制命令类型、校验、发布辅助函数 |
| `app/server/internal/signal/hub.go` | 新增 `HandleClusterCommand` 执行目标节点本地控制操作 |
| `app/server/server/gin.go` | Agent/Worker remote hook 接入控制命令、对账启动 |
| `app/server/internal/service/cluster_service.go` | 增加 `Stats()`、`ReconcileAll()` |
| `app/server/internal/handler/monitor_handler.go` | 增加集群健康指标 |
| `app/server/internal/handler/cluster_handler.go` | 增加集群统计/对账 API |
| `app/server/internal/router/routes/cluster/routes.go` | 注册统计/对账 API |
| `app/web/src/api/cluster.ts` | 前端集群 API 类型与请求 |
| `app/web/src/pages/(app)/cluster/index.tsx` | 集群管理页 |
| `deploy/nginx-cluster.conf` | Agent/Worker 反代 |
| `deploy/docker-compose.yml` | 集群 profile 增加 nginx-cluster |
| `docs/cluster-agent-worker-plan.md` | 更新状态与验收说明 |

---

## Task 1: 前端按 workerUrl 连接目标 Worker

### Task 1.1: 给 socketStore 增加目标 Worker 连接

**Files:**
- Modify: `app/web/src/stores/socketStore.ts`
- Test: `app/web/src/stores/socketStore.test.ts`

- [ ] **Step 1: 写失败测试**

在 `app/web/src/stores/socketStore.test.ts` 中增加：

```ts
import { socketStore } from "./socketStore";

it("connectToWorker uses worker URL before default socket URL", async () => {
  const url = socketStore.connectToWorker("wss://worker-a.example");
  expect(url).toBe("wss://worker-a.example");
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @gospeak/web exec vitest run src/stores/socketStore.test.ts -t connectToWorker`
Expected: FAIL，`connectToWorker` 不存在。

- [ ] **Step 3: 实现 connectToWorker**

在 `app/web/src/stores/socketStore.ts` 的 `connect()` 函数前增加：

```ts
function connectToWorker(workerUrl: string) {
  if (adapter.isConnected()) {
    const current = adapter.getCurrentUrl?.();
    if (current === workerUrl) return current;
    adapter.disconnect();
  }
  const ticket = getWSTicket();
  adapter.connect(workerUrl, ticket);
  setConnecting(false);
  return workerUrl;
}
```

并在 `createWSClient` 返回类型中暴露 `getCurrentUrl?: () => string`，实现为：

```ts
getCurrentUrl: () => currentUrl,
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @gospeak/web exec vitest run src/stores/socketStore.test.ts -t connectToWorker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/web/src/stores/socketStore.ts app/web/src/stores/socketStore.test.ts app/web/src/socket/wsClient.ts
git commit -m "feat(web): support worker URL socket connection"
```

### Task 1.2: 进房编排先连接目标 Worker

**Files:**
- Modify: `app/web/src/components/room/session/runVoiceJoin.ts`
- Modify: `app/web/src/components/room/hooks/useVoiceSession.ts`
- Test: `app/web/src/components/room/session/runVoiceJoin.test.ts`

- [ ] **Step 1: 写失败测试**

在 `runVoiceJoin.test.ts` 增加：

```ts
it("connects to workerUrl before signal join", async () => {
  const connectSignal = vi.fn(async () => {});
  const joinSignalRoom = vi.fn(async () => {});
  const { deps } = makeDeps({ connectSignal, joinSignalRoom });
  await runVoiceJoin(makeToken({ workerUrl: "wss://worker-a.example" }) as any, deps as any);
  expect(connectSignal).toHaveBeenCalledWith("wss://worker-a.example");
  expect(joinSignalRoom).toHaveBeenCalled();
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @gospeak/web exec vitest run src/components/room/session/runVoiceJoin.test.ts -t "workerUrl"`
Expected: FAIL，`connectSignal` 未被调用。

- [ ] **Step 3: 修改 VoiceJoinDeps**

在 `runVoiceJoin.ts` 的 `VoiceJoinDeps` 中增加：

```ts
connectSignal?: (workerUrl: string) => Promise<unknown>;
```

- [ ] **Step 4: 在 executeVoiceJoin 中调用**

在 `executeVoiceJoin` 的 `joinSignalRoom` 调用前增加：

```ts
if (token.workerUrl && deps.connectSignal) {
  await deps.connectSignal(token.workerUrl);
}
```

- [ ] **Step 5: 在 useVoiceSession 中注入**

在 `app/web/src/components/room/hooks/useVoiceSession.ts` 的 `runVoiceJoin` 参数中增加：

```ts
connectSignal: (workerUrl) => socketStore.connectToWorker(workerUrl),
```

- [ ] **Step 6: 运行测试确认通过**

Run: `pnpm --filter @gospeak/web exec vitest run src/components/room/session/runVoiceJoin.test.ts -t "workerUrl"`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add app/web/src/components/room/session/runVoiceJoin.ts app/web/src/components/room/hooks/useVoiceSession.ts app/web/src/components/room/session/runVoiceJoin.test.ts
git commit -m "feat(web): route voice signal join to assigned worker"
```

---

## Task 2: Worker 写面隔离与只读启动

### Task 2.1: Worker 模式跳过迁移和种子

**Files:**
- Modify: `app/server/internal/repository/db.go`
- Modify: `app/server/server/gin.go`
- Test: `app/server/internal/repository/db_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/repository/db_test.go` 增加：

```go
func TestSQLiteReadOnlyWorkerDB(t *testing.T) {
	cfg := &config.Config{
		DBType:     "SQLite",
		DBPath:     filepath.Join(t.TempDir(), "worker.db"),
		ClusterRole: "worker",
	}
	if err := InitDB(cfg); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if err := DB.Exec("CREATE TABLE forbidden (id INTEGER)").Error; err == nil {
		t.Fatal("expected read-only SQLite write to fail")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/repository -run TestSQLiteReadOnlyWorkerDB -v`
Expected: FAIL，当前 Worker 连接仍可写。

- [ ] **Step 3: 修改 connectSQLite 支持 Worker 只读**

在 `app/server/internal/repository/db.go` 的 `connectSQLite` 开头增加：

```go
if cfg.ClusterRole == model.ClusterRoleWorker {
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(10000)", path)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
```

- [ ] **Step 4: Worker 模式跳过 AutoMigrate 与业务种子**

在 `server/gin.go` 中把以下启动逻辑改为只在 `cfg.IsAgent()` 时执行：`seedRoles`、`seedAdminUser`、`EnsureDefaultDomain`、`seedPermissions`、`sfuConfigSvc.SyncFromEnv`、`pluginReg.InitAll/StartEnabled`。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/repository -run TestSQLiteReadOnlyWorkerDB -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/repository/db.go app/server/server/gin.go app/server/internal/repository/db_test.go
git commit -m "feat(cluster): worker mode uses read-only DB and skips seeding"
```

### Task 2.2: Worker 路由显式拒绝业务写接口

**Files:**
- Modify: `app/server/internal/router/router.go`
- Test: `app/server/internal/router/router_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/router/router_test.go` 增加：

```go
func testHandlers() *Handlers {
	return &Handlers{}
}

func TestWorkerModeDoesNotRegisterBusinessWrites(t *testing.T) {
	config.SetCurrent(&config.Config{ClusterRole: model.ClusterRoleWorker})
	r := gin.New()
	SetupRoutes(r, testHandlers())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/room/create", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for worker write route, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/router -run TestWorkerModeDoesNotRegisterBusinessWrites -v`
Expected: FAIL，因为测试 helper 未实现，或路由未按预期注册。

- [ ] **Step 3: 在 router.go 增加显式拒绝中间件**

在 `SetupRoutes` 的 worker 分支中增加：

```go
workerProtected.POST("/room/create", func(c *gin.Context) {
	pkg.Fail(c, pkg.FORBIDDEN, "worker mode does not accept business writes")
	c.Abort()
})
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/router -run TestWorkerModeDoesNotRegisterBusinessWrites -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/router/router.go app/server/internal/router/router_test.go
git commit -m "feat(cluster): explicitly reject worker business writes"
```

---

## Task 3: NATS 控制命令

### Task 3.1: 定义控制命令类型

**Files:**
- Create: `app/server/internal/cluster/control.go`
- Test: `app/server/internal/cluster/control_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/cluster/control_test.go` 增加：

```go
func TestControlCommandValidate(t *testing.T) {
	cmd := ControlCommand{Command: CommandKick, NodeID: "node-a", Room: "lobby", Identity: "alice"}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("expected valid command, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/cluster -run TestControlCommandValidate -v`
Expected: FAIL，`ControlCommand` 未定义。

- [ ] **Step 3: 创建 control.go**

```go
package cluster

import "errors"

const (
	CommandKick        = "kick"
	CommandMute        = "mute"
	CommandUnmute      = "unmute"
	CommandDeleteRoom  = "delete_room"
	CommandDeleteServer = "delete_server"
)

type ControlCommand struct {
	Command    string                 `json:"command"`
	NodeID     string                 `json:"node_id"`
	DomainUUID string                 `json:"domain_uuid,omitempty"`
	Room       string                 `json:"room,omitempty"`
	Identity   string                 `json:"identity,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

func (c ControlCommand) Validate() error {
	if c.Command == "" {
		return errors.New("command is required")
	}
	if c.NodeID == "" {
		return errors.New("node_id is required")
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/cluster -run TestControlCommandValidate -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/cluster/control.go app/server/internal/cluster/control_test.go
git commit -m "feat(cluster): define NATS control command envelope"
```

### Task 3.2: Agent 发布控制命令

**Files:**
- Modify: `app/server/internal/service/cluster_service.go`
- Modify: `app/server/internal/handler/mute_handler.go`
- Modify: `app/server/internal/handler/room_handler.go`
- Test: `app/server/internal/service/cluster_service_test.go`

- [ ] **Step 1: 写失败测试**

在 `cluster_service_test.go` 增加：

```go
type fakeClusterNotifier struct {
	event   string
	payload interface{}
}

func (f *fakeClusterNotifier) PublishInternal(_ context.Context, event string, payload interface{}) error {
	f.event = event
	f.payload = payload
	return nil
}

func TestClusterServicePublishControlCommand(t *testing.T) {
	svc, _ := setupClusterServiceTestDB(t)
	notifier := &fakeClusterNotifier{}
	svc.SetNotifier(notifier)
	err := svc.PublishControl(cluster.ControlCommand{Command: cluster.CommandKick, NodeID: "node-a"})
	if err != nil {
		t.Fatalf("PublishControl: %v", err)
	}
	if notifier.event != cluster.EventControlCommand {
		t.Fatalf("expected control event, got %q", notifier.event)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/service -run TestClusterServicePublishControlCommand -v`
Expected: FAIL，`PublishControl` 未定义。

- [ ] **Step 3: 实现 PublishControl**

在 `cluster_service.go` 增加：

```go
func (s *ClusterService) PublishControl(cmd cluster.ControlCommand) error {
	if err := cmd.Validate(); err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}
	s.publishClusterEvent(cluster.EventControlCommand, cmd)
	return nil
}
```

在 `cluster/events.go` 增加：

```go
EventControlCommand = "cluster.control.command"
```

- [ ] **Step 4: 在 mute/room/domain handler 调用**

在 `mute_handler.go` 中注入控制面发布器并调用：

```go
type ControlPublisher interface {
	PublishControl(cluster.ControlCommand) error
}

func (h *MuteHandler) SetControlPublisher(p ControlPublisher) {
	h.controlPublisher = p
}

// 在 CreateMute 成功分支末尾追加：
if h.controlPublisher != nil {
	_ = h.controlPublisher.PublishControl(cluster.ControlCommand{
		Command: cluster.CommandMute, Identity: username, Payload: map[string]interface{}{"user_id": req.UserID},
	})
}
```

在 `room_handler.go` 的 `Delete` 成功分支追加：

```go
if h.controlPublisher != nil {
	_ = h.controlPublisher.PublishControl(cluster.ControlCommand{
		Command: cluster.CommandDeleteRoom, DomainUUID: room.DomainUUID, Room: room.Name,
	})
}
```

在 `domain_handler.go` 的 `Delete` 成功分支追加：

```go
if h.controlPublisher != nil {
	_ = h.controlPublisher.PublishControl(cluster.ControlCommand{
		Command: cluster.CommandDeleteServer, DomainUUID: req.DomainUUID,
	})
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service -run TestClusterServicePublishControlCommand -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/service/cluster_service.go app/server/internal/service/cluster_service_test.go app/server/internal/handler/mute_handler.go app/server/internal/handler/room_handler.go app/server/internal/handler/domain_handler.go app/server/internal/cluster/events.go
git commit -m "feat(cluster): publish control commands from agent"
```

### Task 3.3: Worker 执行控制命令

**Files:**
- Modify: `app/server/internal/signal/hub.go`
- Modify: `app/server/server/gin.go`
- Test: `app/server/internal/signal/hub_control_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/signal/hub_control_test.go` 增加：

```go
func TestHubHandleClusterCommandKick(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: cluster.CommandKick, Room: "lobby", Identity: "alice"}
	if err := hub.HandleClusterCommand(cmd); err != nil {
		t.Fatalf("HandleClusterCommand: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/signal -run TestHubHandleClusterCommandKick -v`
Expected: FAIL，`HandleClusterCommand` 未定义。

- [ ] **Step 3: 实现 HandleClusterCommand**

在 `hub.go` 增加：

```go
func (h *Hub) HandleClusterCommand(cmd cluster.ControlCommand) error {
	switch cmd.Command {
	case cluster.CommandKick:
		h.KickFromRoom(cmd.DomainUUID, cmd.Room, cmd.Identity)
		return nil
	case cluster.CommandDeleteRoom:
		h.DeleteRoomByDomainName(cmd.DomainUUID, cmd.Room)
		return nil
	case cluster.CommandDeleteServer:
		h.OnDomainDelete(cmd.DomainUUID)
		return nil
	default:
		return fmt.Errorf("unsupported cluster command %q", cmd.Command)
	}
}

func (h *Hub) KickFromRoom(domainUUID, room, targetIdentity string) {
	if targetIdentity == "" {
		return
	}
	h.mu.Lock()
	key := roomKey(domainUUID, room)
	roomState, ok := h.rooms[key]
	if !ok {
		h.mu.Unlock()
		return
	}
	for sid, member := range roomState.Members {
		if member == nil || member.Identity != targetIdentity {
			continue
		}
		delete(roomState.Members, sid)
		delete(h.connSlots, sid)
		delete(roomState.Speaking, targetIdentity)
		delete(roomState.MicMuted, targetIdentity)
		if h.fanout != nil {
			h.fanout.Leave(key, sid)
		}
		break
	}
	h.mu.Unlock()
	h.removeParticipantSafe(room, targetIdentity)
	h.syncRoomToStore(key)
}

func (h *Hub) DeleteRoomByDomainName(domainUUID, room string) {
	key := roomKey(domainUUID, room)
	h.mu.Lock()
	roomState, ok := h.rooms[key]
	if !ok {
		h.mu.Unlock()
		return
	}
	for sid, member := range roomState.Members {
		if member == nil {
			continue
		}
		h.unregisterStreamLocked(key, member.Identity, member.Stream)
		if h.fanout != nil {
			h.fanout.Leave(key, sid)
		}
		delete(h.connSlots, sid)
	}
	delete(h.rooms, key)
	h.mu.Unlock()
	h.deleteRoomSafe(room)
	h.syncRoomToStore(key)
	h.broadcastRoomList(domainUUID)
}
```

- [ ] **Step 4: 接入 remote hook**

在 `server/gin.go` 的 `nb.SetRemoteHook` 中增加：

```go
if event == cluster.EventControlCommand {
	var cmd cluster.ControlCommand
	raw, _ := json.Marshal(payload)
	_ = json.Unmarshal(raw, &cmd)
	if cmd.NodeID == "" || cmd.NodeID == instanceID {
		_ = signalHub.HandleClusterCommand(cmd)
	}
	return
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/signal -run TestHubHandleClusterCommandKick -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/signal/hub.go app/server/internal/signal/hub_control_test.go app/server/server/gin.go
git commit -m "feat(cluster): worker executes NATS control commands"
```

---

## Task 4: 状态对账与 Agent 重启恢复

### Task 4.1: ClusterService.ReconcileAll

**Files:**
- Modify: `app/server/internal/service/cluster_service.go`
- Test: `app/server/internal/service/cluster_service_test.go`

- [ ] **Step 1: 写失败测试**

在 `cluster_service_test.go` 增加：

```go
func TestClusterServiceReconcileAllRemovesOfflineAssignments(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	nodeRepo := repository.NewClusterNodeRepository(db)
	assignRepo := repository.NewServerAssignmentRepository(db)
	node := &model.ClusterNode{
		UUID: "node-old", Name: "old", Status: model.ClusterNodeOffline, SFUHealthy: false,
		MaxServers: 10, MaxRooms: 100,
	}
	if err := nodeRepo.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := assignRepo.Ensure("srv-1", node.UUID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	if err := svc.ReconcileAll(time.Hour); err != nil {
		t.Fatalf("ReconcileAll: %v", err)
	}
	assignments, err := assignRepo.ListByServer("srv-1")
	if err != nil {
		t.Fatalf("ListByServer: %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected offline assignment removed, got %+v", assignments)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/service -run TestClusterServiceReconcileAllRemovesOfflineAssignments -v`
Expected: FAIL，`ReconcileAll` 未定义。

- [ ] **Step 3: 实现 ReconcileAll**

在 `cluster_service.go` 增加：

```go
func (s *ClusterService) ReconcileAll(timeout time.Duration) error {
	if err := s.ReapOffline(timeout); err != nil {
		return err
	}
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	for _, node := range nodes {
		if !cluster.CanSchedule(node) {
			assignments, err := s.assignRepo.ListByNode(node.UUID)
			if err != nil {
				continue
			}
			for _, assignment := range assignments {
				_ = s.assignRepo.Remove(assignment.ServerUUID, node.UUID)
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: 启动时调用**

在 `server/gin.go` 的 `startAgentClusterRuntime` 启动后立即调用：

```go
if err := clusterSvc.ReconcileAll(cfg.ClusterHeartbeatTimeoutDuration()); err != nil {
	logger.WithComponent("Cluster").Warnf("reconcile failed: %v", err)
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service -run TestClusterServiceReconcileAllRemovesOfflineAssignments -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/service/cluster_service.go app/server/internal/service/cluster_service_test.go app/server/server/gin.go
git commit -m "feat(cluster): reconcile cluster state on agent startup"
```

---

## Task 5: 集群健康指标与统计 API

### Task 5.1: ClusterService.Stats

**Files:**
- Modify: `app/server/internal/service/cluster_service.go`
- Test: `app/server/internal/service/cluster_service_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestClusterServiceStats(t *testing.T) {
	svc, db := setupClusterServiceTestDB(t)
	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.ReadyNodes != 0 {
		t.Fatalf("expected zero ready nodes, got %d", stats.ReadyNodes)
	}
}
```

- [ ] **Step 2: 实现 Stats**

```go
type ClusterStats struct {
	TotalNodes     int `json:"total_nodes"`
	ReadyNodes     int `json:"ready_nodes"`
	DrainingNodes  int `json:"draining_nodes"`
	OfflineNodes   int `json:"offline_nodes"`
	Assignments    int `json:"assignments"`
}

func (s *ClusterService) Stats() (ClusterStats, error) {
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return ClusterStats{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	stats := ClusterStats{TotalNodes: len(nodes)}
	for _, node := range nodes {
		switch node.Status {
		case model.ClusterNodeReady, model.ClusterNodeBusy:
			stats.ReadyNodes++
		case model.ClusterNodeDraining:
			stats.DrainingNodes++
		case model.ClusterNodeOffline:
			stats.OfflineNodes++
		}
		assignments, err := s.assignRepo.ListByNode(node.UUID)
		if err == nil {
			stats.Assignments += len(assignments)
		}
	}
	return stats, nil
}
```

- [ ] **Step 3: MonitorHandler 增加字段**

在 `healthSnapshot` 增加：

```go
ClusterTotalNodes   int `json:"cluster_total_nodes"`
ClusterReadyNodes   int `json:"cluster_ready_nodes"`
ClusterAssignments  int `json:"cluster_assignments"`
```

在 `MonitorHandler` 结构体增加 `clusterSvc *service.ClusterService`，构造函数增加对应参数，并在 `collect()` 中填充：

```go
if h.clusterSvc != nil {
	stats, err := h.clusterSvc.Stats()
	if err == nil {
		snap.ClusterTotalNodes = stats.TotalNodes
		snap.ClusterReadyNodes = stats.ReadyNodes
		snap.ClusterAssignments = stats.Assignments
	}
}
```

- [ ] **Step 4: 注册 `/api/v1/cluster/stats`**

在 `cluster_handler.go` 增加 `Stats` handler：

```go
func (h *ClusterHandler) Stats(c *gin.Context) {
	stats, err := h.clusterSvc.Stats()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, stats)
}
```

在 `routes.go` 增加：

```go
r.POST("/stats", middleware.RequirePermission(permcode.PermClusterRead), h.Stats)
```

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/service/cluster_service.go app/server/internal/handler/monitor_handler.go app/server/internal/handler/cluster_handler.go app/server/internal/router/routes/cluster/routes.go
git commit -m "feat(cluster): expose cluster health stats"
```

---

## Task 6: 前端集群管理页

### Task 6.1: cluster API 客户端

**Files:**
- Create: `app/web/src/api/cluster.ts`
- Test: `app/web/src/api/cluster.spec.ts`

- [ ] **Step 1: 创建 API**

```ts
import apiClient from "./apiClient";

export interface ClusterNodeView {
  node: Record<string, unknown>;
  labels: Record<string, string>;
}

export async function listClusterNodes() {
  const res = await apiClient.post({ url: "/api/v1/cluster/nodes/list" });
  return (res as any).data.data.nodes as ClusterNodeView[];
}

export async function scaleServer(serverUuid: string, replicas: number) {
  return apiClient.post({
    url: "/api/v1/cluster/servers/scale",
    data: { server_uuid: serverUuid, replicas },
  });
}
```

- [ ] **Step 2: 创建页面**

在 `app/web/src/pages/(app)/cluster/index.tsx` 创建：

```tsx
import { For, createResource } from "solid-js";
import { listClusterNodes } from "@/api/cluster";

export default function ClusterPage() {
	const [nodes] = createResource(listClusterNodes);
	return (
		<section>
			<h1>集群节点</h1>
			<For each={nodes()}>
				{(view) => (
					<div>
						{String(view.node.name)} - {String(view.node.status)}
					</div>
				)}
			</For>
		</section>
	);
}
```

- [ ] **Step 3: 注册路由**

在 `app/web/src/layouts/layout.tsx` 的管理菜单增加 `/cluster` 入口。

- [ ] **Step 4: Commit**

```bash
git add app/web/src/api/cluster.ts app/web/src/pages/'(app)'/cluster/index.tsx app/web/src/layouts/layout.tsx
git commit -m "feat(web): add cluster management page"
```

---

## Task 7: Nginx 集群路由与部署

### Task 7.1: nginx-cluster.conf

**Files:**
- Create: `deploy/nginx-cluster.conf`

- [ ] **Step 1: 创建配置**

```nginx
events {}

http {
  upstream gospeak_agent {
    server gospeak-agent:9000;
  }
  upstream gospeak_worker {
    server gospeak-worker:8998;
  }

  server {
    listen 80;

    location /api/v1/cluster/ {
      proxy_pass http://gospeak_agent;
    }
    location /ws {
      proxy_pass http://gospeak_worker;
      proxy_http_version 1.1;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";
    }
    location / {
      proxy_pass http://gospeak_agent;
    }
  }
}
```

- [ ] **Step 2: compose 增加 nginx-cluster 服务**

在 `deploy/docker-compose.yml` 的 `gospeak-worker` 后增加：

```yaml
  nginx-cluster:
    <<: *restart
    profiles: ["cluster"]
    image: nginx:alpine
    container_name: gospeak-nginx-cluster
    ports:
      - "${HTTP_PORT:-80}:80"
    volumes:
      - ./nginx-cluster.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      gospeak-agent:
        condition: service_started
      gospeak-worker:
        condition: service_started
```

- [ ] **Step 3: 验证 compose**

Run: `docker compose -f deploy/docker-compose.yml --profile cluster config`
Expected: exit 0，包含 `nginx-cluster`。

- [ ] **Step 4: Commit**

```bash
git add deploy/nginx-cluster.conf deploy/docker-compose.yml
git commit -m "feat(deploy): add cluster nginx routing"
```

---

## Task 8: Phase 6 高可用与自动扩缩容

### Task 8.1: Agent 主备锁

**Files:**
- Create: `app/server/internal/cluster/leader.go`
- Test: `app/server/internal/cluster/leader_test.go`

- [ ] **Step 1: 实现 NATS KV 锁**

```go
type LeaderLock interface {
	TryAcquire(ctx context.Context, nodeID string) (bool, error)
}
```

在 `leader.go` 中实现：

```go
type LeaderLock struct {
	kv nats.KeyValue
}

func OpenLeaderLock(js nats.JetStreamContext, prefix string) (*LeaderLock, error) {
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: prefix + "_leader",
		TTL:    5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &LeaderLock{kv: kv}, nil
}

func (l *LeaderLock) TryAcquire(ctx context.Context, nodeID string) (bool, error) {
	_, err := l.kv.Create("active", []byte(nodeID))
	if err == nats.ErrKeyExists {
		return false, nil
	}
	return err == nil, err
}
```

Agent 启动时尝试获取锁；失败则只启动 Worker 数据面。

- [ ] **Step 2: 自动扩缩容**

在 `ClusterService` 增加 `AutoScale`：

```go
func (s *ClusterService) AutoScale(serverUUID string, targetReplicas int) error {
	stats, err := s.Stats()
	if err != nil {
		return err
	}
	if stats.ReadyNodes == 0 {
		return nil
	}
	_, err = s.ScaleServer(serverUUID, targetReplicas, "")
	return err
}
```

- [ ] **Step 3: 滚动灰度**

在 `ClusterService` 增加 `MarkServerAssignmentsDraining`：

```go
func (s *ClusterService) MarkServerAssignmentsDraining(serverUUID string) error {
	assignments, err := s.assignRepo.ListByServer(serverUUID)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	for _, assignment := range assignments {
		_ = s.assignRepo.UpdateStatus(serverUUID, assignment.NodeUUID, model.ServerAssignmentDraining)
	}
	return nil
}
```

节点 draining 后停止新调度；房间迁移完成后由 `ScaleServer` 删除多余分配。

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/cluster/leader.go app/server/internal/cluster/leader_test.go app/server/internal/service/cluster_service.go
git commit -m "feat(cluster): add leader lock and auto-scaling hooks"
```

## Task 9: 更新高层计划状态

**Files:**
- Modify: `docs/cluster-agent-worker-plan.md`

- [ ] **Step 1: 在顶部状态段增加详细计划链接**

```markdown
> 详细剩余计划：`docs/superpowers/plans/2026-08-02-cluster-control-plane-completion.md`
```

- [ ] **Step 2: Commit**

```bash
git add docs/cluster-agent-worker-plan.md docs/superpowers/plans/2026-08-02-cluster-control-plane-completion.md
git commit -m "docs(cluster): add remaining control plane implementation plan"
```

---

## 自检清单

- [ ] 覆盖前端 workerUrl socket 路由
- [ ] 覆盖 Worker 只读 DB 与写面拒绝
- [ ] 覆盖 NATS 控制命令发布/执行
- [ ] 覆盖 Agent 重启对账
- [ ] 覆盖集群健康指标与管理页
- [ ] 覆盖 Nginx 集群路由
- [ ] 覆盖 Phase 6 主备/自动扩缩/灰度扩展点
- [ ] 无 TBD/TODO 占位符
- [ ] 类型、函数名、路径与现有代码一致
