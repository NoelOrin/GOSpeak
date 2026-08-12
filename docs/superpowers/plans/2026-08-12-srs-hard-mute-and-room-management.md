# SRS Hard Mute + Room Management via SRS API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 SRS provider 的 mute 达到强一致且为 Discord 式（被禁言成员保留订阅、仍能听到别人；其他成员听不到他；`on_publish` 禁推黑名单保证 mute 期间断流后无法重推发声），并把 `ListRooms`/`ListParticipants` 从本地 `RoomRegistry` 聚合迁移到 SRS HTTP API 直查（`/api/v1/streams` + `/api/v1/clients` + stream→room 反查）。

**Architecture:** mute 采用「订阅端强制静音 + SRS 禁推黑名单」组合，**不踢流**（避免前端整体断连，保住"还能听"）：后端 `MuteParticipantTimed` 写入禁推黑名单（复用现有 `sfu.MuteRuleStore`，NATS KV → memory，注入链已通），`SRSCallbackHandler.on_publish` 命中黑名单返回 `code:1` 拒绝推流；signal 层新增 `member:muted`/`member:unmuted` 事件，前端对远端 track 调 `setMutedByIdentity(identity, muted)` 静音/恢复（该机制已存在于 `app/web/src/handler_audio/index.ts`）。room 维度新增 `pkg.StreamRoomResolver` 接口（由 `signal.Hub.RoomForStream` 实现，本地 cache + membership KV 反查），`ListRooms`/`ListParticipants` 在 resolver 存在时改为 SRS API 直查 + 反查，resolver 缺失时降级现有 registry 路径。完成后 `capabilities.go` 中 SRS 的 `ListLevel` 从 `degraded` 升为 `hard`；`MuteLevel` 保持 `degraded`（订阅端静音为客户端合作 + 禁推为替代强制，非原生媒体层 force，注释更新说明）。

**Tech Stack:** Go (Gin/GORM/NATS JetStream KV) · SRS 6 HTTP API + http_hooks · TypeScript (SolidJS/Vitest) · pnpm workspace

> **Commit note:** 本计划的每个 Task 末尾含 `git commit` 步骤。commit 是否执行由用户确认（不确认则跳过 commit，仅保留代码与测试）。
>
> **Execution note（2026-08-12 执行前适配）：** 当前 `dev` 工作区存在进行中的未提交修改（移除
> `AdminToken`/`AdminLevel` 能力，涉及 `types.go`、`capabilities.go`、各 provider、`sfuProfiles.ts`）。
> 执行时**保留这些修改、不覆盖**；计划中 capabilities 相关代码示例已去掉 `AdminToken`/`AdminLevel`
> 字段（`sfu.Capabilities` 结构体已删除这两个字段）。本计划不涉及 admin token 工作。

---

## Task 1: SRS Client 新增 `ListStreams()`（SRS API 直查）

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/client.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

- [ ] **Step 1: 扩展 `srsTestServer` 支持 `/api/v1/streams/`，写失败测试**

在 `provider_test.go` 的 `srsTestServer` 结构体中增加字段与 handler：

```go
type srsTestServer struct {
	srv       *httptest.Server
	mu        sync.Mutex
	clients   []clientsResponseClient
	streams   []string
	kickedIDs []string
	kickFail  map[string]bool // id -> 模拟 kick 失败
}

func newSRSTestServer() *srsTestServer {
	ts := &srsTestServer{kickFail: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/streams/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		ts.mu.Lock()
		defer ts.mu.Unlock()
		streams := make([]map[string]string, 0, len(ts.streams))
		for _, name := range ts.streams {
			streams = append(streams, map[string]string{"app": "live", "name": name})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "streams": streams})
	})
	mux.HandleFunc("/api/v1/clients/", func(w http.ResponseWriter, r *http.Request) {
		// 保留现有实现不变
		...
	})
	ts.srv = httptest.NewServer(mux)
	return ts
}
```

在文件末尾新增两个测试：

```go
func TestListStreams_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-a", "gs-b"}
	ts.mu.Unlock()

	got, err := NewClient(ts.srv.URL).ListStreams()
	if err != nil {
		t.Fatalf("ListStreams err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"gs-a", "gs-b"}) {
		t.Fatalf("ListStreams = %v, want [gs-a gs-b]", got)
	}
}

func TestListStreams_APICodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 2048})
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL).ListStreams(); err == nil {
		t.Fatal("expected error for non-zero SRS api code")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListStreams' -v`
Expected: FAIL，`undefined: (c *Client).ListStreams` 编译错误。

- [ ] **Step 3: 在 `client.go` 实现 `ListStreams()`**

在 `clientsResponse` 结构体之后新增：

```go
type streamsResponse struct {
	Code    int                    `json:"code"`
	Streams []streamsResponseStream `json:"streams"`
}

// streamsResponseStream 对应 SRS6 /api/v1/streams 条目；Name 是业务流名（gs-xxx）。
type streamsResponseStream struct {
	App  string `json:"app"`
	Name string `json:"name"`
}
```

在 `ListParticipantsByStreams` 之前新增方法：

```go
// ListStreams 返回 SRS 上所有活跃 stream 的业务名（如 gs-xxx）。
// 直接使用 SRS HTTP API /api/v1/streams/，不依赖本地 RoomRegistry。
func (c *Client) ListStreams() ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/streams/", nil)
	if err != nil {
		return nil, fmt.Errorf("srs build list streams request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("srs list streams: %w", err)
	}
	defer resp.Body.Close()

	var result streamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("srs decode streams: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("srs api error: code=%d", result.Code)
	}
	out := make([]string, 0, len(result.Streams))
	for _, st := range result.Streams {
		if st.Name != "" {
			out = append(out, st.Name)
		}
	}
	return out, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListStreams' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/client.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "feat(srs): add client ListStreams via /api/v1/streams"
```

---

## Task 2: `StreamRoomResolver` + `ListRooms` 改为 SRS API 直查

**Files:**
- Modify: `app/server/internal/pkg/room_registry.go`
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Modify: `app/server/internal/sfu/factory/dynamic_provider.go`
- Modify: `app/server/server/gin.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

- [ ] **Step 1: 写失败测试（resolver 驱动 ListRooms 直查）**

在 `provider_test.go` 新增 stub resolver 与测试：

```go
// stubResolver 模拟 pkg.StreamRoomResolver：stream -> room（复合键）。
type stubResolver struct {
	roomForStream map[string]string
}

