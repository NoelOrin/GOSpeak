# MediaSoup Provider 完善 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补全 mediasoup provider 的 participant 模型、离开清理广播、服务端 mute、active speaker 检测、e2e runbook。

**Architecture:** worker 维护 identity↔producer/transport 索引作单一可信源;Go signal 层加 `ParticipantCleanupHandler` 接口,Hub OnDisconnect 类型断言调用(仅 mediasoup 实现,其它 provider 不受影响);active speaker 纯前端 WebAudio AnalyserNode;mute 走 producer pause/resume 端点。

**Tech Stack:** TypeScript(mediasoup-worker, Express) · Go(net/http, httptest, go-socket.io) · sfu-client(mediasoup-client, WebAudio)

## Global Constraints

- 代码不加注释(除复杂逻辑必须),代码不用 emoji,仅文档可用 — 见 `CLAUDE.md` 代码规范。
- Go 文件 snake_case,类型 PascalCase,import 三组(标准库/第三方/内部)空行分隔。
- Service 返回 `*pkg.AppError`,Handler 用 `pkg.HandleError`。
- **不修改其它 SFU provider**(livekit/srs/agora/daily)。Hub 改动经接口类型断言,未实现者 nil 跳过。
- 提交规范 Conventional Commits(`feat:`/`fix:`/`docs:`/`refactor:`)。
- 测试日志 Markdown 存 `agent_test_logs/`(若执行中跑 agent 测试)。

参考设计:`docs/superpowers/specs/2026-07-06-mediasoup-completion-design.md`

---

## File Structure

| 文件 | 责任 | 动作 |
|------|------|------|
| `packages/mediasoup-worker/src/worker.ts` | participant 索引 + transport/producer 生命周期 | 修改 |
| `packages/mediasoup-worker/src/api.ts` | HTTP 端点(participants/close/pause/resume) | 修改 |
| `app/server/internal/mediasoup/bridge.go` | bridge HTTP 客户端 + 新方法 + `ParticipantInfo` 类型 | 修改 |
| `app/server/internal/mediasoup/bridge_test.go` | bridge httptest 单测 | 新建 |
| `app/server/internal/mediasoup/signal.go` | socketIndex + OnParticipantLeft + close-transport 事件 | 修改 |
| `app/server/internal/mediasoup/signal_test.go` | OnParticipantLeft 广播+清理单测 | 新建 |
| `app/server/internal/mediasoup/provider.go` | ListParticipants/RemoveParticipant/Mute 实现 | 修改 |
| `app/server/internal/mediasoup/provider_test.go` | provider 委托单测 | 新建 |
| `app/server/internal/signal/hub.go` | `ParticipantCleanupHandler` 接口 + OnDisconnect 调用 | 修改 |
| `packages/sfu-client/src/mediasoup-client.ts` | AnalyserNode getLevel + active speaker timer + CREATE_TRANSPORT 带 identity | 修改 |
| `deploy/docker-compose.example.yml` | mediasoup-worker 注释块补 ANNOUNCED_IP | 修改 |
| `docs/mediasoup-selfhost-runbook.md` | 自部署 e2e runbook | 新建 |
| `docs/sfu-provider-maturity.md` | 更新 mediasoup 行(ListParticipants/Mute/RemoveParticipant 改 ✅) | 修改 |

---

## Task 1: worker participant 索引 + transport/producer 登记

**Files:**
- Modify: `packages/mediasoup-worker/src/worker.ts`

**Interfaces:**
- Produces: `RoomState.participants: Map<string, ParticipantState>`;`createTransport(roomId, identity, direction)`;`addProducer` 从 appData.identity 登记;`getParticipant`/`removeParticipant`;`listParticipants`;`closeParticipant`;`pauseProducer`/`resumeProducer`;`pauseParticipant`/`resumeParticipant`。

- [ ] **Step 1: 扩展 RoomState + ParticipantState 类型**

替换 `worker.ts:15-20` 的 `RoomState` 接口:

```ts
interface ParticipantState {
	sendTransportId?: string;
	recvTransportId?: string;
	producerIds: Set<string>;
}

interface RoomState {
	router: Router;
	transports: Map<string, WebRtcTransport>;
	producers: Map<string, Producer>;
	consumers: Map<string, Consumer>;
	participants: Map<string, ParticipantState>;
}
```

- [ ] **Step 2: createRouter 初始化 participants 字段**

替换 `worker.ts:55-60` 的 `state` 初始化:

```ts
		const state: RoomState = {
			router,
			transports: new Map(),
			producers: new Map(),
			consumers: new Map(),
			participants: new Map(),
		};
```

- [ ] **Step 3: createTransport 接收 identity+direction 并登记**

替换 `worker.ts:78-98` 的 `createTransport` 方法:

```ts
	async createTransport(roomId: string, identity?: string, direction?: "send" | "recv"): Promise<WebRtcTransport> {
		const room = this.rooms.get(roomId);
		if (!room) throw new Error("room not found");
		const announcedIp = process.env.ANNOUNCED_IP || undefined;
		const transport = await room.router.createWebRtcTransport({
			listenIps: [
				{
					ip: process.env.LISTEN_IP || "0.0.0.0",
					announcedIp,
				},
			],
			enableUdp: true,
			enableTcp: true,
			preferUdp: true,
			initialAvailableOutgoingBitrate: 1_000_000,
			maxSendMessageSize: 262_144,
		});
		room.transports.set(transport.id, transport);
		transport.observer.on("close", () => room.transports.delete(transport.id));
		if (identity) {
			let participant = room.participants.get(identity);
			if (!participant) {
				participant = { producerIds: new Set() };
				room.participants.set(identity, participant);
			}
			if (direction === "send") participant.sendTransportId = transport.id;
			else if (direction === "recv") participant.recvTransportId = transport.id;
		}
		return transport;
	}
```

- [ ] **Step 4: addProducer 从 appData.identity 登记 + close 清理**

替换 `worker.ts:104-109` 的 `addProducer` 方法:

```ts
	addProducer(roomId: string, producer: Producer): void {
		const room = this.rooms.get(roomId);
		if (!room) return;
		room.producers.set(producer.id, producer);
		const identity = (producer.appData as { identity?: string } | null)?.identity;
		if (identity) {
			let participant = room.participants.get(identity);
			if (!participant) {
				participant = { producerIds: new Set() };
				room.participants.set(identity, participant);
			}
			participant.producerIds.add(producer.id);
		}
		producer.observer.on("close", () => {
			room.producers.delete(producer.id);
			if (identity) {
				room.participants.get(identity)?.producerIds.delete(producer.id);
			}
		});
	}
```

