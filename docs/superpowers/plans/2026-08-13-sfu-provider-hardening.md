# SFU Provider 加固 Implementation Plan

> **Status (2026-08-13):** 🟠 进行中 (2026-08-13) — 代码已 staged 未提交 (srs/cloudflare provider/hub_mute/capabilities)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复提交 74c835a 引入的 SRS / Cloudflare provider 质量与边界问题（🔵 全量 + 非核心 🟡），包括 HTTP 状态检查、API 失败回退、N+1 反查、删房一致化、跨实例语义诚实化、广播定向化、注入加锁、能力口径对齐、agora 禁用对称、文档清理。

**Architecture:** 全部改动位于 Go 后端 `app/server/internal/`。遵循"诚实上报能力/失败，不做静默假成功"原则：SRS API 失败回退 registry、Cloudflare 本地 miss 返回 `ErrSFUNotSupported`、删除房间部分失败不再清理登记、`member:muted` 从全局广播改为按房间定向广播。性能项通过 provider 内短 TTL 缓存收敛 N+1。

**Tech Stack:** Go 1.2x、Gin、NATS JetStream KV、httptest、`-race`。

---

## Findings 覆盖表（Review → Task）

| Review finding | Task |
|---|---|
| 🔵 `srs/client.go:81/86/92` ListStreams/fetchClients 未检查 HTTP 状态码 | T1 |
| 🟡 `srs/provider.go:78` resolver 注入后 SRS 宕机 502、无 registry 回退 | T2 |
| 🔵 `srs/provider.go:109` MemberCount 未按 identity 去重 | T3 |
| 🔵 `srs/provider.go:94/95` 三处 stream 过滤循环重复 | T4 |
| 🟡 `srs/provider.go:105/161/164` 逐流 RoomForStream N+1 | T5 |
| 🟡 `srs/provider.go:322/305` deleteRoomFromSRS TOCTOU/部分失败 | T6 |
| 🔵 `srs/provider.go:326/328` 部分失败死分支 | T6 |
| 🔵 `cloudflare/client.go:66/68` GetSession/GetSessionTracks 重复 | T7 |
| 🔵 `cloudflare/provider.go:126` getSession miss 无日志 | T7 |
| 🔵 `cloudflare/provider.go:146` TrackState.Status 未过滤 | T7 |
| 🔵 `cloudflare/provider.go:141` GetSessionTracks/CloseTracks TOCTOU | T7 |
| 🟡 `cloudflare/provider.go:128/129` unmute 裸 ErrSFUNotSupported、媒体层 no-op | T8 |
| 🟡 `cloudflare/provider.go:134` 跨实例 session miss 静默假成功 | T8 |
| 🟡 `signal/hub_mute.go:38/55` identityForUserID 重复 DB 查询 | T9 |
| 🟡/🔵 `signal/hub_mute.go:38/39` member:muted 全局广播跨租户泄漏 | T9 |
| 🔵 `srs/provider.go:30` Set* 注入无锁数据竞争 | T10 |
| 🔵 `dynamic_provider.go:72` SetStreamRoomResolver 重建无测试 | T10 |
| 🟡 `capabilities.go:26` SRS ListLevel=hard 过度承诺 | T11 |
| 🟡 `provider.ts:4/5` agora 仅前端禁用、后端可 switch | T12 |
| 🟡 `srs/provider.go:228` 旧客户端不处理 member:muted | T13 |
| 🔵 `app/docs/sfu/*.md`、`docs/sfu-provider-maturity.md` GenerateAdminToken 过期 | T13 |
| 🔵 `capabilities_test.go:27` gofmt 缩进错误 | T14 |

---

### Task 1: SRS client 检查 HTTP 状态码

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/client.go`
- Test: `app/server/internal/sfu/providers/srs/client_test.go`

背景：`ListStreams`/`fetchClients` 未检查 `resp.StatusCode`，非 2xx 但 body 可解码（如网关 502 返回 `{"code":0}`）时静默返回空列表，房间/参与者被误判为无。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/sfu/providers/srs/client_test.go` 新增（参考现有 httptest 模式）：

```go
func TestClient_ListStreams_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":0,"streams":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.ListStreams(); err == nil {
		t.Fatal("ListStreams should error on non-2xx response")
	}
}
```

确保 `client_test.go` 已 import `net/http`、`net/http/httptest`；缺失则补充。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestClient_ListStreams_Non2xx_ReturnsError -v`
Expected: FAIL，当前返回空列表无错误。

- [ ] **Step 3: 最小实现**

修改 `app/server/internal/sfu/providers/srs/client.go` 的 `ListStreams` 与 `fetchClients`，在 `http.Do` 后立即检查：

```go
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("srs list streams: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("srs list streams: unexpected status=%d", resp.StatusCode)
	}
```

`fetchClients` 同位置加同样的状态检查（错误文案 `"srs list clients: unexpected status=%d"`）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestClient_' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/client.go app/server/internal/sfu/providers/srs/client_test.go
git commit -m "fix(srs): error on non-2xx responses from SRS HTTP API"
```

---