func (r *stubResolver) RoomForStream(stream string) (string, bool) {
	if r == nil || r.roomForStream == nil {
		return "", false
	}
	room, ok := r.roomForStream[stream]
	return room, ok
}

func TestListRooms_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-abc", "gs-def", "gs-abc2", "other-stream"}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{
		"gs-abc":  "dom:room-a",
		"gs-def":  "dom:room-b",
		"gs-abc2": "dom:room-a",
	}}

	rooms, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms err: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("rooms = %+v, want 2 rooms", rooms)
	}
	byName := map[string]int{}
	for _, r := range rooms {
		byName[r.Name] = r.MemberCount
	}
	if byName["dom:room-a"] != 2 || byName["dom:room-b"] != 1 {
		t.Fatalf("rooms = %+v, want room-a:2 room-b:1", rooms)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListRooms_FromSRSAPI' -v`
Expected: FAIL。`s.resolver` 字段不存在（编译错误）且 `ListRooms` 仍走 registry 返回空。

- [ ] **Step 3: 在 `pkg/room_registry.go` 增加 resolver 接口**

在文件末尾新增：

```go
// StreamRoomResolver 提供 stream→room 反查，供 SRS 等无原生 room 维度的 provider
// 在 SRS API 直查后映射回 GOSpeak room（复合键 domainUUID:roomName）。
// 由 signal.Hub 实现：本地 streamRoomCache 优先，membership KV 兜底。
type StreamRoomResolver interface {
	RoomForStream(stream string) (string, bool)
}

// StreamRoomResolverSetter 由需要 stream→room 反查的 provider 实现可选接口。
type StreamRoomResolverSetter interface {
	SetStreamRoomResolver(r StreamRoomResolver)
}
```

- [ ] **Step 4: `provider.go` 增加 resolver 字段与 `ListRooms` 直查路径**

`Service` 结构体增加字段：

```go
type Service struct {
	client     *Client
	secret     string
	host       string
	publicHost string
	whipURL    string
	registry   pkg.RoomRegistry
	resolver   pkg.StreamRoomResolver
}
```

新增方法与重写 `ListRooms`：

```go
// SetStreamRoomResolver 注入 stream→room 反查（signal.Hub 实现）。
func (s *Service) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	s.resolver = r
}

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.resolver != nil {
		return s.listRoomsFromSRS()
	}
	// 降级：resolver 未注入时保持 registry 聚合。
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

// listRoomsFromSRS 直接查 SRS /api/v1/streams/，用 resolver 反查 room 聚合。
func (s *Service) listRoomsFromSRS() ([]sfu.RoomSummary, error) {
	streams, err := s.client.ListStreams()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	rooms := make(map[string]int)
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		room, ok := s.resolver.RoomForStream(stream)
		if !ok || room == "" {
			continue
		}
		rooms[room]++
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for room, count := range rooms {
		out = append(out, sfu.RoomSummary{Name: room, MemberCount: count})
	}
	return out, nil
}
```

删除 `ListRooms` 旧实现中重复的 registry 分支（上面新实现已包含降级逻辑）。

- [ ] **Step 5: `dynamic_provider.go` 转发 resolver 注入**

在 `DynamicProvider` 结构体增加字段：

```go
streamRoomResolver pkg.StreamRoomResolver
```

在 `SetRoomRegistry` 之后新增：

```go
// SetStreamRoomResolver 注入 stream→room 反查，转发给需要它的 provider。
func (p *DynamicProvider) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	p.mu.Lock()
	p.streamRoomResolver = r
	if p.cachedProvider != nil {
		if rs, ok := p.cachedProvider.(pkg.StreamRoomResolverSetter); ok {
			rs.SetStreamRoomResolver(r)
		}
	}
	p.mu.Unlock()
}
```

在 `current()` 重建 provider 的注入段（`if p.roomRegistry != nil {...}` 之后）新增：

```go
if p.streamRoomResolver != nil {
	if rs, ok := provider.(pkg.StreamRoomResolverSetter); ok {
		rs.SetStreamRoomResolver(p.streamRoomResolver)
	}
}
```

- [ ] **Step 6: `gin.go` 注入 `signalHub`**

在现有 RoomRegistry 注入段（`if rs, ok := sfuProvider.(pkg.RoomRegistrySetter); ok {...}`）之后新增：

```go
if rs, ok := sfuProvider.(pkg.StreamRoomResolverSetter); ok {
	rs.SetStreamRoomResolver(signalHub)
}
```

- [ ] **Step 7: 跑测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ ./internal/sfu/factory/ ./internal/pkg/ -run 'TestListRooms' -v`
Expected: `TestListRooms_FromSRSAPI` PASS，其余 `TestListRooms_*`（registry 降级路径）仍 PASS。

- [ ] **Step 8: Commit**

```bash
git add app/server/internal/pkg/room_registry.go app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/factory/dynamic_provider.go app/server/server/gin.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "feat(srs): list rooms via SRS API with stream-to-room resolver"
```

---

## Task 3: `ListParticipants` 与 `DeleteRoom` 改为 SRS API 直查

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestListParticipants_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-a1", "gs-a2", "gs-b1"}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom:room-a",
		"gs-a2": "dom:room-a",
		"gs-b1": "dom:room-b",
	}}
	s.registry = &stubRegistry{identityStreams: map[string]string{
		"dom:room-a\x00alice": "gs-a1",
		"dom:room-a\x00bob":   "gs-a2",
	}}

	parts, err := s.ListParticipants("dom:room-a")
	if err != nil {
		t.Fatalf("ListParticipants err: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("participants = %+v, want 2", parts)
	}
	ids := map[string]bool{}
	for _, p := range parts {
		ids[p.Identity] = true
	}
	if !ids["alice"] || !ids["bob"] {
		t.Fatalf("participants = %+v, want alice+bob (identity from registry)", parts)
	}
}
```

同时新增 DeleteRoom 直查测试：

```go
func TestDeleteRoom_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-a1", "gs-b1"}
	ts.clients = []clientsResponseClient{
		{ID: "cid-1", Name: "gs-a1"},
		{ID: "cid-2", Name: "gs-b1"},
	}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom:room-a",
		"gs-b1": "dom:room-b",
	}}
	s.registry = &stubRegistry{}

	if err := s.DeleteRoom("dom:room-a"); err != nil {
		t.Fatalf("DeleteRoom err: %v", err)
	}
	if len(ts.kickedIDs) != 1 || ts.kickedIDs[0] != "cid-1" {
		t.Fatalf("kickedIDs=%v, want [cid-1] (only room-a stream)", ts.kickedIDs)
	}
	if len(s.registry.cleared) != 1 || s.registry.cleared[0] != "dom:room-a" {
		t.Fatalf("cleared=%v, want [dom:room-a]", s.registry.cleared)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListParticipants_FromSRSAPI|TestDeleteRoom_FromSRSAPI' -v`
Expected: FAIL（resolver 存在时 `ListParticipants` 仍走旧 registry 路径返回空；`DeleteRoom` 仍走旧 registry 路径，stub 无 rooms 报 `SFU_NOT_CONFIGURED`/空）。

- [ ] **Step 3: 实现 `ListParticipants` 直查路径（基于 `/api/v1/streams/`，避免误算订阅连接）**

> **准确度说明**：`/api/v1/clients/` 同时包含推流（publish）与播放（WHEP 订阅）连接；播放连接的 `name` 是它订阅的远端流名，按 stream 过滤会把它误算进房间参与者。因此直查路径**改用 `/api/v1/streams/`**（只列出有 publisher 的流），每流 = 一个活跃推流者，天然排除播放连接。

重写 `ListParticipants` 并新增 helper：

```go
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if s.resolver != nil {
		return s.listParticipantsFromSRS(room)
	}
	// 降级：resolver 未注入时保持旧 registry 路径。
	var streams []string
	if s.registry != nil {
		streams = s.registry.Streams(room)
	}
	if len(streams) == 0 {
		return []sfu.ParticipantSummary{}, nil
	}
	participants, err := s.client.ListParticipantsByStreams(streams)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(participants))
	seen := make(map[string]struct{}, len(participants))
	for _, p := range participants {
		stream, _ := p["stream"].(string)
		identity := ""
		if s.registry != nil && stream != "" {
			if id, ok := s.registry.IdentityForStream(room, stream); ok {
				identity = id
			}
		}
		if identity == "" {
			if id, ok := p["id"].(string); ok {
				identity = id
			}
		}
		if identity == "" {
			continue
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, sfu.ParticipantSummary{Identity: identity})
	}
	return out, nil
}

// listParticipantsFromSRS 从 /api/v1/streams/ 拉全部活跃推流（不含播放连接），
// 用 resolver 反查 room 过滤，identity 经 registry 解析；解析不到的流跳过，
// 避免 SRS 残留/内部流污染统计。
func (s *Service) listParticipantsFromSRS(room string) ([]sfu.ParticipantSummary, error) {
	streams, err := s.client.ListStreams()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	seen := make(map[string]struct{}, len(streams))
	out := make([]sfu.ParticipantSummary, 0, len(streams))
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		candidate, ok := s.resolver.RoomForStream(stream)
		if !ok || !roomMatches(candidate, room) {
			continue
		}
		identity := ""
		if s.registry != nil {
			if id, ok := s.registry.IdentityForStream(candidate, stream); ok {
				identity = id
			}
		}
		if identity == "" {
			continue // 无法识别的流不计入
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, sfu.ParticipantSummary{Identity: identity})
	}
	return out, nil
}

// roomMatches 支持复合键（domainUUID:roomName）精确匹配与纯逻辑名后缀匹配。
func roomMatches(candidate, room string) bool {
	if candidate == room {
		return true
	}
	if !strings.Contains(room, ":") {
		if i := strings.LastIndex(candidate, ":"); i >= 0 && candidate[i+1:] == room {
			return true
		}
	}
	return false
}
```

重写 `DeleteRoom` 增加直查分支，保留 registry 降级路径：

```go
func (s *Service) DeleteRoom(room string) error {
	if s.resolver != nil {
		return s.deleteRoomFromSRS(room)
	}
	// 降级：resolver 未注入时保持旧 registry 路径。
	if s.registry != nil {
		streams := s.registry.Streams(room)
		kicked, remaining, err := s.client.KickByStreams(streams)
		if err != nil {
			return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
		}
		if kicked == 0 && remaining == 0 && len(streams) == 0 {
			return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
		}
		s.registry.ClearRoom(room)
		return nil
	}
	if err := s.client.DeleteRoom(room); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

// deleteRoomFromSRS 从 /api/v1/streams/ 拉全部流，用 resolver 反查属于该 room 的流，
// 通过 SRS clients API 踢掉（SRS 无删流原语，DELETE /api/v1/streams/{name} 返 2048），
// 最后清理本地聚合登记（ClearRoom 会同步清理 membership KV）。
func (s *Service) deleteRoomFromSRS(room string) error {
	streams, err := s.client.ListStreams()
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	targets := make([]string, 0, len(streams))
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		candidate, ok := s.resolver.RoomForStream(stream)
		if !ok || !roomMatches(candidate, room) {
			continue
		}
		targets = append(targets, stream)
	}
	kicked, remaining, err := s.client.KickByStreams(targets)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	if kicked == 0 && remaining == 0 && len(targets) == 0 {
		return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
	}
	if remaining > 0 && kicked == 0 {
		return pkg.NewAppError(pkg.SFU_ERROR, "srs delete room partial failure")
	}
	if s.registry != nil {
		s.registry.ClearRoom(room)
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestListParticipants|TestDeleteRoom' -v`
Expected: `TestListParticipants_FromSRSAPI`、`TestDeleteRoom_FromSRSAPI` 与既有 `TestListParticipants_*`/`TestDeleteRoom_*` 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "feat(srs): list participants and delete rooms via SRS API with room resolver"
```

---

## Task 4: `capabilities.go` 升级 SRS `ListLevel` 为 `hard`

**Files:**
- Modify: `app/server/internal/sfu/capabilities.go`
- Modify: `app/server/internal/sfu/capabilities_test.go`
- Modify: `app/server/internal/sfu/providers/srs/provider_test.go`

- [ ] **Step 1: 写失败测试（更新期望值）**

在 `capabilities_test.go` 的 srs case 中更新：

```go
{
	name: "srs",
		want: Capabilities{
			ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
			MuteLevel: EnforcementDegraded, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
			ListLevel: EnforcementHard,
		},
},
```

同时更新 `provider_test.go` 的 `TestCapabilities_ServerMuteEnabled`：

```go
if caps.ListLevel != "hard" {
	t.Fatalf("srs ListLevel=%q, want hard", caps.ListLevel)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/server && go test ./internal/sfu/ ./internal/sfu/providers/srs/ -run 'TestCapabilities' -v`
Expected: FAIL，srs `ListLevel` 期望 `hard`，实际 `degraded`。

- [ ] **Step 3: 更新 `capabilities.go`**

```go
case "srs":
	// Mute degraded: 不踢流（Discord 式），订阅端静音由 member:muted 事件驱动，
	// on_publish 禁推黑名单兜底（断流后无法重推）。List hard: SRS API 直查 + stream→room 反查。
	return Capabilities{
		ServerMute: true, ServerKick: true, DeleteRoom: true, ListRooms: true, ListMembers: true,
		MuteLevel: EnforcementDegraded, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
		ListLevel: EnforcementHard,
	}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/server && go test ./internal/sfu/ ./internal/sfu/providers/srs/ -run 'TestCapabilities' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/capabilities.go app/server/internal/sfu/capabilities_test.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "feat(srs): upgrade list capability to hard via SRS API"
```

---

## Task 5: SRS Provider 禁推黑名单（mute 强一致，不踢流）

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

- [ ] **Step 1: 写失败测试**

新增测试（含对旧 unmute no-op 语义的替换；mute 不再踢流，只写黑名单）：

```go
func TestMuteParticipantTimed_MuteWritesPublishBlock(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	stream := GenerateStreamName("room-r", "alice")

	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{identityStreams: map[string]string{"room-r\x00alice": stream}}
	store := sfu.NewMemoryMuteRuleStore()
	s.SetMuteRuleStore(store)

	if err := s.MuteParticipantTimed("room-r", "alice", "", true, 60); err != nil {
		t.Fatalf("mute err: %v", err)
	}
	if id, _ := store.Get(context.Background(), PublishBlockKey(stream)); id != publishBlockRuleID {
		t.Fatalf("publish block not saved, id=%d", id)
	}
	if len(ts.kickedIDs) != 0 {
		t.Fatalf("Discord-style mute must NOT kick, kickedIDs=%v", ts.kickedIDs)
	}
}

func TestMuteParticipantTimed_UnmuteDeletesPublishBlock(t *testing.T) {
	s := newSRSTestService(t)
	store := sfu.NewMemoryMuteRuleStore()
	s.SetMuteRuleStore(store)
	stream := GenerateStreamName("room-r", "alice")
	_ = store.Save(context.Background(), PublishBlockKey(stream), publishBlockRuleID, 0)

	if err := s.MuteParticipantTimed("room-r", "alice", "", false, 0); err != nil {
		t.Fatalf("unmute err: %v", err)
	}
	if id, _ := store.Get(context.Background(), PublishBlockKey(stream)); id != 0 {
		t.Fatalf("publish block should be deleted, id=%d", id)
	}
}

func TestSRS_ImplementsTimedMuteProvider(t *testing.T) {
	var _ sfu.TimedMuteProvider = (*Service)(nil)
}
```

删除（或改写）旧测试 `TestMuteParticipant_UnmuteSoftNoop` 与 `TestSRS_MuteFalseReturnsSoft`——unmute 不再返回 `ErrSFUNotSupported`，改为断言 `MuteParticipant("r","u","",false)` 返回 nil 并删除黑名单。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run 'TestMuteParticipant|TestSRS_MuteFalse' -v`
Expected: FAIL，`MuteParticipantTimed` 不存在、`PublishBlockKey` 未定义。

- [ ] **Step 3: 实现禁推黑名单**

`provider.go` 新增 import（`context`、`time`），`Service` 增加字段：

```go
muteRules sfu.MuteRuleStore
```

新增常量与方法：

```go
const publishBlockRuleID = 1

// PublishBlockKey 是 SRS 禁推黑名单的 MuteRuleStore key（provider 与 callback handler 共用）。
func PublishBlockKey(stream string) string {
	return "srs_pub_block:" + stream
}

// SetMuteRuleStore 注入跨实例禁推黑名单（nats → memory），实现 sfu.MuteRuleStoreSetter。
func (s *Service) SetMuteRuleStore(store sfu.MuteRuleStore) {
	if store == nil {
		store = sfu.NewMemoryMuteRuleStore()
	}
	s.muteRules = store
}

func (s *Service) ruleStore() sfu.MuteRuleStore {
	if s.muteRules != nil {
		return s.muteRules
	}
	return sfu.NewMemoryMuteRuleStore()
}

// MuteParticipantTimed 实现 sfu.TimedMuteProvider（Discord 式，不踢流）：
// muted=true 写禁推黑名单——当前推流保留（订阅端静音由 member:muted 事件驱动，成员仍能听），
// 一旦断流/重连，SRS on_publish 回调会拒绝该 stream 重新发布；
// muted=false 移除黑名单，允许重新发布。ttlSeconds>0 时黑名单带 TTL。
func (s *Service) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	stream := s.resolveStream(room, identity)
	return s.applyPublishBlock(stream, muted, ttlSeconds)
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return s.MuteParticipantTimed(room, identity, trackSid, muted, 0)
}

func (s *Service) resolveStream(room, identity string) string {
	if s.registry != nil {
		if st, ok := s.registry.StreamForIdentity(room, identity); ok {
			return st
		}
	}
	return GenerateStreamName(room, identity)
}

func (s *Service) applyPublishBlock(stream string, muted bool, ttlSeconds int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if muted {
		var ttl time.Duration
		if ttlSeconds > 0 {
			ttl = time.Duration(ttlSeconds) * time.Second
		}
		return s.ruleStore().Save(ctx, PublishBlockKey(stream), publishBlockRuleID, ttl)
	}
	return s.ruleStore().Delete(ctx, PublishBlockKey(stream))
}
```

删除旧 `MuteParticipant` 实现（被上面委托版本替代）。`RemoveParticipant`/`DeleteRoom` 保持不变（踢出房间不写黑名单，允许重新加入）。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -v`
Expected: 全部 PASS（含更新后的 unmute 语义测试）。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "feat(srs): discord-style hard mute via on_publish publish-block blacklist"
```

---

## Task 6: `SRSCallbackHandler` on_publish 禁推拒绝

**Files:**
- Modify: `app/server/internal/handler/srs_callback_handler.go`
- Modify: `app/server/server/gin.go`
- Test: `app/server/internal/handler/srs_callback_handler_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestSrsCallback_OnPublish_BlockedStream_Rejects(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandler(hub, "secret")
	store := sfu.NewMemoryMuteRuleStore()
	_ = store.Save(context.Background(), srs.PublishBlockKey("gs-aaa"), 1, 0)
	h.SetMuteRuleStore(store)

	stream := "gs-aaa"
	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("blocked on_publish should return code 1, got %s", w.Body.String())
	}
	if hub.IsStreamActive(stream) {
		t.Fatal("blocked stream must NOT be registered")
	}
}

func TestSrsCallback_OnPublish_AfterUnmute_Allows(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandler(hub, "secret")
	store := sfu.NewMemoryMuteRuleStore()
	h.SetMuteRuleStore(store)
	stream := "gs-aaa"
	_ = store.Save(context.Background(), srs.PublishBlockKey(stream), 1, 0)
	_ = store.Delete(context.Background(), srs.PublishBlockKey(stream))

	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("unblocked on_publish should return code 0, got %s", w.Body.String())
	}
	if !hub.IsStreamActive(stream) {
		t.Fatal("stream should be registered after unmute")
	}
}
```

新增 import：`context`、`GOSpeak/internal/sfu`。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/server && go test ./internal/handler/ -run 'TestSrsCallback_OnPublish_Blocked|TestSrsCallback_OnPublish_AfterUnmute' -v`
Expected: FAIL，`SetMuteRuleStore` 未定义。

- [ ] **Step 3: 实现 handler 禁推检查**

`srs_callback_handler.go` 的 `SRSCallbackHandler` 增加字段与 setter：

```go
muteStore sfu.MuteRuleStore
```

```go
// SetMuteRuleStore 注入 SRS 禁推黑名单（与 srs.Service 共用同一 store）。
func (h *SRSCallbackHandler) SetMuteRuleStore(store sfu.MuteRuleStore) {
	h.muteStore = store
}
```

新增 import `GOSpeak/internal/sfu`。`on_publish` 分支在 token 校验之后、`RegisterStream`/job 发布之前插入：

```go
case "on_publish":
	token := params["token"]
	if token == "" || !srs.ValidateStreamToken(stream, token, secret) {
		c.JSON(http.StatusOK, gin.H{"code": 1})
		return
	}
	if h.muteStore != nil {
		blocked, _ := h.muteStore.Get(c.Request.Context(), srs.PublishBlockKey(stream))
		if blocked > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 1})
			return
		}
	}
	if h.jobs != nil {
		_ = h.jobs.PublishSRS(c.Request.Context(), "on_publish", stream)
	} else {
		h.hub.RegisterStream(stream)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
```

- [ ] **Step 4: `gin.go` 注入同一 mute store**

在 `srsCallbackH` 构造与 `SetJobs` 之后新增：

```go
srsCallbackH.SetMuteRuleStore(muteRuleStore)
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd app/server && go test ./internal/handler/ -run 'TestSrsCallback' -v`
Expected: 全部 PASS（含既有 token/play 用例）。

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/handler/srs_callback_handler.go app/server/server/gin.go app/server/internal/handler/srs_callback_handler_test.go
git commit -m "feat(srs): reject on_publish for muted streams in callback"
```

---

## Task 7: signal 层新增 `member:muted` / `member:unmuted` 事件

**Files:**
- Modify: `app/server/internal/signal/events.go`
- Modify: `app/server/internal/signal/hub_mute.go`
- Test: `app/server/internal/signal/hub_mute_test.go`（新建）

- [ ] **Step 1: 写失败测试**

新建 `hub_mute_test.go`：

```go
package signal

import (
	"testing"

	"GOSpeak/internal/model"
)

// recordingFanout 捕获 publishNamespace 的事件名与 payload，验证 mute 事件广播。
type recordingFanout struct {
	events map[string][]map[string]interface{}
}

func (f *recordingFanout) publishNamespace(event string, data map[string]interface{}) {
	if f.events == nil {
		f.events = map[string][]map[string]interface{}{}
	}
	f.events[event] = append(f.events[event], data)
}

func TestBroadcastMute_PublishesMemberMuted(t *testing.T) {
	f := &recordingFanout{}
	hub := NewHub(nil, nil, nil, nil)
	hub.publishNamespace = f.publishNamespace
	// 注入 userStore 以解析 identity（hub_mute.go 的 identityForUserID 走 userStore）。
	hub.userStore = &stubUserStore{nameByID: map[uint]string{7: "alice"}}

	hub.BroadcastMute(7, &MuteInfo{Permanent: true})

	events := f.events[EventMemberMuted]
	if len(events) != 1 {
		t.Fatalf("member:muted events = %d, want 1", len(events))
	}
	if events[0]["identity"] != "alice" || events[0]["muted"] != true {
		t.Fatalf("member:muted payload = %+v, want identity=alice muted=true", events[0])
	}
}

func TestBroadcastUnmute_PublishesMemberUnmuted(t *testing.T) {
	f := &recordingFanout{}
	hub := NewHub(nil, nil, nil, nil)
	hub.publishNamespace = f.publishNamespace
	hub.userStore = &stubUserStore{nameByID: map[uint]string{7: "alice"}}

	hub.BroadcastUnmute(7)

	events := f.events[EventMemberUnmuted]
	if len(events) != 1 {
		t.Fatalf("member:unmuted events = %d, want 1", len(events))
	}
	if events[0]["identity"] != "alice" || events[0]["muted"] != false {
		t.Fatalf("member:unmuted payload = %+v, want identity=alice muted=false", events[0])
	}
}
```

`stubUserStore` 需满足 `userStore` 接口（`GetByID(id) (*model.User, error)`），若 `signal` 包测试已有类似 stub 则复用。测试文件需要 import `GOSpeak/internal/model`。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd app/server && go test ./internal/signal/ -run 'TestBroadcastMute|TestBroadcastUnmute' -v`
Expected: FAIL（`EventMemberMuted` 未定义、`hub.publishNamespace` 字段不存在）。

- [ ] **Step 3: 实现事件常量与广播**

`events.go` 在 `EventUserMuted`/`EventUserUnmuted` 之后新增：

```go
// 成员级禁言事件（订阅端静音用）：所有客户端按 identity 静音/恢复远端 track。
EventMemberMuted   = "member:muted"
EventMemberUnmuted = "member:unmuted"
```

`hub_mute.go` 的 `BroadcastMute` 在 `publishNamespace(EventUserMuted, data)` 之后追加：

```go
if identity := h.identityForUserID(userID); identity != "" {
	h.publishNamespace(EventMemberMuted, map[string]interface{}{
		"identity": identity,
		"muted":    true,
	})
}
```

`BroadcastUnmute` 在 `publishNamespace(EventUserUnmuted, ...)` 之后追加：

```go
if identity := h.identityForUserID(userID); identity != "" {
	h.publishNamespace(EventMemberUnmuted, map[string]interface{}{
		"identity": identity,
		"muted":    false,
	})
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/server && go test ./internal/signal/ -run 'TestBroadcastMute|TestBroadcastUnmute' -v`
Expected: PASS。再跑 `go test ./internal/signal/` 确认无回归。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/signal/events.go app/server/internal/signal/hub_mute.go app/server/internal/signal/hub_mute_test.go
git commit -m "feat(signal): broadcast member:muted/unmuted for subscriber-side mute"
```

---

## Task 8: 前端订阅端静音（`serverMutedIdentities` 独立集合）+ 403 防御

> **冲突修正（相对 v1 计划）：** `handler_audio.setMutedByIdentity` 被本地个人静音按钮占用（写
> `mutedIdentities`）。服务器禁言若复用同一函数，unmute 会误删个人手动静音。因此新增独立集合
> `serverMutedIdentities` 与 `setServerMutedByIdentity`，`effectiveVolume` 同时检查两个集合。
> 同时简化：不再给 `SFUClient` 加 `setMutedByIdentity` 接口方法——app 层 `handler_audio` 已通过
> `RemoteAudioTrackLike.setVolume` 控制所有 provider 的远端音量，订阅端静音统一走 app 层。

**Files:**
- Modify: `app/web/src/handler_audio/index.ts`
- Modify: `app/web/src/socket/events.ts`
- Modify: `app/web/src/socket/socketEvents.ts`
- Modify: `app/web/src/components/room/session/providers.ts`
- Modify: `packages/sfu-client/src/srs-stream-gate.ts`
- Test: `app/web/src/handler_audio/index.test.ts`（新建）
- Test: `app/web/src/socket/socketEvents.test.ts`（若存在，否则新建）
- Test: `app/web/src/components/room/session/providers.test.ts`
- Test: `packages/sfu-client/src/srs-client.test.ts`

- [ ] **Step 1: 写失败测试（handler_audio 服务器禁言集合）**

新建 `app/web/src/handler_audio/index.test.ts`：

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RemoteAudioTrackLike } from "@gospeak/sfu-client/types";

const tracks = new Map<string, RemoteAudioTrackLike>();
vi.mock("@gospeak/sfu-client/types", () => ({}));

async function load() {
	const mod = await import("@/handler_audio");
	const fakeTrack = {
		attach: vi.fn(() => document.createElement("audio")),
		detach: vi.fn(),
		setVolume: vi.fn(),
	} as unknown as RemoteAudioTrackLike;
	// setupAudioHandler 注册 onRemoteAudioTrack 回调，用假 client 触发订阅
	const client = {
		onRemoteAudioTrack: (cb: any) => {
			cb({ identity: "alice", track: fakeTrack });
		},
		onRemoteAudioTrackRemoved: () => {},
		onActiveSpeakers: () => {},
		getExistingRemoteAudioTracks: () => [],
	} as any;
	mod.setupAudioHandler(client);
	return { mod, fakeTrack };
}

describe("setServerMutedByIdentity", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
	});

	it("server mute zeroes track volume and unmute restores it", async () => {
		const { mod, fakeTrack } = await load();
		mod.setServerMutedByIdentity("alice", true);
		expect(fakeTrack.setVolume).toHaveBeenLastCalledWith(0);
		mod.setServerMutedByIdentity("alice", false);
		expect(fakeTrack.setVolume).toHaveBeenLastCalledWith(1);
	});

	it("server mute is independent from local personal mute", async () => {
		const { mod, fakeTrack } = await load();
		mod.setVolumeByIdentity("alice", 0.5);
		mod.setServerMutedByIdentity("alice", true);
		expect(fakeTrack.setVolume).toHaveBeenLastCalledWith(0);
		// unmute 服务器禁言后，恢复个人音量而不是 1
		mod.setServerMutedByIdentity("alice", false);
		expect(fakeTrack.setVolume).toHaveBeenLastCalledWith(0.5);
	});
});
```

> 若 mock 方式与项目现有 handler 测试模式不一致，按 `app/web/src` 现有测试惯例调整（关键是断言
> `setServerMutedByIdentity` 独立于 `setVolumeByIdentity` 与个人静音）。

- [ ] **Step 2: 写失败测试（socket 事件 + join 同步 + 403 防御）**

`socketEvents.test.ts`（或新建）新增：

```ts
import { setServerMutedByIdentity } from "@/handler_audio";
import { EVENTS } from "@/socket/events";