- [ ] **Step 5: 新增 participant 查询 + 操作方法**

在 `worker.ts` 的 `listProducers` 方法之后(`worker.ts:130` 后)插入:

```ts
	getParticipant(roomId: string, identity: string): ParticipantState | undefined {
		return this.rooms.get(roomId)?.participants.get(identity);
	}

	listParticipants(roomId: string): Array<{ identity: string; producerCount: number; hasSendTransport: boolean; hasRecvTransport: boolean }> {
		const room = this.rooms.get(roomId);
		if (!room) return [];
		return Array.from(room.participants.entries()).map(([identity, p]) => ({
			identity,
			producerCount: p.producerIds.size,
			hasSendTransport: p.sendTransportId !== undefined,
			hasRecvTransport: p.recvTransportId !== undefined,
		}));
	}

	async closeParticipant(roomId: string, identity: string): Promise<string[]> {
		const room = this.rooms.get(roomId);
		if (!room) return [];
		const participant = room.participants.get(identity);
		if (!participant) return [];
		const closedProducerIds: string[] = [];
		for (const pid of participant.producerIds) {
			const producer = room.producers.get(pid);
			if (producer && !producer.closed) {
				producer.close();
				closedProducerIds.push(pid);
			}
		}
		if (participant.sendTransportId) {
			const t = room.transports.get(participant.sendTransportId);
			if (t && !t.closed) t.close();
		}
		if (participant.recvTransportId) {
			const t = room.transports.get(participant.recvTransportId);
			if (t && !t.closed) t.close();
		}
		room.participants.delete(identity);
		return closedProducerIds;
	}

	pauseProducer(roomId: string, producerId: string): void {
		const producer = this.rooms.get(roomId)?.producers.get(producerId);
		if (producer && !producer.closed) producer.pause();
	}

	resumeProducer(roomId: string, producerId: string): void {
		const producer = this.rooms.get(roomId)?.producers.get(producerId);
		if (producer && !producer.closed) producer.resume();
	}

	pauseParticipant(roomId: string, identity: string): void {
		const room = this.rooms.get(roomId);
		const participant = room?.participants.get(identity);
		if (!room || !participant) return;
		for (const pid of participant.producerIds) {
			const producer = room.producers.get(pid);
			if (producer && !producer.closed) producer.pause();
		}
	}

	resumeParticipant(roomId: string, identity: string): void {
		const room = this.rooms.get(roomId);
		const participant = room?.participants.get(identity);
		if (!room || !participant) return;
		for (const pid of participant.producerIds) {
			const producer = room.producers.get(pid);
			if (producer && !producer.closed) producer.resume();
		}
	}
```

- [ ] **Step 6: closeRouter 也清 participants(close 已级联,无需额外,但确认 closeRouter 删 rooms 即可)**

`closeRouter`(`worker.ts:132-136`)已 `router.close()` 级联关 transport/producer,且 `rooms.delete`。无需改。确认即可。

- [ ] **Step 7: 类型检查**

Run: `cd packages/mediasoup-worker && pnpm exec tsc --noEmit`
Expected: 无错误退出码 0。

- [ ] **Step 8: Commit**

```bash
git add packages/mediasoup-worker/src/worker.ts
git commit -m "feat(mediasoup-worker): participant 索引 + producer/transport 生命周期登记"
```

---

## Task 2: worker HTTP 端点(participants/close/pause/resume)

**Files:**
- Modify: `packages/mediasoup-worker/src/api.ts`

**Interfaces:**
- Consumes: Task 1 的 worker 方法。
- Produces: HTTP 端点供 Task 3 Go bridge 调用。

- [ ] **Step 1: createTransport 端点传 identity+direction**

替换 `api.ts:38-52` 的 transports 端点:

```ts
	router.post("/rooms/:roomId/transports", async (req, res) => {
		try {
			const { identity, direction } = req.body || {};
			const transport = await worker.createTransport(req.params.roomId, identity, direction);
			res.json({
				id: transport.id,
				iceParameters: transport.iceParameters,
				iceCandidates: transport.iceCandidates,
				dtlsParameters: transport.dtlsParameters,
				sctpParameters: transport.sctpParameters,
			});
		} catch (err) {
			const message = (err as Error).message;
			res.status(message === "room not found" ? 404 : 500).json({ error: message });
		}
	});
```

- [ ] **Step 2: 新增 participants 端点**

在 `api.ts` 的 `producers` GET 端点之后(`api.ts:104` 后,`return router` 之前)插入:

```ts
	router.get("/rooms/:roomId/participants", (req, res) => {
		if (!worker.getRoom(req.params.roomId)) return res.status(404).json({ error: "room not found" });
		res.json({ participants: worker.listParticipants(req.params.roomId) });
	});

	router.post("/rooms/:roomId/participants/:identity/close", async (req, res) => {
		if (!worker.getRoom(req.params.roomId)) return res.status(404).json({ error: "room not found" });
		const closedProducerIds = await worker.closeParticipant(req.params.roomId, req.params.identity);
		res.json({ ok: true, closedProducerIds });
	});

	router.post("/rooms/:roomId/participants/:identity/pause", (req, res) => {
		if (!worker.getParticipant(req.params.roomId, req.params.identity)) {
			return res.status(404).json({ error: "participant not found" });
		}
		worker.pauseParticipant(req.params.roomId, req.params.identity);
		res.json({ ok: true });
	});

	router.post("/rooms/:roomId/participants/:identity/resume", (req, res) => {
		if (!worker.getParticipant(req.params.roomId, req.params.identity)) {
			return res.status(404).json({ error: "participant not found" });
		}
		worker.resumeParticipant(req.params.roomId, req.params.identity);
		res.json({ ok: true });
	});

	router.post("/rooms/:roomId/producers/:producerId/pause", (req, res) => {
		if (!worker.getProducer(req.params.roomId, req.params.producerId)) {
			return res.status(404).json({ error: "producer not found" });
		}
		worker.pauseProducer(req.params.roomId, req.params.producerId);
		res.json({ ok: true });
	});

	router.post("/rooms/:roomId/producers/:producerId/resume", (req, res) => {
		if (!worker.getProducer(req.params.roomId, req.params.producerId)) {
			return res.status(404).json({ error: "producer not found" });
		}
		worker.resumeProducer(req.params.roomId, req.params.producerId);
		res.json({ ok: true });
	});
```

- [ ] **Step 3: 类型检查**

Run: `cd packages/mediasoup-worker && pnpm exec tsc --noEmit`
Expected: 退出码 0。