### Task 2: SRS API 失败回退 registry

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：`gin.go` 无条件注入 resolver 后，`ListRooms`/`ListParticipants`/`DeleteRoom` 永远走 SRS HTTP API；SRS 宕机时直接 502，本地 registry 聚合路径被旁路。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/sfu/providers/srs/provider_test.go` 新增：

```go
func TestListRooms_FallsBackToRegistry_WhenSRSUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "srs down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := &config.Config{SRSHost: "127.0.0.1", SRSApiPort: "1"} // 端口 1 无服务
	_ = srv
	svc := NewService(cfg)
	svc.SetRoomRegistry(&stubRegistry{
		rooms:   []string{"dom-a:r1"},
		streams: map[string][]string{"dom-a:r1": {"gs-1"}},
	})
	svc.SetStreamRoomResolver(&stubResolver{})

	rooms, err := svc.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms should fall back to registry, got err=%v", err)
	}
	if len(rooms) != 1 || rooms[0].Name != "dom-a:r1" {
		t.Fatalf("expected registry room dom-a:r1, got %+v", rooms)
	}
}
```

`SRSApiPort` 用不可能服务的端口（如 `"1"`）保证 `ListStreams` 连接失败；`httptest` import 已存在。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestListRooms_FallsBackToRegistry_WhenSRSUnavailable -v`
Expected: FAIL，返回 `SFU_ERROR` 而非 registry 结果。

- [ ] **Step 3: 最小实现**

修改 `app/server/internal/sfu/providers/srs/provider.go` 的 `ListRooms`：

```go
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.resolver != nil {
		rooms, err := s.listRoomsFromSRS()
		if err == nil {
			return rooms, nil
		}
		log.Printf("[srs] list rooms from SRS API failed, fallback to registry: %v", err)
	}
	// 降级：SRS API 不可达时保持 registry 聚合（与 resolver 未注入时行为一致）。
	if s.registry == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "srs room registry not configured")
	}
	rooms := s.registry.Rooms()
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		streams := s.registry.Streams(name)
		out = append(out, sfu.RoomSummary{Name: name, MemberCount: len(streams)})
	}
	return out, nil
}
```

`ListParticipants` 与 `DeleteRoom` 同样加回退：`listParticipantsFromSRS`/`deleteRoomFromSRS` 出错时记日志并落入原 registry 分支。新增 import：`log`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListRooms_FallsBackToRegistry_WhenSRSUnavailable|TestListRooms|TestListParticipants|TestDeleteRoom' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "fix(srs): fall back to registry when SRS API is unavailable"
```

---

### Task 3: listRoomsFromSRS 按 identity 去重

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：同一 identity 多 tab/多 stream 时 `MemberCount` 按 stream 计数虚高，与 `listParticipantsFromSRS` 的 identity 去重口径不一致。

- [ ] **Step 1: 写失败测试**

```go
func TestListRooms_MemberCount_DeduplicatedByIdentity(t *testing.T) {
	svc := NewService(&config.Config{})
	reg := &stubRegistry{identityStreams: map[string]string{
		"dom-a:r1\x00alice": "gs-a1",
		"dom-a:r1\x00alice": "gs-a2", // 同 identity 双 stream
		"dom-a:r1\x00bob":   "gs-b1",
	}}
	svc.SetRoomRegistry(reg)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1", "gs-a2": "dom-a:r1", "gs-b1": "dom-a:r1",
	}})

	rooms, err := svc.listRoomsFromSRS()
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) != 1 || rooms[0].MemberCount != 2 {
		t.Fatalf("expected 2 unique members in dom-a:r1, got %+v", rooms)
	}
}
```

`listRoomsFromSRS` 需要 `s.client` 指向可控 server：用 `srsTestServer`（`newSRSTestServer()` + `svc.client = NewClient(ts.srv.URL)`），并 `ts.streams = []string{"gs-a1","gs-a2","gs-b1"}`。按现有测试文件中的 server 构造方式补齐。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestListRooms_MemberCount_DeduplicatedByIdentity -v`
Expected: FAIL，`MemberCount = 3`。

- [ ] **Step 3: 最小实现**

```go
func (s *Service) listRoomsFromSRS() ([]sfu.RoomSummary, error) {
	streams, err := s.client.ListStreams()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	// room -> identity 集合，按 identity 去重后再计数（与 ListParticipants 口径一致）。
	rooms := make(map[string]map[string]struct{})
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		candidate, ok := s.resolver.RoomForStream(stream)
		if !ok || candidate == "" {
			continue
		}
		identity := ""
		if s.registry != nil {
			if id, ok := s.registry.IdentityForStream(candidate, stream); ok {
				identity = id
			}
		}
		if identity == "" {
			identity = strings.TrimPrefix(stream, streamNamePrefix)
		}
		if rooms[candidate] == nil {
			rooms[candidate] = make(map[string]struct{})
		}
		rooms[candidate][identity] = struct{}{}
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for room, members := range rooms {
		out = append(out, sfu.RoomSummary{Name: room, MemberCount: len(members)})
	}
	return out, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListRooms' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "fix(srs): dedupe room member count by identity"
```

---

### Task 4: 抽取 collectRoomStreams 统一过滤逻辑

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：`listRoomsFromSRS`/`listParticipantsFromSRS`/`deleteRoomFromSRS` 三处重复"前缀过滤 + RoomForStream + roomMatches"。