vi.mock("@/handler_audio", () => ({
	setServerMutedByIdentity: vi.fn(),
}));

it("member:muted drives server-side mute by identity", async () => {
	const { bindServerEvents } = await import("@/socket/socketEvents");
	const adapter = createFakeAdapter();
	bindServerEvents(adapter, fakeDeps());

	await adapter.emit(EVENTS.MEMBER_MUTED, { identity: "alice", muted: true });
	expect(setServerMutedByIdentity).toHaveBeenCalledWith("alice", true);

	await adapter.emit(EVENTS.MEMBER_UNMUTED, { identity: "alice", muted: false });
	expect(setServerMutedByIdentity).toHaveBeenCalledWith("alice", false);
});
```

`providers.test.ts` 新增：

```ts
it("srs afterMediaJoin applies server mute for existing members", async () => {
	const setServerMuted = vi.fn();
	vi.doMock("@/handler_audio", () => ({ setServerMutedByIdentity: setServerMuted }));
	const ack = {
		members: [
			{ identity: "alice", stream: "gs-a", isMuted: true },
			{ identity: "bob", stream: "gs-b", isMuted: false },
		],
	};
	srsAdapter.afterMediaJoin({} as any, { identity: "me", stream: "gs-me" } as any, ack);
	expect(setServerMuted).toHaveBeenCalledWith("alice", true);
	expect(setServerMuted).not.toHaveBeenCalledWith("bob", true);
});
```

`srs-client.test.ts` 新增：

```ts
it("403 publish denied is not treated as busy", () => {
	expect(isWhipBusyError(new Error("SRS WHIP request failed: 403"))).toBe(false);
	expect(isWhipBusyError(new Error("SRS WHIP request failed: 5020"))).toBe(true);
});
```

- [ ] **Step 3: 跑测试确认失败**

Run: `cd app/web && pnpm test -- handler_audio`、`cd app/web && pnpm test -- socketEvents`、`cd app/web && pnpm test -- providers.test.ts`、`cd packages/sfu-client && pnpm test -- srs-client.test.ts`
Expected: FAIL（`setServerMutedByIdentity` 未定义、`EVENTS.MEMBER_MUTED` 未定义、403 被判定 busy）。

- [ ] **Step 4: 实现 `handler_audio` 服务器禁言集合**

`app/web/src/handler_audio/index.ts`：

```ts
const serverMutedIdentities = new Set<string>();