- [ ] **Step 4: Commit**

```bash
git add packages/mediasoup-worker/src/api.ts
git commit -m "feat(mediasoup-worker): participants/close/pause/resume HTTP 端点"
```

---

## Task 3: Go bridge 新方法 + 类型

**Files:**
- Modify: `app/server/internal/mediasoup/bridge.go`
- Test: `app/server/internal/mediasoup/bridge_test.go`

**Interfaces:**
- Consumes: Task 2 的 worker HTTP 端点。
- Produces: `ParticipantInfo` 类型;`ListParticipants`/`CloseParticipant`/`PauseProducer`/`ResumeProducer`/`PauseParticipant`/`ResumeParticipant` 方法;`CreateTransport` 扩展签名(供 Task 5 signal 调用)。

- [ ] **Step 1: 写失败测试 — bridge ListParticipants**

新建 `app/server/internal/mediasoup/bridge_test.go`:

```go
package mediasoup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMockWorker(t *testing.T, handler http.HandlerFunc) *BridgeClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewBridgeClient(srv.URL)
}

func TestListParticipants(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/participants", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"participants": []map[string]interface{}{
				{"identity": "alice", "producerCount": 1, "hasSendTransport": true, "hasRecvTransport": true},
			},
		})
	})
	b := newMockWorker(t, mux.ServeHTTP)

	got, err := b.ListParticipants("r1")
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(got) != 1 || got[0].Identity != "alice" || got[0].ProducerCount != 1 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestCloseParticipant(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/participants/alice/close", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "closedProducerIds": []string{"p1"}})
	})
	b := newMockWorker(t, mux.ServeHTTP)

	got, err := b.CloseParticipant("r1", "alice")
	if err != nil {
		t.Fatalf("CloseParticipant: %v", err)
	}
	if len(got) != 1 || got[0] != "p1" {
		t.Fatalf("unexpected closedProducerIds: %v", got)
	}
}

func TestCloseParticipant_NotFoundIsNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/participants/ghost/close", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"participant not found"}`))
	})
	b := newMockWorker(t, mux.ServeHTTP)

	got, err := b.CloseParticipant("r1", "ghost")
	if err != nil {
		t.Fatalf("404 should map to nil error, got: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil closedProducerIds, got %v", got)
	}
}