- [ ] **Step 1: 写失败测试**

```go
func TestCollectRoomStreams_FiltersByPrefixAndRoom(t *testing.T) {
	svc := NewService(&config.Config{})
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1", "gs-a2": "dom-a:r1", "gs-b1": "dom-b:r1",
	}})
	got := svc.collectRoomStreams([]string{"gs-a1", "gs-a2", "gs-b1", "x-other"}, "dom-a:r1")
	if len(got) != 2 {
		t.Fatalf("expected 2 streams for dom-a:r1, got %+v", got)
	}
	if got[0].stream != "gs-a1" || got[1].stream != "gs-a2" {
		t.Fatalf("unexpected streams: %+v", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestCollectRoomStreams_FiltersByPrefixAndRoom -v`
Expected: 编译失败（`collectRoomStreams` 未定义）。

- [ ] **Step 3: 最小实现**

在 `app/server/internal/sfu/providers/srs/provider.go` 增加：

```go
// roomStream 记录属于某 room 的 SRS 业务流及其反查出的房间键。
type roomStream struct {
	stream  string
	roomKey string
}

// collectRoomStreams 从 SRS 全量流中筛选属于 room 的流（统一三处调用点的过滤逻辑）。
func (s *Service) collectRoomStreams(streams []string, room string) []roomStream {
	out := make([]roomStream, 0, len(streams))
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		candidate, ok := s.resolver.RoomForStream(stream)
		if !ok || !roomMatches(candidate, room) {
			continue
		}
		out = append(out, roomStream{stream: stream, roomKey: candidate})
	}
	return out
}
```

将 `listRoomsFromSRS`/`listParticipantsFromSRS`/`deleteRoomFromSRS` 三处循环替换为调用 `collectRoomStreams`（`listParticipantsFromSRS` 与 `deleteRoomFromSRS` 使用 `rs.roomKey`/`rs.stream`）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestCollectRoomStreams|TestListRooms|TestListParticipants|TestDeleteRoom' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "refactor(srs): extract shared room stream filtering helper"
```

---

### Task 5: stream→room 短 TTL 缓存（收敛 N+1）

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：`listRoomsFromSRS`/`listParticipantsFromSRS` 对每流调用 `resolver.RoomForStream`，cache miss 时逐流走 membership KV，活跃流多时 N+1 放大。

- [ ] **Step 1: 写失败测试**

```go
type countingResolver struct {
	stubResolver
	calls int
}

func (r *countingResolver) RoomForStream(stream string) (string, bool) {
	r.calls++
	return r.stubResolver.RoomForStream(stream)
}