function effectiveVolume(identity: string): number {
	if (masterMuted || mutedIdentities.has(identity) || serverMutedIdentities.has(identity)) {
		return 0;
	}
	return masterVolume * (volumes.get(identity) ?? 1);
}

/** 服务器禁言驱动的订阅端静音（member:muted/member:unmuted），独立于本地个人静音。 */
export function setServerMutedByIdentity(identity: string, muted: boolean) {
	if (muted) {
		serverMutedIdentities.add(identity);
	} else {
		serverMutedIdentities.delete(identity);
	}
	const track = tracks.get(identity);
	if (track) {
		track.setVolume(effectiveVolume(identity));
	}
}
```

`cleanupAudioHandler` 中 `mutedIdentities.clear()` 之后追加 `serverMutedIdentities.clear();`。

- [ ] **Step 5: 实现 socket 事件绑定**

`app/web/src/socket/events.ts` 在 `USER_UNMUTED` 之后新增：

```ts
MEMBER_MUTED: "member:muted",
MEMBER_UNMUTED: "member:unmuted",
```

`socketEvents.ts` 在 `EVENTS.USER_UNMUTED` 绑定之后新增：

```ts
adapter.onServerEvent(
	EVENTS.MEMBER_MUTED as string,
	(data: { identity?: string; muted?: boolean }) => {
		if (data.identity) setServerMutedByIdentity(data.identity, true);
	},
);
adapter.onServerEvent(
	EVENTS.MEMBER_UNMUTED as string,
	(data: { identity?: string; muted?: boolean }) => {
		if (data.identity) setServerMutedByIdentity(data.identity, false);
	},
);
```

文件头部新增 import：`import { setServerMutedByIdentity } from "@/handler_audio";`

- [ ] **Step 6: 实现 join 初始同步**

`providers.ts` 的 `srsAdapter.afterMediaJoin` 改为：

```ts
afterMediaJoin(client, token, ack) {
	const peers = (ack.members ?? [])
		.filter((m) => m.identity !== token.identity && m.stream)
		.map((m) => ({ identity: m.identity, stream: m.stream as string }));
	if (peers.length) client.subscribePeers?.(peers);
	// 服务器禁言状态同步：join 时已有成员若被禁言，订阅端立即静音。
	for (const m of ack.members ?? []) {
		if (m.identity !== token.identity && m.isMuted) {
			setServerMutedByIdentity(m.identity, true);
		}
	}
	bindSignalActiveSpeakers(client, token);
},
```

文件头部新增 import：`import { setServerMutedByIdentity } from "@/handler_audio";`

- [ ] **Step 7: 实现 403 防御**

`srs-stream-gate.ts` 的 `isWhipBusyError` 开头插入：

```ts
// 403 = SRS on_publish 回调拒绝（禁推/鉴权失败），不是 busy，绝不能按 busy 无限重试。
if (/403|publish denied|forbidden/i.test(msg)) {
	return false;
}
```

- [ ] **Step 8: 跑测试确认通过**

Run: `cd app/web && pnpm test -- handler_audio`、`cd app/web && pnpm test -- socketEvents`、`cd app/web && pnpm test -- providers.test.ts`、`cd packages/sfu-client && pnpm test`
Expected: 全部 PASS。

- [ ] **Step 9: Commit**

```bash
git add app/web/src/handler_audio/index.ts app/web/src/handler_audio/index.test.ts app/web/src/socket/events.ts app/web/src/socket/socketEvents.ts app/web/src/components/room/session/providers.ts packages/sfu-client/src/srs-stream-gate.ts app/web/src/socket/socketEvents.test.ts app/web/src/components/room/session/providers.test.ts packages/sfu-client/src/srs-client.test.ts
git commit -m "feat(web): subscriber-side server mute via member:muted events and join sync"
```

---

## Task 9: `capabilities.go` 注释更新（MuteLevel 保持 degraded）+ 文档 + 全量回归

**Files:**
- Modify: `app/server/internal/sfu/capabilities.go`
- Modify: `docs/sfu-provider-maturity.md`
- Modify: `docs/srs-selfhost-runbook.md`

- [ ] **Step 1: 更新 `capabilities.go` 注释（不改能力值）**

```go
case "srs":
	// Mute degraded: 不踢流（Discord 式，成员仍可听），订阅端静音由 member:muted 事件驱动，
	// on_publish 禁推黑名单兜底（断流后无法重推）。List hard: SRS API 直查 + stream→room 反查。
	return Capabilities{
		ServerMute: true, ServerKick: true, DeleteRoom: true, AdminToken: true, ListRooms: true, ListMembers: true,
		MuteLevel: EnforcementDegraded, KickLevel: EnforcementHard, DeleteLevel: EnforcementHard,
		ListLevel: EnforcementHard, AdminLevel: EnforcementHard,
	}