func TestPauseProducer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/producers/p1/pause", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	b := newMockWorker(t, mux.ServeHTTP)

	if err := b.PauseProducer("r1", "p1"); err != nil {
		t.Fatalf("PauseProducer: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/mediasoup/ -run 'TestListParticipants|TestCloseParticipant|TestPauseProducer' -v`
Expected: FAIL — `b.ListParticipants undefined` 等。

- [ ] **Step 3: 实现 ParticipantInfo 类型 + 新方法**

在 `bridge.go` 的 `ConsumeResult` 结构体之后(line 40 后)插入类型:

```go
type ParticipantInfo struct {
	Identity         string `json:"identity"`
	ProducerCount    int    `json:"producerCount"`
	HasSendTransport bool   `json:"hasSendTransport"`
	HasRecvTransport bool   `json:"hasRecvTransport"`
}
```

在 `bridge.go` 的 `Consume` 方法之后(line 130 后,`do` 之前)插入方法:

```go
func (b *BridgeClient) ListParticipants(roomID string) ([]ParticipantInfo, error) {
	var result struct {
		Participants []ParticipantInfo `json:"participants"`
	}
	if err := b.do(http.MethodGet, "/rooms/"+roomID+"/participants", nil, &result); err != nil {
		return nil, err
	}
	return result.Participants, nil
}

func (b *BridgeClient) CloseParticipant(roomID, identity string) ([]string, error) {
	var result struct {
		OK               bool     `json:"ok"`
		ClosedProducerID []string `json:"closedProducerIds"`
	}
	err := b.do(http.MethodPost, "/rooms/"+roomID+"/participants/"+identity+"/close", bytes.NewReader([]byte("{}")), &result)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return result.ClosedProducerID, nil
}

func (b *BridgeClient) PauseProducer(roomID, producerID string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/producers/"+producerID+"/pause", bytes.NewReader([]byte("{}")), nil)
}

func (b *BridgeClient) ResumeProducer(roomID, producerID string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/producers/"+producerID+"/resume", bytes.NewReader([]byte("{}")), nil)
}

func (b *BridgeClient) PauseParticipant(roomID, identity string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/participants/"+identity+"/pause", bytes.NewReader([]byte("{}")), nil)
}

func (b *BridgeClient) ResumeParticipant(roomID, identity string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/participants/"+identity+"/resume", bytes.NewReader([]byte("{}")), nil)
}
```

- [ ] **Step 4: 加 isNotFound 辅助 + 扩展 CreateTransport 签名**

在 `bridge.go` 的 `do` 方法之后(文件末尾)加:

```go
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status=404")
}
```

修改 `CreateTransport`(line 83-89)接受 identity+direction:

```go
func (b *BridgeClient) CreateTransport(roomID, identity, direction string) (*TransportParams, error) {
	body, err := json.Marshal(map[string]string{
		"identity":  identity,
		"direction": direction,
	})
	if err != nil {
		return nil, err
	}
	var result TransportParams
	if err := b.do(http.MethodPost, "/rooms/"+roomID+"/transports", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/mediasoup/ -run 'TestListParticipants|TestCloseParticipant|TestPauseProducer' -v`
Expected: PASS。

- [ ] **Step 6: 全包编译(signal.go 仍用旧 CreateTransport 签名,需同步改 — 此步在 Task 5 完成,此处先确认 bridge_test 通过即可)**

Run: `cd app/server && go build ./internal/mediasoup/`
Expected: 编译失败因 signal.go line 62 调 `CreateTransport(req.Room)` 旧签名 — 预期。Task 5 修复。先跳过整体 build,仅跑 bridge_test。

Run: `cd app/server && go vet ./internal/mediasoup/bridge.go ./internal/mediasoup/bridge_test.go`
(若 vet 报 signal.go 错误属预期,忽略;bridge_test 跑通即可。)

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/mediasoup/bridge.go app/server/internal/mediasoup/bridge_test.go
git commit -m "feat(mediasoup): bridge 新增 participant/mute 方法 + CreateTransport 签名扩展"
```

注:此提交后 signal.go 暂不编译,Task 5 修复。若需保持每提交可编译,可临时把 signal.go 的 `CreateTransport(req.Room)` 改为 `CreateTransport(req.Room, "", "")` 一并提交。下面 Step 8 做这个临时修复以保持绿。

- [ ] **Step 8: 临时修复 signal.go 调用点保持编译**

`signal.go` line 62:
```go
		params, err := m.bridge.CreateTransport(req.Room, req.Identity, req.Direction)
```
并在 create-transport handler 的 req 结构体加字段(line 56-58):

```go
		var req struct {
			Room      string `json:"room"`
			Identity  string `json:"identity,omitempty"`
			Direction string `json:"direction,omitempty"`
		}
```

Run: `cd app/server && go build ./internal/mediasoup/`
Expected: 退出码 0。

```bash
git add app/server/internal/mediasoup/signal.go
git commit -m "fix(mediasoup): signal create-transport 适配新 CreateTransport 签名"
```

---

## Task 4: signal 包 ParticipantCleanupHandler 接口 + Hub OnDisconnect 调用

**Files:**
- Modify: `app/server/internal/signal/hub.go`

**Interfaces:**
- Produces: `signal.ParticipantCleanupHandler` 接口;Hub OnDisconnect 调用它(类型断言,未实现者跳过)。

**约束:** 此任务不改其它 provider。仅加接口 + OnDisconnect 一处类型断言调用。

- [ ] **Step 1: 加 ParticipantCleanupHandler 接口 + Hub 字段**

在 `hub.go` 的 `StreamNameResolver` 接口之后(line 67 后)加:

```go
// ParticipantCleanupHandler 处理参与者离开时的 SFU 专属清理(如 mediasoup 广播 producer-closed + 关 transport)。
// 仅 mediasoup 实现;其它 provider 不实现此接口,Hub OnDisconnect 类型断言跳过。
type ParticipantCleanupHandler interface {
	OnParticipantLeft(room, identity string)
}
```

`Hub` 结构体(line 80 `sfuSignalHandler` 后)加字段:

```go
	sfuSignalHandler     SFUSignalHandler
	participantCleanup   ParticipantCleanupHandler
```

修改 `SetSFUSignalHandler`(line 106-108):

```go
func (h *Hub) SetSFUSignalHandler(handler SFUSignalHandler) {
	h.sfuSignalHandler = handler
	if ch, ok := handler.(ParticipantCleanupHandler); ok {
		h.participantCleanup = ch
	}
}
```

- [ ] **Step 2: OnDisconnect 异步清理段调用 participantCleanup**

`hub.go` line 173-180 的 goroutine 内,`removeParticipantSafe` 之后加调用。替换该 goroutine 体:

```go
		go func(cleanups []disconnectCleanup) {
			for _, c := range cleanups {
				h.removeParticipantSafe(c.room, c.identity)
				if c.deleted {
					h.deleteRoomSafe(c.room)
				}
				if h.participantCleanup != nil {
					h.participantCleanup.OnParticipantLeft(c.room, c.identity)
				}
			}
		}(cleanups)
```

- [ ] **Step 3: 编译 + 既有测试不破**

Run: `cd app/server && go build ./internal/signal/`
Expected: 退出码 0。

Run: `cd app/server && go test ./internal/signal/ -run TestHub_OnDisconnect -v`
Expected: 既有 OnDisconnect 测试 PASS(未设 participantCleanup → nil → 跳过)。

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/signal/hub.go
git commit -m "feat(signal): ParticipantCleanupHandler 接口 + OnDisconnect 调用(仅 mediasoup 生效)"
```

---

## Task 5: MediasoupSignal socketIndex + OnParticipantLeft + close-transport 事件

**Files:**
- Modify: `app/server/internal/mediasoup/signal.go`
- Test: `app/server/internal/mediasoup/signal_test.go`

**Interfaces:**
- Consumes: Task 4 的 `signal.ParticipantCleanupHandler` 接口;Task 3 的 `bridge.CloseParticipant`。
- Produces: `MediasoupSignal.OnParticipantLeft(room, identity)`;produce 时登记 socketIndex;`sfu:close-transport` 事件。

- [ ] **Step 1: 写失败测试 — OnParticipantLeft 广播 + 清理**

新建 `app/server/internal/mediasoup/signal_test.go`:

```go
package mediasoup

import (
	"sync"
	"testing"
)

type stubBridge struct {
	mu       sync.Mutex
	closedId string
}

func (s *stubBridge) CloseParticipant(room, identity string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closedId = identity
	return nil, nil
}

func TestOnParticipantLeft_BroadcastsAndCloses(t *testing.T) {
	var (
		mu      sync.Mutex
		gotRoom string
		gotEvt  string
		gotId   string
	)
	bcast := func(room, event string, data interface{}) {
		mu.Lock()
		defer mu.Unlock()
		gotRoom = room
		gotEvt = event
		gotId = data.(map[string]interface{})["identity"].(string)
	}
	stub := &stubBridge{}
	sig := &MediasoupSignal{bridge: stub, broadcast: bcast}

	sig.OnParticipantLeft("r1", "alice")

	stub.mu.Lock()
	if stub.closedId != "alice" {
		t.Fatalf("CloseParticipant called with %q, want alice", stub.closedId)
	}
	stub.mu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	if gotRoom != "r1" || gotEvt != "sfu:producer-closed" || gotId != "alice" {
		t.Fatalf("broadcast got room=%q evt=%q id=%q", gotRoom, gotEvt, gotId)
	}
}
```

注:测试直接构造 `MediasoupSignal{bridge: stub, broadcast: bcast}` — 需 `MediasoupSignal` 字段 `bridge` 类型为 `*BridgeClient`。stub 不是 BridgeClient,故 Step 3 需把 bridge 字段改为接口。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/mediasoup/ -run TestOnParticipantLeft -v`
Expected: FAIL — `sig.OnParticipantLeft undefined` 或字段类型不匹配。

- [ ] **Step 3: 重构 MediasoupSignal — bridge 字段改接口 + 加 socketIndex + OnParticipantLeft**

`signal.go` 顶部 import 加 `"sync"`。定义接口(在 `BroadcastFn` 之后):

```go
type participantBridge interface {
	CloseParticipant(roomID, identity string) ([]string, error)
}
```

修改 `MediasoupSignal` 结构体(line 13-16):

```go
type MediasoupSignal struct {
	bridge      participantBridge
	broadcast   BroadcastFn
	socketIndex sync.Map
}

func NewMediasoupSignal(bridge *BridgeClient, broadcast BroadcastFn) *MediasoupSignal {
	return &MediasoupSignal{bridge: bridge, broadcast: broadcast}
}
```

`*BridgeClient` 已有 `CloseParticipant` 方法 → 满足 `participantBridge` 接口,`NewMediasoupSignal` 签名不变,gin.go 调用点无需改。

produce handler 成功后(line 98-108 的 broadcast 块)登记 socketIndex。在 `if m.broadcast != nil {` 之前加:

```go
		var appDataMap map[string]interface{}
		_ = json.Unmarshal(req.AppData, &appDataMap)
		if id, ok := appDataMap["identity"].(string); ok && id != "" {
			m.socketIndex.Store(s.ID(), participantEntry{room: req.Room, identity: id})
		}
```

并定义 `participantEntry`(接口后):

```go
type participantEntry struct {
	room     string
	identity string
}
```

在 `RegisterRoutes` 之后加 `OnParticipantLeft`:

```go
func (m *MediasoupSignal) OnParticipantLeft(room, identity string) {
	if m.broadcast != nil {
		m.broadcast(room, "sfu:producer-closed", map[string]interface{}{
			"room":     room,
			"identity": identity,
		})
	}
	go func() {
		_, _ = m.bridge.CloseParticipant(room, identity)
	}()
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/mediasoup/ -run TestOnParticipantLeft -v`
Expected: PASS。

- [ ] **Step 5: 加 sfu:close-transport 事件(前端显式离开)**

在 `RegisterRoutes` 的 consume handler 之后(line 125 后)加:

```go
	server.OnEvent("/", "sfu:close-transport", safeHandler(func(s socketio.Conn, payload string) (string, error) {
		var req struct {
			Room     string `json:"room"`
			Identity string `json:"identity"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" || req.Identity == "" {
			return `{"error":"room and identity required"}`, nil
		}
		m.OnParticipantLeft(req.Room, req.Identity)
		return `{"ok":true}`, nil
	}))
```

- [ ] **Step 6: 全包编译 + 测试**

Run: `cd app/server && go build ./internal/mediasoup/`
Expected: 退出码 0。

Run: `cd app/server && go test ./internal/mediasoup/ -v`
Expected: 全 PASS。

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/mediasoup/signal.go app/server/internal/mediasoup/signal_test.go
git commit -m "feat(mediasoup): OnParticipantLeft 广播 producer-closed + close-transport 事件"
```

---

## Task 6: provider 实现 ListParticipants/RemoveParticipant/Mute

**Files:**
- Modify: `app/server/internal/mediasoup/provider.go`
- Test: `app/server/internal/mediasoup/provider_test.go`

**Interfaces:**
- Consumes: Task 3 的 bridge 方法。
- Produces: provider 9 方法中 mediasoup 的 ListParticipants/RemoveParticipant/MuteParticipant/MuteRoomParticipant 不再返回 notSupported。

- [ ] **Step 1: 写失败测试 — provider 委托 bridge**

新建 `app/server/internal/mediasoup/provider_test.go`:

```go
package mediasoup

import (
	"sync"
	"testing"
)

type stubProviderBridge struct {
	mu               sync.Mutex
	listedRoom       string
	closedIdentity   string
	pausedProducer   string
	resumedProducer  string
	pausedIdentity   string
	resumedIdentity  string
	listResult       []ParticipantInfo
	closeResult      []string
}

func (s *stubProviderBridge) ListParticipants(roomID string) ([]ParticipantInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listedRoom = roomID
	return s.listResult, nil
}

func (s *stubProviderBridge) CloseParticipant(roomID, identity string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closedIdentity = identity
	return s.closeResult, nil
}

func (s *stubProviderBridge) PauseProducer(roomID, producerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pausedProducer = producerID
	return nil
}

func (s *stubProviderBridge) ResumeProducer(roomID, producerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumedProducer = producerID
	return nil
}

func (s *stubProviderBridge) PauseParticipant(roomID, identity string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pausedIdentity = identity
	return nil
}

func (s *stubProviderBridge) ResumeParticipant(roomID, identity string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumedIdentity = identity
	return nil
}

func newSvcWithStub() *Service {
	return &Service{Bridge: &BridgeClient{}}
}

func TestListParticipants_Delegates(t *testing.T) {
	svc := &Service{Bridge: nil}
	svc.partBridge = &stubProviderBridge{listResult: []ParticipantInfo{{Identity: "alice"}}}
	got, err := svc.ListParticipants("r1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.([]ParticipantInfo)) != 1 {
		t.Fatalf("got %v", got)
	}
}
```

注:provider 需暴露 `partBridge` 字段供测试注入。Step 3 实现。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/mediasoup/ -run TestListParticipants_Delegates -v`
Expected: FAIL — `svc.partBridge undefined`。

- [ ] **Step 3: provider 加 partBridge 接口字段 + 实现方法**

`provider.go` 定义接口(在 `type Service struct` 之前):

```go
type providerBridge interface {
	ListParticipants(roomID string) ([]ParticipantInfo, error)
	CloseParticipant(roomID, identity string) ([]string, error)
	PauseProducer(roomID, producerID string) error
	ResumeProducer(roomID, producerID string) error
	PauseParticipant(roomID, identity string) error
	ResumeParticipant(roomID, identity string) error
}
```

修改 `Service` 结构体(line 10-13):

```go
type Service struct {
	Bridge     *BridgeClient
	partBridge providerBridge
	host       string
}

func NewService(cfg *config.Config) *Service {
	b := NewBridgeClient(cfg.MediaSoupBridgeURL)
	return &Service{
		Bridge:     b,
		partBridge: b,
		host:       cfg.MediaSoupHost,
	}
}
```

`*BridgeClient` 实现全部 6 方法 → 满足 `providerBridge`。

替换 `ListParticipants`/`MuteParticipant`/`MuteRoomParticipant`/`RemoveParticipant`(line 41-55):

```go
func (s *Service) ListParticipants(room string) (interface{}, error) {
	participants, err := s.partBridge.ListParticipants(room)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return participants, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	var err error
	if trackSid != "" {
		if muted {
			err = s.partBridge.PauseProducer(room, trackSid)
		} else {
			err = s.partBridge.ResumeProducer(room, trackSid)
		}
	} else {
		if muted {
			err = s.partBridge.PauseParticipant(room, identity)
		} else {
			err = s.partBridge.ResumeParticipant(room, identity)
		}
	}
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	var err error
	if muted {
		err = s.partBridge.PauseParticipant(room, identity)
	} else {
		err = s.partBridge.ResumeParticipant(room, identity)
	}
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) RemoveParticipant(room, identity string) error {
	if _, err := s.partBridge.CloseParticipant(room, identity); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/mediasoup/ -run TestListParticipants_Delegates -v`
Expected: PASS。

- [ ] **Step 5: 补 mute/remove 委托测试**

在 `provider_test.go` 末尾加:

```go
func TestMuteParticipant_ByTrackSid(t *testing.T) {
	stub := &stubProviderBridge{}
	svc := &Service{partBridge: stub}
	if err := svc.MuteParticipant("r1", "alice", "p1", true); err != nil {
		t.Fatalf("err: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.pausedProducer != "p1" {
		t.Fatalf("expected PauseProducer p1, got %q", stub.pausedProducer)
	}
}

func TestMuteParticipant_ByIdentity(t *testing.T) {
	stub := &stubProviderBridge{}
	svc := &Service{partBridge: stub}
	if err := svc.MuteParticipant("r1", "alice", "", false); err != nil {
		t.Fatalf("err: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.resumedIdentity != "alice" {
		t.Fatalf("expected ResumeParticipant alice, got %q", stub.resumedIdentity)
	}
}

func TestRemoveParticipant_Delegates(t *testing.T) {
	stub := &stubProviderBridge{closeResult: []string{"p1"}}
	svc := &Service{partBridge: stub}
	if err := svc.RemoveParticipant("r1", "alice"); err != nil {
		t.Fatalf("err: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.closedIdentity != "alice" {
		t.Fatalf("expected CloseParticipant alice, got %q", stub.closedIdentity)
	}
}
```

Run: `cd app/server && go test ./internal/mediasoup/ -v`
Expected: 全 PASS。

- [ ] **Step 6: 删除冗余 newSvcWithStub helper(测试未用)**

`provider_test.go` 中的 `newSvcWithStub` 函数若未被引用则删除,避免编译警告。

- [ ] **Step 7: 全服务编译**

Run: `cd app/server && go build ./...`
Expected: 退出码 0。

- [ ] **Step 8: Commit**

```bash
git add app/server/internal/mediasoup/provider.go app/server/internal/mediasoup/provider_test.go
git commit -m "feat(mediasoup): provider 实现 ListParticipants/RemoveParticipant/Mute"
```

---

## Task 7: sfu-client active speaker(AnalyserNode) + CREATE_TRANSPORT 带 identity

**Files:**
- Modify: `packages/sfu-client/src/mediasoup-client.ts`

**Interfaces:**
- Consumes: 无新接口,复用现有 `onActiveSpeakersCb`。
- Produces: 真实 active speaker 检测;CREATE_TRANSPORT payload 带 identity。

- [ ] **Step 1: MediaSoupRemoteAudioTrack 加 AnalyserNode + getLevel**

替换 `mediasoup-client.ts:20-62` 的 `MediaSoupRemoteAudioTrack` 类:

```ts
class MediaSoupRemoteAudioTrack implements RemoteAudioTrackLike {
	private elements: HTMLAudioElement[] = [];
	private audioContext: AudioContext;
	private gainNode: GainNode;
	private analyser: AnalyserNode;
	private levelBuffer: Uint8Array;

	constructor(private consumer: mediasoupTypes.Consumer) {
		this.audioContext = new AudioContext();
		this.gainNode = this.audioContext.createGain();
		this.gainNode.gain.value = 1;
		this.analyser = this.audioContext.createAnalyser();
		this.analyser.fftSize = 512;
		this.levelBuffer = new Uint8Array(this.analyser.fftSize);
		this.analyser.connect(this.gainNode);
		this.gainNode.connect(this.audioContext.destination);
	}

	attach(): HTMLMediaElement {
		const audioElement = document.createElement("audio");
		audioElement.autoplay = true;
		const source = this.audioContext.createMediaStreamSource(
			new MediaStream([this.consumer.track]),
		);
		source.connect(this.analyser);
		this.elements.push(audioElement);
		return audioElement;
	}

	detach(): HTMLMediaElement[] {
		const detached = [...this.elements];
		this.elements = [];
		for (const element of detached) {
			element.pause();
			element.remove();
		}
		return detached;
	}

	setVolume(volume: number): void {
		this.gainNode.gain.value = Math.max(0, Math.min(1, volume));
	}

	getLevel(): number {
		this.analyser.getByteTimeDomainData(this.levelBuffer);
		let sumSquares = 0;
		for (let i = 0; i < this.levelBuffer.length; i++) {
			const v = (this.levelBuffer[i] - 128) / 128;
			sumSquares += v * v;
		}
		return Math.sqrt(sumSquares / this.levelBuffer.length);
	}

	stop(): void {
		this.consumer.close();
		this.detach();
		this.audioContext.close();
	}
}
```

- [ ] **Step 2: MediaSoupSFUClient 加 activeSpeakerTimer 字段**

在 `mediasoup-client.ts:83-86` 字段块(`onProducerClosedBound` 等)后加:

```ts
	private activeSpeakerTimer: ReturnType<typeof setInterval> | null = null;
```

- [ ] **Step 3: joinRoom 启动 active speaker timer**

在 `joinRoom` 末尾(`this.recvTransport?.on("connectionstatechange", ...)` 之后,line 188 后)加:

```ts
		this.activeSpeakerTimer = setInterval(() => {
			if (this.remoteTracks.size === 0) return;
			let loudest: { identity: string; level: number } | null = null;
			for (const [identity, track] of this.remoteTracks) {
				const level = track.getLevel();
				if (!loudest || level > loudest.level) {
					loudest = { identity, level };
				}
			}
			if (loudest && loudest.level > 0.01) {
				this.onActiveSpeakersCb?.([loudest.identity]);
			} else {
				this.onActiveSpeakersCb?.([]);
			}
		}, 500);
```

- [ ] **Step 4: leaveRoom 清 timer**

在 `leaveRoom` 方法开头(line 192 `this.hasJoined = false;` 之后)加:

```ts
		if (this.activeSpeakerTimer) {
			clearInterval(this.activeSpeakerTimer);
			this.activeSpeakerTimer = null;
		}
```

- [ ] **Step 5: 删除 consumeProducer 的 FIXME 回退行**

删除 `mediasoup-client.ts:350`:
```ts
		this.onActiveSpeakersCb?.(Array.from(this.remoteTracks.keys())); // FIXME: mediasoup active speaker not implemented
```

- [ ] **Step 6: CREATE_TRANSPORT payload 带 identity**

替换 `createSendTransport`(line 282)的 sfuEmit 调用:

```ts
		const data = await this.sfuEmit(MEDIASOUP_EVENTS.CREATE_TRANSPORT, { room: this.roomId, direction: "send", identity: this.identity });
```

替换 `createRecvTransport`(line 309)的 sfuEmit 调用:

```ts
		const data = await this.sfuEmit(MEDIASOUP_EVENTS.CREATE_TRANSPORT, { room: this.roomId, direction: "recv", identity: this.identity });
```

- [ ] **Step 7: 类型检查 + lint**

Run: `cd packages/sfu-client && pnpm exec tsc --noEmit`
Expected: 退出码 0。

Run: `pnpm --filter @go-rtc/web check 2>/dev/null; cd packages/sfu-client && pnpm exec biome check src/mediasoup-client.ts 2>/dev/null || true`
(若 sfu-client 无 biome 配置,跳过;主检 tsc。)

- [ ] **Step 8: 重新构建 sfu-client dist(web 依赖)**

Run: `cd packages/sfu-client && pnpm build`
Expected: dist/ 更新,无错误。

- [ ] **Step 9: Commit**

```bash
git add packages/sfu-client/src/mediasoup-client.ts packages/sfu-client/dist/
git commit -m "feat(sfu-client): mediasoup active speaker WebAudio 检测 + transport 带 identity"
```

---

## Task 8: 前端 leaveRoom 发 close-transport 事件

**Files:**
- Modify: `packages/sfu-client/src/mediasoup-client.ts`

**Interfaces:**
- Consumes: Task 5 的 `sfu:close-transport` 事件。
- Produces: 前端显式离开时通知后端清理(配合 socket disconnect 兜底)。

- [ ] **Step 1: MEDIASOUP_EVENTS 加 CLOSE_TRANSPORT**

`mediasoup-client.ts:10-18` 的 `MEDIASOUP_EVENTS` 加:

```ts
	CLOSE_TRANSPORT: "sfu:close-transport",
```

- [ ] **Step 2: leaveRoom 发 close-transport**

在 `leaveRoom` 的 timer 清理之后、producer.close 之前加:

```ts
		if (this.socket && this.roomId && this.identity) {
			try {
				await this.sfuEmit(MEDIASOUP_EVENTS.CLOSE_TRANSPORT, { room: this.roomId, identity: this.identity });
			} catch {
				// 离开时忽略清理错误,socket 可能已断
			}
		}
```

- [ ] **Step 3: 类型检查 + 重建**

Run: `cd packages/sfu-client && pnpm exec tsc --noEmit && pnpm build`
Expected: 退出码 0。

- [ ] **Step 4: Commit**

```bash
git add packages/sfu-client/src/mediasoup-client.ts packages/sfu-client/dist/
git commit -m "feat(sfu-client): mediasoup leaveRoom 发 close-transport 通知后端清理"
```

---

## Task 9: docker-compose mediasoup-worker 注释块补 ANNOUNCED_IP

**Files:**
- Modify: `deploy/docker-compose.example.yml`

- [ ] **Step 1: 查看现有 mediasoup-worker 注释块**

Run: `sed -n '140,165p' deploy/docker-compose.example.yml`

- [ ] **Step 2: 补 ANNOUNCED_IP + RTC port 映射说明**

在 mediasoup-worker 注释块 environment 段加(若已有则确认):

```yaml
  # mediasoup-worker:
  #   build:
  #     context: .
  #     dockerfile: deploy/mediasoup-worker/Dockerfile
  #   container_name: gospeak-mediasoup-worker
  #   ports:
  #     - "3001:3001"
  #     - "40000-49999:40000-49999/udp"
  #   environment:
  #     LISTEN_IP: 0.0.0.0
  #     ANNOUNCED_IP: 127.0.0.1   # LAN 部署改为宿主内网 IP,否则浏览器 ICE 不可达
  #     RTC_MIN_PORT: 40000
  #     RTC_MAX_PORT: 49999
  #     PORT: 3001
```

注:实际内容根据 Step 1 查看结果调整,保留注释状态(默认不起),仅补 ANNOUNCED_IP 行 + ports。

- [ ] **Step 3: yaml 合法性**

Run: `python3 -c "import yaml; yaml.safe_load(open('deploy/docker-compose.example.yml'))" && echo OK`
Expected: `OK`。

- [ ] **Step 4: Commit**

```bash
git add deploy/docker-compose.example.yml
git commit -m "docs(deploy): mediasoup-worker 注释块补 ANNOUNCED_IP + RTC 端口"
```

---

## Task 10: mediasoup-selfhost-runbook

**Files:**
- Create: `docs/mediasoup-selfhost-runbook.md`

- [ ] **Step 1: 写 runbook(仿 srs-selfhost-runbook.md 结构)**

新建 `docs/mediasoup-selfhost-runbook.md`:

````markdown
# MediaSoup 自部署端到端 Runbook

注意: mediasoup-worker 是独立 Node 进程,Go 后端经 HTTP bridge 与之通信。浏览器 WebRTC 媒体直连 worker 的 RTC 端口(40000-49999/udp),不经 Go 后端。`ANNOUNCED_IP` 必须为浏览器可达地址 — dev 单机用 127.0.0.1,LAN 部署用宿主内网 IP,否则 ICE candidate 不可达。

dev 环境(浏览器与 docker 同宿主)。LAN 部署见末节。

信令(socket.io `sfu:*` 事件)走现有 socket.io 连接,与 LiveKit/SRS 共用同一 WS 通道,无额外代理。

## 1. 起 mediasoup-worker

```bash
# 方式 A: docker(取消 deploy/docker-compose.example.yml mediasoup-worker 注释块后)
docker compose -f deploy/docker-compose.example.yml up -d mediasoup-worker
curl -s http://localhost:3001/health   # 期望 {"ok":true,...}

# 方式 B: 本地 node(开发调试)
cd packages/mediasoup-worker
pnpm install
ANNOUNCED_IP=127.0.0.1 LISTEN_IP=0.0.0.0 pnpm start
```

## 2. 后端切 mediasoup

编辑 `app/server/.env.dev`:
- 注释 `SFU_PROVIDER="livekit"` 行
- 设 `SFU_PROVIDER="mediasoup"`
- 确认 `MEDIASOUP_BRIDGE_URL="http://localhost:3001"`(默认值已对)

启动:
```bash
pnpm dev:server
```

## 3. 前端切 mediasoup

新建 `app/web/.env.local`(已 gitignore):
```
VITE_SFU_PROVIDER=mediasoup
```

启动:
```bash
pnpm dev:web
```

## 4. 双向音频验证

1. 浏览器 A 开 `http://localhost:<vite端口>/room/<room-id>`,授权麦克风,加入。
2. 浏览器 B(不同 profile 或机)同房间加入。
3. A 发声 → B 听到;B 发声 → A 听到。
4. A 离开(关 tab)→ B 侧 `onRemoteAudioTrackRemovedCb` 触发,A 音轨停止播放,AudioContext 释放。
5. 服务端 mute 验证:调 `POST /api/v1/.../mute` mute A → B 听不到 A;unmute 恢复。
6. active speaker:仅 A 发声 → B 侧 `onActiveSpeakersCb(["A"])`;无人发声 → `[]`。

## 5. LAN 部署

- worker `ANNOUNCED_IP` 设宿主内网 IP(如 192.168.1.10)。
- 宿主防火墙放行 3001/tcp + 40000-49999/udp。
- 浏览器与 worker 不同机时,RTC udp 必须可达 ANNOUNCED_IP。

## 6. mac UDP 异常注意

mac 上 docker 的 udp 转发在某些 Docker Desktop 版本表现异常(RTP 丢包/无音频)。若遇:
- 优先用方式 B(本地 node worker),绕过 docker udp。
- 或升级 Docker Desktop 至最新版。
- 排查:浏览器 chrome://webrtc-internals 看 ICE candidate 连通性。
````

- [ ] **Step 2: Commit**

```bash
git add docs/mediasoup-selfhost-runbook.md
git commit -m "docs(mediasoup): 自部署 e2e runbook"
```

---

## Task 11: 更新 sfu-provider-maturity.md

**Files:**
- Modify: `docs/sfu-provider-maturity.md`

- [ ] **Step 1: 更新方法覆盖矩阵 MediaSoup 列**

`docs/sfu-provider-maturity.md` 表格 MediaSoup 列:
- `ListParticipants`: ❌ → ✅
- `MuteParticipant`: ❌ → ✅(producer pause/resume)
- `MuteRoomParticipant`: ❌ → ✅
- `RemoveParticipant`: ❌ → ✅(close transport)

- [ ] **Step 2: 更新 MediaSoup 详细缺口段**

替换 `docs/sfu-provider-maturity.md` 中 MediaSoup 详细缺口表(4 行 notSupported)为:

```markdown
MediaSoup 已实现全部 participant 相关方法:
- `ListParticipants` — bridge 转发 worker participant 索引
- `MuteParticipant` — producer pause/resume(trackSid 当 producerId;空则批量)
- `MuteRoomParticipant` — 批量 pause/resume 该 identity 所有 producer
- `RemoveParticipant` — close 该 identity 的 transport(级联关 producer)

MediaSoup 仍通过自有信令路径([signal.go](/Users/noelorin/GOSpeak/app/server/internal/mediasoup/signal.go))协商媒体,并实现 `ParticipantCleanupHandler` 接口,在 Hub OnDisconnect 时广播 `sfu:producer-closed` + 清理 worker transport。active speaker 由前端 WebAudio AnalyserNode 检测(sfu-client),非服务端 observer。
```

- [ ] **Step 3: 更新评估时间**

`docs/sfu-provider-maturity.md` 顶部 `**评估时间**: 2026-07-02` → `2026-07-06`。

- [ ] **Step 4: Commit**

```bash
git add docs/sfu-provider-maturity.md
git commit -m "docs: 更新 mediasoup 成熟度矩阵(ListParticipants/Mute/Remove 已实现)"
```

---

## Task 12: e2e 验证 + 收尾

**Files:** 无新文件(验证 + 日志)

- [ ] **Step 1: 起 mediasoup-worker**

Run: `cd packages/mediasoup-worker && ANNOUNCED_IP=127.0.0.1 LISTEN_IP=0.0.0.0 pnpm start &`
(后台,或单独终端)

- [ ] **Step 2: 切环境 + 启后端**

编辑 `app/server/.env.dev` 设 `SFU_PROVIDER=mediasoup`。
Run: `pnpm dev:server`
确认日志无 mediasoup bridge 连接错误。

- [ ] **Step 3: 切前端 + 启 web**

新建 `app/web/.env.local` 设 `VITE_SFU_PROVIDER=mediasoup`。
Run: `pnpm dev:web`

- [ ] **Step 4: 双浏览器 tab 验证双向音频 + 离开清理 + mute + active speaker**

按 runbook 第 4 节步骤逐项验证。记录结果到 `agent_test_logs/mediasoup-e2e-<时间>.md`(模板见 AGENTS.md §Test Logging)。

- [ ] **Step 5: 全量 Go 测试**

Run: `cd app/server && go test ./internal/mediasoup/ ./internal/signal/ -v`
Expected: 全 PASS。

- [ ] **Step 6: 全量编译 + lint**

Run: `cd app/server && go build ./...`
Expected: 退出码 0。

Run: `cd packages/mediasoup-worker && pnpm exec tsc --noEmit && cd ../../packages/sfu-client && pnpm exec tsc --noEmit`
Expected: 退出码 0。

- [ ] **Step 7: 恢复 .env.dev 默认 provider(若需)**

验证完视情况恢复 `SFU_PROVIDER=livekit`。.env.local 保留或删(已 gitignore)。

- [ ] **Step 8: 最终 commit(测试日志)**

```bash
git add agent_test_logs/mediasoup-e2e-*.md
git commit -m "test(mediasoup): e2e 验证日志 — participant清理/mute/active speaker"
```

---

## Self-Review 结果

**1. Spec coverage:**
- participant 模型 + producer-closed 广播 → Task 1,2,4,5,8 ✅
- 服务端 mute → Task 1(pause/resume),3(bridge),6(provider) ✅
- active speaker → Task 7 ✅
- e2e runbook → Task 9,10 ✅
- ListParticipants/RemoveParticipant → Task 1,3,6 ✅
- 不影响其它 SFU → Task 4(接口类型断言,Global Constraints) ✅

**2. Placeholder scan:** 无 TBD/TODO。所有代码块完整。

**3. Type consistency:**
- `ParticipantInfo`(bridge.go) ↔ worker listParticipants 返回结构 ✅
- `participantBridge`/`providerBridge` 接口方法名与 bridge.go 方法一致(ListParticipants/CloseParticipant/PauseProducer/ResumeProducer/PauseParticipant/ResumeParticipant) ✅
- `OnParticipantLeft(room, identity)` signal.go ↔ hub.go ParticipantCleanupHandler ✅
- `closeProducerIds` JSON tag ↔ bridge CloseParticipant result struct ✅
- `MEDIASOUP_EVENTS.CLOSE_TRANSPORT` ↔ signal.go `sfu:close-transport` 字符串 ✅

注:Task 3 Step 8 临时修复 signal.go 使每提交可编译,与 Task 5 重构一致(produce handler 后续加 socketIndex 登记不冲突)。