func TestStreamRoomCache_ReusesLookupsAcrossCalls(t *testing.T) {
	ts := newSRSTestServer()
	ts.streams = []string{"gs-a1"}
	defer ts.srv.Close()

	resolver := &countingResolver{stubResolver: stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1",
	}}}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetStreamRoomResolver(resolver)
	svc.SetRoomRegistry(&stubRegistry{})

	if _, err := svc.listRoomsFromSRS(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.listParticipantsFromSRS("dom-a:r1"); err != nil {
		t.Fatal(err)
	}
	if resolver.calls > 2 {
		t.Fatalf("expected <=2 RoomForStream calls across two lists, got %d", resolver.calls)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestStreamRoomCache_ReusesLookupsAcrossCalls -v`
Expected: FAIL，两次调用共 2 次反查（当前无缓存，两处各自反查）。

- [ ] **Step 3: 最小实现**

在 `Service` 增加缓存字段并在 `NewService` 初始化：

```go
type streamRoomEntry struct {
	room  string
	found bool
	at    time.Time
}

// streamRoomCacheTTL 控制 stream→room 反查缓存窗口；缓存只为收敛 N+1，允许短暂陈旧。
const streamRoomCacheTTL = 5 * time.Second
```

`Service` 新增字段：`streamCache map[string]streamRoomEntry`、`cacheMu sync.Mutex`；`NewService` 初始化 `streamCache: make(map[string]streamRoomEntry)`。新增方法：

```go
// resolveStreamRoom 优先读短 TTL 缓存，miss 时经 resolver 反查并回填。
func (s *Service) resolveStreamRoom(stream string) (string, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if entry, ok := s.streamCache[stream]; ok && time.Since(entry.at) < streamRoomCacheTTL {
		return entry.room, entry.found
	}
	if s.resolver == nil {
		return "", false
	}
	room, ok := s.resolver.RoomForStream(stream)
	s.streamCache[stream] = streamRoomEntry{room: room, found: ok, at: time.Now()}
	return room, ok
}
```

`collectRoomStreams` 与 `listRoomsFromSRS` 中的 `s.resolver.RoomForStream(stream)` 改为 `s.resolveStreamRoom(stream)`。新增 import：`sync`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestStreamRoomCache|TestListRooms|TestListParticipants|TestDeleteRoom' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "perf(srs): cache stream-to-room lookups to avoid N+1"
```

---

### Task 6: deleteRoomFromSRS 一致化（TOCTOU + 部分失败）

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：`KickByStreams` 在 `remaining>0` 时必返回 error，原 `remaining>0 && kicked==0` 分支不可达；`kicked>0 && remaining>0` 时仍 `ClearRoom` 返回成功（残留流被误判删除）；ListStreams 快照与 Kick 之间新加入的流会被 `ClearRoom` 连带清掉登记，形成孤儿流。

- [ ] **Step 1: 写失败测试**

```go
func TestDeleteRoom_PartialKick_DoesNotClearRegistry(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.srv.Close()
	ts.streams = []string{"gs-a1", "gs-a2"}
	ts.clients = []clientsResponseClient{
		{ID: "cid-1", Name: "gs-a1"},
		{ID: "cid-2", Name: "gs-a2"},
	}
	ts.kickFail["cid-2"] = true // 按 client id 模拟第二流踢失败（返回 code 2049）

	reg := &stubRegistry{
		streams: map[string][]string{"dom-a:r1": {"gs-a1", "gs-a2"}},
	}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetRoomRegistry(reg)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1", "gs-a2": "dom-a:r1",
	}})

	err := svc.DeleteRoom("dom-a:r1")
	if err == nil {
		t.Fatal("partial kick must surface as error")
	}
	if len(reg.cleared) != 0 {
		t.Fatalf("registry must NOT be cleared on partial failure, cleared=%v", reg.cleared)
	}
}
```

`srsTestServer` 的 `kickFail` 按 client id 匹配（`provider_test.go:105`），`KickByStreams` 按业务流名解析 id（`TestKickByStreams_UsesNameNotInternalStreamID` 已验证），因此 `ts.clients` 需同时设置。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestDeleteRoom_PartialKick_DoesNotClearRegistry -v`
Expected: FAIL（当前部分失败仍 ClearRoom 或错误未携带计数）。

- [ ] **Step 3: 最小实现**

```go
func (s *Service) deleteRoomFromSRS(room string) error {
	streams, err := s.client.ListStreams()
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	roomStreams := s.collectRoomStreams(streams, room)
	targets := make([]string, 0, len(roomStreams))
	for _, rs := range roomStreams {
		targets = append(targets, rs.stream)
	}
	if len(targets) == 0 {
		return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
	}
	kicked, remaining, err := s.client.KickByStreams(targets)
	if err != nil {
		// KickByStreams 在 remaining>0 时返回 error；把计数带进错误，避免静默丢部分失败。
		return pkg.NewAppError(pkg.SFU_ERROR,
			fmt.Sprintf("srs delete room partial failure: kicked=%d remaining=%d: %v", kicked, remaining, err))
	}
	if kicked == 0 {
		return pkg.NewAppError(pkg.SFU_ERROR, "srs delete room: no stream kicked")
	}
	// kick 后复查：KickByStreams 与 ClearRoom 之间新加入的流不能被连带清理。
	after, listErr := s.client.ListStreams()
	if listErr == nil && len(s.collectRoomStreams(after, room)) > 0 {
		return pkg.NewAppError(pkg.SFU_ERROR, "srs delete room partial failure: new streams joined during kick")
	}
	if s.registry != nil {
		s.registry.ClearRoom(room)
	}
	return nil
}
```

删除原不可达分支 `if remaining > 0 && kicked == 0`。`fmt` 已 import。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestDeleteRoom' -v`
Expected: 全部 PASS（既有 DeleteRoom 用例使用可全部踢掉的场景，不受影响）。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "fix(srs): surface partial kick failures and re-check before clearing room"
```

---

### Task 7: Cloudflare client/impl 加固

**Files:**
- Modify: `app/server/internal/sfu/providers/cloudflare/client.go`
- Modify: `app/server/internal/sfu/providers/cloudflare/provider.go`
- Test: `app/server/internal/sfu/providers/cloudflare/provider_test.go`

背景与修复项：
- 🔵 `GetSession`/`GetSessionTracks` URL 拼接重复 → 提取 `sessionPath`。
- 🔵 `getSession` miss 静默无日志 → 记录 debug 日志。
- 🔵 `TrackState.Status` 未过滤 → 排除非活跃本地轨道。
- 🔵 `GetSessionTracks`→`CloseTracks` TOCTOU → CloseTracks 失败后复查，已无 local track 视为成功。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/sfu/providers/cloudflare/provider_test.go` 按现有模式新增（用 httptest 模拟 CF API；若该文件已有 server stub 则复用）：

```go
func TestMuteParticipant_SkipsClosedTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apps/test-app/sessions/sess-1" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(SessionStateResponse{Tracks: []TrackState{
				{Location: "local", MID: "m-active", Status: "active"},
				{Location: "local", MID: "m-closed", Status: "closed"},
			}})
		case r.URL.Path == "/apps/test-app/sessions/sess-1/tracks/close" && r.Method == http.MethodPut:
			var req CloseTrackRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode close request: %v", err)
			}
			if len(req.Tracks) != 1 || req.Tracks[0].MID != "m-active" {
				t.Fatalf("close specs = %+v, want only active mid", req.Tracks)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CloseTrackResponse{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := NewService(&config.Config{CFAppID: "test-app", CFAppSecret: "test-secret"})
	svc.client.baseURL = server.URL
	svc.client.httpClient = http.DefaultClient
	svc.putSession("r1", "alice", "sess-1", 1, "")

	if err := svc.MuteParticipant("r1", "alice", "", true); err != nil {
		t.Fatalf("MuteParticipant failed: %v", err)
	}
}
```

测试目标：CloseTracks 请求体 `tracks[].mid` 只含 `Status != "closed"` 的轨道（`encoding/json`/`net/http` 已 import）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/cloudflare/ -run TestMuteParticipant_SkipsClosedTracks -v`
Expected: FAIL（当前把 closed 轨道也加入 close specs）。

- [ ] **Step 3: 最小实现**

`client.go` 增加：

```go
// sessionPath 是 Cloudflare Realtime session 相关接口的公共路径。
func (c *Client) sessionPath(sessionID string) string {
	return fmt.Sprintf("/apps/%s/sessions/%s", c.appID, sessionID)
}
```

`GetSession`/`GetSessionTracks` 改用 `c.sessionPath(sessionID)`。

`provider.go` 的 `MuteParticipant` 中过滤轨道并加复查：

```go
	state, err := s.client.GetSessionTracks(meta.sessionID)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare get session state: "+err.Error())
	}
	specs := make([]CloseTrackSpec, 0, len(state.Tracks))
	for _, tr := range state.Tracks {
		if tr.Location != "local" || tr.MID == "" {
			continue
		}
		if tr.Status == "closed" {
			// 已关闭轨道不再 CloseTracks，避免重复关闭报错。
			continue
		}
		specs = append(specs, CloseTrackSpec{MID: tr.MID})
	}
	if len(specs) == 0 {
		return nil
	}
	if _, err := s.client.CloseTracks(meta.sessionID, &CloseTrackRequest{Tracks: specs, Force: true}); err != nil {
		// TOCTOU 兜底：快照与关闭之间轨道可能已被移除；复查后若无 local track 视为成功。
		if state2, sErr := s.client.GetSessionTracks(meta.sessionID); sErr == nil {
			alive := false
			for _, tr := range state2.Tracks {
				if tr.Location == "local" && tr.Status != "closed" {
					alive = true
					break
				}
			}
			if !alive {
				return nil
			}
		}
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare close tracks: "+err.Error())
	}
	return nil
```

`getSession` miss 处（`MuteParticipant` 的 `if !ok || meta.sessionID == ""` 分支）前加日志：

```go
	if !ok {
		log.Printf("[cloudflare] session lookup miss room=%s identity=%s", room, identity)
		return nil
	}
```

确认 `log` 已 import（缺失则补充）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/cloudflare/ -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/cloudflare/client.go app/server/internal/sfu/providers/cloudflare/provider.go app/server/internal/sfu/providers/cloudflare/provider_test.go
git commit -m "fix(cloudflare): skip closed tracks, idempotent close, share session path"
```

---

### Task 8: Cloudflare 错误类型与跨实例诚实降级

**Files:**
- Modify: `app/server/internal/sfu/providers/cloudflare/provider.go`
- Test: `app/server/internal/sfu/providers/cloudflare/provider_test.go`

背景：unmute 返回裸 `pkg.ErrSFUNotSupported`（非 AppError，`HandleError` 映射成 500 而非 502）；本地 session miss 被当作"已禁言"返回 nil，跨实例媒体静音静默不生效却上报 degraded。

- [ ] **Step 1: 写失败测试**

```go
func TestMuteParticipant_Unmute_ReturnsAppError(t *testing.T) {
	svc := NewService(&config.Config{CFAppID: "app", CFAppSecret: "sec"})
	err := svc.MuteParticipant("r", "alice", "", false)
	if err == nil {
		t.Fatal("unmute should return ErrSFUNotSupported")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("unmute should return *pkg.AppError, got %T", err)
	}
}

func TestMuteParticipant_LocalSessionMiss_SoftFallback(t *testing.T) {
	svc := NewService(&config.Config{CFAppID: "app", CFAppSecret: "sec"})
	err := svc.MuteParticipant("r", "ghost", "", true)
	if !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("local miss must not fake success, err=%v", err)
	}
}
```

新增 import：`errors`、`GOSpeak/internal/pkg`（若缺失）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/cloudflare/ -run 'TestMuteParticipant_' -v`
Expected: FAIL（当前 unmute 返回裸 error、miss 返回 nil）。

- [ ] **Step 3: 最小实现**

```go
func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	if !muted {
		// 媒体层无法恢复已关闭的轨道，unmute 是 no-op；
		// 返回包装后的 ErrSFUNotSupported（AppError）让 HandleError 映射为 502，客户端按软语义重新发布。
		return pkg.NewErrSFUNotSupported()
	}
	if s.client == nil || s.appID == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID is required")
	}
	meta, ok := s.getSession(room, identity)
	if !ok || meta.sessionID == "" {
		// 本地无会话记录：可能是"从未发布"（本可视为已禁言），也可能是跨实例会话。
		// 无法确认媒体状态时诚实上报 not-supported，由调用方按 soft 语义处理，避免假 degraded 成功。
		return pkg.NewErrSFUNotSupported()
	}
	// ... 其余逻辑不变 ...
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/cloudflare/ -run 'TestMuteParticipant_' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/cloudflare/provider.go app/server/internal/sfu/providers/cloudflare/provider_test.go
git commit -m "fix(cloudflare): honest soft fallback for unmute and cross-instance session miss"
```

---

### Task 9: member:muted 定向广播 + identity 解析复用

**Files:**
- Modify: `app/server/internal/signal/hub_mute.go`
- Test: `app/server/internal/signal/hub_mute_test.go`

背景：`member:muted/unmuted` 走 `publishNamespace` 全局广播且 payload 只有 identity，跨租户泄漏禁言状态；`BroadcastMute`/`BroadcastUnmute` 各自再调一次 `identityForUserID`（DB 查询），与 `enforceUserMediaMute` 内重复。

- [ ] **Step 1: 写失败测试**

修改 `hub_mute_test.go` 的 `captureMuteBus` 记录 room 事件，并新增：

```go
func TestBroadcastMute_PublishesMemberMuted_ToMemberRooms(t *testing.T) {
	hub := NewHub(nil, nil, &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}, nil)
	bus := &captureMuteBus{}
	hub.SetEventBus(bus)

	// 让 alice 出现在一个房间的成员快照中。
	conn := newTestConn()
	_, _ = hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`)

	hub.BroadcastMute(7, &MuteInfo{Permanent: true})

	bus.mu.Lock()
	defer bus.mu.Unlock()
	if len(bus.room["dom-a:lobby"][EventMemberMuted]) != 1 {
		t.Fatalf("expected member:muted published to room dom-a:lobby, got %+v", bus.room)
	}
}
```