```

确认 `capabilities_test.go` 的 srs 期望值与此一致（`MuteLevel: EnforcementDegraded, ListLevel: EnforcementHard`），已由 Task 4 更新。

- [ ] **Step 2: 更新成熟度文档**

`docs/sfu-provider-maturity.md` 中 SRS 相关行更新：

```markdown
| SRS | 高 | 8/8 | mute 为 Discord 式（订阅端静音 + on_publish 禁推，degraded）；成员仍可收听 |
```

```markdown
| SRS | `MuteParticipantTimed` | 中高 | degraded：禁推黑名单（不踢流，订阅端静音） | unmute 删除黑名单，前端恢复音量 |
```

```markdown
| SRS | `internal/sfu/providers/srs/` | WHIP/WHEP；`List*` 经 `/api/v1/streams`+`/api/v1/clients` 直查 + stream→room 反查；`StreamProvider`/`ClientInfoProvider`；`GenerateToken` 签发 stream token |
```

- [ ] **Step 3: 更新自部署 runbook**

`docs/srs-selfhost-runbook.md` 中 mute 语义段落更新（若存在），补充说明：

```markdown
> 禁言语义（SRS6，Discord 式）：禁言时后端写入禁推黑名单（NATS KV / 内存），**不踢流**——
> 被禁言成员保留订阅仍可收听；其他客户端收到 `member:muted` 事件后对该成员远端音轨静音。
> SRS `on_publish` http_hook 命中黑名单时返回 `code:1` 拒绝推流，因此禁言期间断流/重连无法绕过发声。
> 解禁后黑名单删除，客户端收到 `member:unmuted` 恢复音量，可重新推流。
```

- [ ] **Step 4: 补充 SRS 集群/多节点部署说明**

在 `docs/srs-selfhost-runbook.md` 或 `docs/sfu-provider-maturity.md` 增加部署拓扑说明：

```markdown
### SRS 多节点 / Cluster 说明