`captureMuteBus` 增加字段 `room map[string]map[string][]map[string]interface{}` 并在 `PublishRoom` 中记录；`newTestConn` 按 `hub_ws_test.go` 中已有 fake client 构造。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/signal/ -run TestBroadcastMute_PublishesMemberMuted_ToMemberRooms -v`
Expected: FAIL（当前走 `publishNamespace`，room 无记录）。

- [ ] **Step 3: 最小实现**

在 `hub_mute.go` 增加房间收集 helper，并改造 `BroadcastMute`/`BroadcastUnmute`：

```go
// roomsForIdentity 返回 identity 当前所在房间键（本地 rooms + 跨实例 membership KV）。
// 供禁言事件定向广播与媒体强制共用。
func (h *Hub) roomsForIdentity(identity string) []string {
	seen := make(map[string]struct{})
	rooms := make([]string, 0, 2)
	h.mu.RLock()
	for roomName, room := range h.rooms {
		if roomLookupIdentity(room, identity) != nil {
			seen[roomName] = struct{}{}
			rooms = append(rooms, roomName)
		}
	}
	h.mu.RUnlock()
	if h.membershipStore != nil {
		kvCtx, kvCancel := kvTimeoutCtx()
		names, err := h.membershipStore.ListRoomNames(kvCtx)
		kvCancel()
		if err == nil {
			for _, roomName := range names {
				if roomName == "" {
					continue
				}
				if _, ok := seen[roomName]; ok {
					continue
				}
				kvCtx2, kvCancel2 := kvTimeoutCtx()
				snap, err := h.membershipStore.GetRoomMembers(kvCtx2, roomName)
				kvCancel2()
				if err != nil {
					continue
				}
				for _, m := range snap.Members {
					if m.Identity == identity {
						seen[roomName] = struct{}{}
						rooms = append(rooms, roomName)
						break
					}
				}
			}
		}
	}
	return rooms
}
```

`BroadcastMute` 中成员事件改为按房间定向：

```go
	identity := h.identityForUserID(userID)
	if identity != "" {
		payload := map[string]interface{}{"identity": identity, "muted": true}
		rooms := h.roomsForIdentity(identity)
		if len(rooms) == 0 {
			// 用户不在任何房间（离线）：全局广播仍可达，配合 join 自愈清理。
			h.publishNamespace(EventMemberMuted, payload)
		} else {
			for _, room := range rooms {
				h.publishRoom(room, EventMemberMuted, payload)
			}
		}
	}