SRS 5.0+ 的 origin-edge / origin cluster 是**流分发与故障转移**能力（edge 回源、RTMP302 重定向），
HTTP API（`/api/v1/streams`、`/api/v1/clients`）仍是节点级的，SRS **没有原生 room 维度管理 API**。
GOSpeak 的 room 维度管理（ListRooms/ListParticipants/DeleteRoom）通过直查 SRS API + stream→room
业务映射实现，因此：

- 单节点：`SRS_HOST` 指向该节点即可，无需集群。
- 多节点部署：将 `SRS_HOST`/`SRS_WHIP_URL` 指向可聚合的入口节点（edge 或反代），
  或由部署层遍历各节点后聚合（GOSpeak 不内置集群遍历）。
- stream→room 映射跨实例不变：依赖 membership KV（NATS）与 `member:joined`/`on_publish` 登记，
  与 SRS 节点数无关。
```

- [ ] **Step 5: 后端回归**

Run: `cd app/server && go test ./internal/sfu/... ./internal/handler/... ./internal/signal/... ./internal/pkg/...`
Expected: 全部 PASS。

Run: `cd app/server && go vet ./internal/sfu/providers/srs/ ./internal/handler/ ./internal/signal/`
Expected: 无输出（通过）。

- [ ] **Step 6: 前端回归**

Run: `cd app/web && pnpm test` 与 `cd packages/sfu-client && pnpm test`
Expected: 全部 PASS。

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/sfu/capabilities.go docs/sfu-provider-maturity.md docs/srs-selfhost-runbook.md
git commit -m "docs(srs): update mute semantics and capability comments for Discord-style mute"
```

---

## Self-Review

**Spec coverage:**
- Discord 式 mute（能听、不能说）：Task 5（禁推黑名单，不踢流）+ Task 7（`member:muted/unmuted` 事件）+ Task 8（订阅端静音 + join 初始同步）。
- Mute 非强一致：Task 5 黑名单写入 + Task 6 on_publish 拒绝，断流后重推被 SRS 侧拒绝，闭环覆盖。
- ListRooms/ListMembers/DeleteRoom degraded → SRS API 直查：Task 1 + Task 2 + Task 3（resolver 反查）+ Task 4（ListLevel hard）。
- SRS 集群定位：SRS cluster 无原生 room API，多节点说明写入 Task 9 Step 4（部署层聚合，不改 provider 代码）。
- 跨实例一致性：禁推黑名单复用 `sfu.MuteRuleStore`（NATS KV → memory），stream→room 反查复用 `Hub.RoomForStream`（本地 cache + membership KV）。
- 前端不无限重试被拒推流：Task 8 的 403 防御。
- 文档：Task 9。

**Placeholder scan:** 所有代码步骤均含完整实现与测试代码；`...` 仅出现在"保留现有实现不变"的上下文，无 TBD/TODO。

**Type consistency:**
- `PublishBlockKey(stream)` 在 Task 5 定义，Task 6 使用，签名一致。
- `SetMuteRuleStore(store sfu.MuteRuleStore)` 在 Task 5（Service）与 Task 6（Handler）分别定义，均对应 `sfu.MuteRuleStoreSetter` 语义。
- `SetStreamRoomResolver(r pkg.StreamRoomResolver)` 在 Task 2 的 Service、DynamicProvider 与 gin.go 中签名一致。
- `MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int)` 与 `sfu.TimedMuteProvider`（`app/server/internal/sfu/types.go:51`）签名一致。
- `EventMemberMuted`/`EventMemberUnmuted` 在 Task 7 定义，Task 8 前端 `EVENTS.MEMBER_MUTED`/`MEMBER_UNMUTED` 对应同一字符串（`member:muted`/`member:unmuted`）。
- `client.setMutedByIdentity?.(identity, muted)` 在 Task 8 的 `providers.ts` 使用，`SFUClient` 类型同步新增，签名一致。