```

`BroadcastUnmute` 同构改为 `publishRoom` 定向（`muted:false`）。`enforceUserMediaMute` 中重复的房间收集逻辑可保留（其 target 收集含 identity 过滤，行为等价），但将 `identityForUserID` 改为一次性解析后传入（在 `BroadcastMute` 已解析处复用）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/signal/ -run 'TestBroadcastMute|TestBroadcastUnmute' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/signal/hub_mute.go app/server/internal/signal/hub_mute_test.go
git commit -m "fix(signal): scope member mute events to member rooms"
```

---

### Task 10: SRS 注入加锁 + dynamic provider 重建测试

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`
- Test: `app/server/internal/sfu/factory/dynamic_provider_test.go`

背景：`SetStreamRoomResolver`/`SetMuteRuleStore` 无锁写 `Service` 字段，与并发 `List*`/Mute 读构成数据竞争。

- [ ] **Step 1: 写失败测试**

```go
func TestService_ConcurrentInjectAndList_NoRace(t *testing.T) {
	svc := NewService(&config.Config{})
	svc.SetRoomRegistry(&stubRegistry{})
	svc.SetStreamRoomResolver(&stubResolver{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.SetStreamRoomResolver(&stubResolver{})
			svc.SetMuteRuleStore(sfu.NewMemoryMuteRuleStore())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.ListRooms()
			_, _ = svc.MuteParticipantTimed("r", "alice", "", true, 0)
		}()
	}
	wg.Wait()
}
```

`sync` 已在测试文件 import。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test -race ./internal/sfu/providers/srs/ -run TestService_ConcurrentInjectAndList_NoRace -v`
Expected: FAIL，`-race` 报告数据竞争。

- [ ] **Step 3: 最小实现**

`Service` 增加 `mu sync.RWMutex`，Setter 写锁、读取读锁：

```go
func (s *Service) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	s.mu.Lock()
	s.resolver = r
	s.mu.Unlock()
}

func (s *Service) SetMuteRuleStore(store sfu.MuteRuleStore) {
	if store == nil {
		store = sfu.NewMemoryMuteRuleStore()
	}
	s.mu.Lock()
	s.muteRules = store
	s.mu.Unlock()
}

func (s *Service) ruleStore() sfu.MuteRuleStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.muteRules == nil {
		s.muteRules = sfu.NewMemoryMuteRuleStore()
	}
	return s.muteRules
}
```

`ListRooms`/`ListParticipants`/`DeleteRoom` 读取 `s.resolver`/`s.registry` 处加 `s.mu.RLock()`（保持短临界区，`resolver.RoomForStream` 调用可在锁外执行：先读引用再解锁）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test -race ./internal/sfu/providers/srs/ -run TestService_ConcurrentInjectAndList_NoRace -v`
Expected: PASS。

- [ ] **Step 5: dynamic provider 转发与重建重注入测试**

在 `app/server/internal/sfu/factory/dynamic_provider_test.go` 新增（`pkg` 需要 import）：

```go
type stubRoomResolver struct{ room string }

func (r *stubRoomResolver) RoomForStream(stream string) (string, bool) {
	if r == nil || r.room == "" {
		return "", false
	}
	return r.room, true
}

type resolverSetterStub struct {
	stubProvider
	got pkg.StreamRoomResolver
}

func (s *resolverSetterStub) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	s.got = r
}

func TestDynamicProvider_StreamRoomResolverForwardedAndStored(t *testing.T) {
	p := newDynamicProviderWithConfig(t, "srs")
	stub := &resolverSetterStub{}
	p.mu.Lock()
	p.cachedProvider = stub
	p.mu.Unlock()

	resolver := &stubRoomResolver{room: "dom-a:r1"}
	p.SetStreamRoomResolver(resolver)

	if stub.got != resolver {
		t.Fatal("resolver must be forwarded to the cached provider")
	}
	if p.streamRoomResolver != resolver {
		t.Fatal("resolver must be stored for re-injection after provider rebuild")
	}
}
```

`stubProvider` 与 `newDynamicProviderWithConfig` 均已在同文件定义；该测试验证 `SetStreamRoomResolver` 的立即转发，并锁定 `p.streamRoomResolver`（`current()` 重建时的重注入数据源）。

Run: `cd app/server && go test ./internal/sfu/factory/ -run TestDynamic -v`
Expected: 新用例 PASS。

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go app/server/internal/sfu/factory/dynamic_provider_test.go
git commit -m "fix(srs): guard provider injection with mutex; test resolver survives rebuild"
```

---

### Task 11: ListParticipants 无法解析流 fallback（对齐 ListLevel=hard）

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：`listParticipantsFromSRS` 静默丢弃 identity 不可解析的流，`CapabilitiesFor("srs").ListLevel=hard` 承诺与实际不符。

- [ ] **Step 1: 写失败测试**

```go
func TestListParticipants_UnresolvableStream_IncludedWithPlaceholder(t *testing.T) {
	ts := newSRSTestServer()
	ts.streams = []string{"gs-orphan"}
	defer ts.srv.Close()

	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-orphan": "dom-a:r1",
	}})
	svc.SetRoomRegistry(&stubRegistry{}) // IdentityForStream 全部 miss

	parts, err := svc.listParticipantsFromSRS("dom-a:r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("unresolvable active stream must still be listed, got %+v", parts)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestListParticipants_UnresolvableStream_IncludedWithPlaceholder -v`
Expected: FAIL（当前返回空列表）。

- [ ] **Step 3: 最小实现**

`listParticipantsFromSRS` 中 identity 空时 fallback：

```go
		identity := ""
		if s.registry != nil {
			if id, ok := s.registry.IdentityForStream(rs.roomKey, rs.stream); ok {
				identity = id
			}
		}
		if identity == "" {
			// 登记丢失的孤儿流仍计入参与者：保证 ListLevel=hard 的口径是
			// "返回真实活跃发布者"，而不是静默丢失。
			identity = strings.TrimPrefix(rs.stream, streamNamePrefix)
		}
```

（该循环已改为基于 `collectRoomStreams` 的结果。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListParticipants' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "fix(srs): list unresolvable active streams to honor hard list level"
```

---

### Task 12: 后端 switch-provider 拒绝禁用 provider（agora）

**Files:**
- Modify: `app/server/internal/service/sfu_config_service.go`
- Test: `app/server/internal/service/sfu_config_service_test.go`

背景：agora 仅前端禁用，后端 `/api/v1/sfu/switch-provider` 仍可激活；切到 agora 后前端 `loadSfuClient` 抛错，全员语音不可用。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/service/sfu_config_service_test.go` 按 `TestSwitchProvider_NoConfigChange` 的构造方式新增：

```go
func TestSwitchProvider_RejectsDisabledProvider(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{SFUProvider: "livekit"}
	svc := NewSFUConfigService(repo, baseCfg)
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	_, err := svc.SwitchProvider("agora")
	if err == nil {
		t.Fatal("switch to disabled provider must fail")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.FORBIDDEN {
		t.Fatalf("expected FORBIDDEN AppError, got %v", err)
	}
}
```

`newSFUConfigTestRepo` 已在同文件定义（`TestSwitchProvider_NoConfigChange` 使用）；`pkg`/`errors` import 缺失时补充。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/service/ -run TestSwitchProvider_RejectsDisabledProvider -v`
Expected: FAIL（当前切换成功）。

- [ ] **Step 3: 最小实现**

`app/server/internal/service/sfu_config_service.go` 的 `SwitchProvider` 开头：

```go
// frontendDisabledProviders 与前端 DISABLED_SFU_PROVIDERS 对齐：
// 这些 provider 仅保留代码与类型，不允许作为激活 provider。
var frontendDisabledProviders = map[string]bool{"agora": true}

func (s *SFUConfigService) SwitchProvider(provider string) (*model.SFUConfig, error) {
	if frontendDisabledProviders[provider] {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "provider is temporarily disabled")
	}
	// ... 原逻辑 ...
```

确认 `pkg` 已 import；`errors` 在测试文件中补充。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service/ -run 'TestSwitchProvider' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/service/sfu_config_service.go app/server/internal/service/sfu_config_service_test.go
git commit -m "fix(sfu): reject switching to frontend-disabled providers"
```

---

### Task 13: 文档清理与旧客户端兼容说明

**Files:**
- Modify: `app/docs/sfu/index.md`
- Modify: `app/docs/sfu/comparison.md`
- Modify: `docs/sfu-provider-maturity.md`
- Modify: `app/server/internal/sfu/providers/srs/provider.go`（注释）

- [ ] **Step 1: 删除已移除接口的文档引用**

`app/docs/sfu/index.md` 删除第 21 行附近 `GenerateAdminToken() (string, error)` 方法行。

`app/docs/sfu/comparison.md` 删除第 8 行 `| \`GenerateAdminToken\` | ... |` 行。

`docs/sfu-provider-maturity.md` 更新第 22/34/48 行：删除 `GenerateAdminToken` 相关行/单元格，若该行承载其他语义则改写为当前接口（`StreamName`/`ClientInfo` 等）。

- [ ] **Step 2: 补充旧客户端兼容说明**

`app/server/internal/sfu/providers/srs/provider.go` 的 `MuteParticipantTimed` 注释追加：

```go
// 兼容性说明：Discord 式禁言不再踢流，媒体层拦截依赖 SRS on_publish 禁推黑名单 +
// 订阅端 member:muted 静音。旧前端若不处理 member:muted，被禁言者仍可被听到；
// 升级前端与后端需同版本部署。
```

- [ ] **Step 3: 验证**

Run: `cd /Users/noelorin/GOSpeak && rg -n "GenerateAdminToken" app/docs docs/sfu-provider-maturity.md`
Expected: 无输出（或仅剩明确的"已移除"说明）。

- [ ] **Step 4: Commit**

```bash
git add app/docs/sfu/index.md app/docs/sfu/comparison.md docs/sfu-provider-maturity.md app/server/internal/sfu/providers/srs/provider.go
git commit -m "docs(sfu): drop removed GenerateAdminToken references, note mute compat"
```

---

### Task 14: gofmt 修复与全仓格式校验

**Files:**
- Modify: `app/server/internal/sfu/capabilities_test.go`

- [ ] **Step 1: 修复缩进**

Run: `cd app/server && gofmt -w internal/sfu/capabilities_test.go`
Expected: `srs` 用例的 `want:` 块缩进修正。

- [ ] **Step 2: 验证无格式问题**

Run: `cd app/server && gofmt -l internal/ server/`
Expected: 无输出。

- [ ] **Step 3: 回归测试**

Run: `cd app/server && go test ./internal/sfu/ -v`
Expected: 全部 PASS。

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/sfu/capabilities_test.go
git commit -m "style(sfu): gofmt capabilities test"
```

---

## 验收清单

- [ ] `cd app/server && go test -race ./internal/sfu/... ./internal/signal/... ./internal/service/...` 全部通过
- [ ] `cd app/server && go build ./...` 成功
- [ ] `cd app/server && gofmt -l internal/ server/` 无输出
- [ ] `rg -n "GenerateAdminToken" app/docs docs/sfu-provider-maturity.md` 无输出

## 残余风险

- Cloudflare 真正的跨实例媒体静音需要共享 session 状态（本次只做到"诚实 soft 降级"），多实例 + Cloudflare 场景仍无媒体层强静音。
- SRS stream→room 缓存为 5s 短 TTL，极端下房间列表最多延迟 5s 反映新流。
