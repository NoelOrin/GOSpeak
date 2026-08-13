# SRS 硬禁言核心修复 Implementation Plan

> **Status (2026-08-13):** 🟠 进行中 (2026-08-13) — 代码已 staged 未提交 (mute_expiry job/roomMatches/NATS TTL)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复提交 74c835a（Discord 式硬禁言 + SRS API 房间管理）的后端核心缺陷：跨租户房间匹配、禁言黑名单静默失效、离线 unmute 黑名单残留、回调 fail-open、跨实例缓存失效与 NATS TTL 失真。

**Architecture:** 全部改动位于 Go 后端 `app/server/internal/`。核心思路：(1) `roomMatches` 从"后缀回退匹配"改为"复合键精确匹配"，消除跨租户枚举/误踢；(2) `srs.Service` 在构造期固定默认 `MuteRuleStore`，消除"每次调用新建临时 store"的静默假成功；(3) `OnRoomJoin` 在禁言检查通过后清理黑名单残留，使离线 unmute 自愈；(4) 回调 `on_publish` 对 store 读取错误 fail-closed；(5) `CachedMuteRuleStore` 提供 `GetFresh` 跳过 L1，`NATSMuteRuleStore` 把 per-key TTL 编码进 value；(6) 新增到期扫描定时任务，让临时禁言到期后广播 `member:unmuted`。

**Tech Stack:** Go 1.2x、Gin、GORM/SQLite、nhooyr/websocket、NATS JetStream KV（embedded）、`golang-jwt/jwt/v5`。

---

## Findings 覆盖表（Review → Task）

| Review finding | Task |
|---|---|
| 🔴 `srs/provider.go:198` roomMatches 跨租户后缀匹配 | T1 |
| 🔴 `srs/provider.go:217` ruleStore() 每次新建临时 store | T2 |
| 🔴 `signal/hub_mute.go:130` 离线 unmute 黑名单永不删除 | T6 |
| 🟡 `srs_callback_handler.go:72` muteStore.Get 错误被吞（fail-open） | T3 |
| 🟡 `srs/provider.go:254` CachedMuteRuleStore L1 30s 跨实例失效 | T4 |
| 🟡 `srs/provider.go:250/224/246` NATS 忽略 per-key TTL | T5 |
| 🟡 `signal/hub_mute.go:38` 到期无 member:unmuted 广播（惰性扫描） | T7 |
| 🔵 `srs/provider.go:210` publishBlockRuleID 魔法数字 | T2（加注释） |

---

### Task 1: roomMatches 改为精确匹配（修复跨租户 🔴）

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`（`roomMatches` 函数）
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：`pkg.RoomKey(domainUUID, roomName)` 对空 domain 返回裸名（`app/server/internal/pkg/room_key.go`），因此 `GET /api/v1/signal/participants?room=<name>`（`domain_uuid` 为空）传入裸名；`roomMatches("domainB:lobby", "lobby")` 的后缀回退分支返回 true，导致跨租户聚合他域参与者、`DeleteRoom` 误踢他域同名房间流。`resolver.RoomForStream` 返回的 candidate 与调用方 room 均按同一 `RoomKey` 规则生成，精确匹配即可覆盖平台房（裸名==裸名）与域房（复合键==复合键）。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/sfu/providers/srs/provider_test.go` 新增：

```go
func TestRoomMatches_ExactKeyOnly(t *testing.T) {
	cases := []struct {
		name      string
		candidate string
		room      string
		want      bool
	}{
		{"same composite key", "dom-a:lobby", "dom-a:lobby", true},
		{"same platform key", "lobby", "lobby", true},
		{"cross-domain same name must NOT match", "dom-b:lobby", "lobby", false},
		{"bare name must NOT match composite", "dom-a:lobby", "lobby", false},
		{"different domain same room", "dom-a:lobby", "dom-b:lobby", false},
		{"empty keys", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := roomMatches(tc.candidate, tc.room); got != tc.want {
				t.Fatalf("roomMatches(%q, %q) = %v, want %v", tc.candidate, tc.room, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestRoomMatches_ExactKeyOnly -v`
Expected: FAIL，`{"cross-domain same name must NOT match"}` 与 `{"bare name must NOT match composite"}` 两例 `roomMatches` 返回 true。

- [ ] **Step 3: 最小实现**

修改 `app/server/internal/sfu/providers/srs/provider.go`：

```go
// roomMatches 判断 stream 反查出的房间键是否属于目标房间。
// 房间键由 pkg.RoomKey 生成：域房为 "domainUUID:roomName" 复合键，平台房为裸名。
// 只做精确匹配：后缀回退会让裸名请求命中任意域的复合键，导致跨租户枚举/误踢。
func roomMatches(candidate, room string) bool {
	return candidate == room
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestRoomMatches_ExactKeyOnly -v`
Expected: PASS。

- [ ] **Step 5: 回归测试**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ ./internal/signal/`
Expected: 全部通过（`deleteRoomFromSRS`/`listParticipantsFromSRS` 现有用例使用复合键，不受影响）。

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "fix(srs): match room keys exactly to prevent cross-domain enumeration"
```

---

### Task 2: ruleStore() 固定默认 store（修复静默假成功 🔴）

**Files:**
- Modify: `app/server/internal/sfu/providers/srs/provider.go`（`NewService`、`ruleStore`、`SetMuteRuleStore`）
- Test: `app/server/internal/sfu/providers/srs/provider_test.go`

背景：`ruleStore()` 在 `muteRules == nil` 时每次 `new` 一个 `MemoryMuteRuleStore`，`Save`/`Delete` 落在一次性实例上，黑名单立即丢失，但 `MuteParticipantTimed` 返回 nil，对上层伪装"degraded 成功"。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/sfu/providers/srs/provider_test.go` 新增：

```go
func TestRuleStore_StableAcrossCalls_WithoutInjection(t *testing.T) {
	svc := NewService(&config.Config{})
	ctx := context.Background()
	if err := svc.MuteParticipantTimed("dom-a:r1", "alice", "", true, 0); err != nil {
		t.Fatal(err)
	}
	stream := GenerateStreamName("dom-a:r1", "alice")
	// 未注入 store 时，mute 写入必须可被同一 service 读取（否则黑名单静默丢失）。
	id, err := svc.ruleStore().Get(ctx, PublishBlockKey(stream))
	if err != nil {
		t.Fatal(err)
	}
	if id != publishBlockRuleID {
		t.Fatalf("rule id = %d, want %d (mute must persist in same instance)", id, publishBlockRuleID)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestRuleStore_StableAcrossCalls_WithoutInjection -v`
Expected: FAIL，`rule id = 0`（每次调用新建 store，写入丢失）。

- [ ] **Step 3: 最小实现**

修改 `app/server/internal/sfu/providers/srs/provider.go`：

```go
// SetMuteRuleStore 注入跨实例禁推黑名单（nats → memory），实现 sfu.MuteRuleStoreSetter。
// nil 输入回退到默认内存 store，保证 provider 内部读写一致。
func (s *Service) SetMuteRuleStore(store sfu.MuteRuleStore) {
	if store == nil {
		store = sfu.NewMemoryMuteRuleStore()
	}
	s.muteRules = store
}

func NewService(cfg *config.Config) *Service {
	// ... 保留现有 host/apiPort/whipURL 归一化 ...
	return &Service{
		client:     NewClient(baseURL),
		secret:     cfg.SRSSecret,
		host:       baseURL,
		publicHost: strings.TrimSpace(cfg.SRSPublicHost),
		whipURL:    whipURL,
		muteRules:  sfu.NewMemoryMuteRuleStore(),
	}
}

// ruleStore 返回稳定的黑名单 store：未注入时也是构造期创建的默认实例，
// 而不是每次调用新建（否则 Save/Delete 落在一次性实例，禁言静默失效）。
func (s *Service) ruleStore() sfu.MuteRuleStore {
	if s.muteRules == nil {
		s.muteRules = sfu.NewMemoryMuteRuleStore()
	}
	return s.muteRules
}
```

同时给 `publishBlockRuleID` 补注释（🔵:210）：

```go
// publishBlockRuleID 是 SRS 禁推黑名单的占位 ruleID：SRS 侧只有"存在/不存在"两种状态，
// 不需要真实 rule 标识；值恒为 1。
const publishBlockRuleID = 1
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ -run TestRuleStore_StableAcrossCalls_WithoutInjection -v`
Expected: PASS。

- [ ] **Step 5: 回归测试**

Run: `cd app/server && go test ./internal/sfu/providers/srs/ ./internal/handler/`
Expected: 全部通过。

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/sfu/providers/srs/provider.go app/server/internal/sfu/providers/srs/provider_test.go
git commit -m "fix(srs): keep stable default mute rule store to prevent silent mute loss"
```

---

### Task 3: on_publish 回调 fail-closed（修复黑名单读取吞错 🟡）

**Files:**
- Modify: `app/server/internal/handler/srs_callback_handler.go`
- Test: `app/server/internal/handler/srs_callback_handler_test.go`

背景：`h.muteStore.Get` 错误被 `_` 丢弃，NATS/KV 抖动时 `blocked=0` 走放行分支，被禁言者可重推绕过硬禁言（fail-open）。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/srs_callback_handler_test.go` 新增：

```go
type failingMuteStore struct{}

func (failingMuteStore) Save(context.Context, string, int, time.Duration) error { return nil }
func (failingMuteStore) Get(context.Context, string) (int, error) {
	return 0, errors.New("kv down")
}
func (failingMuteStore) Delete(context.Context, string) error { return nil }
func (failingMuteStore) Backend() string                      { return "memory" }

func TestSrsCallback_OnPublish_MuteStoreError_Denies(t *testing.T) {
	hub := newCallbackHub()
	h := NewSRSCallbackHandler(hub, "secret")
	h.SetMuteRuleStore(failingMuteStore{})

	stream := "gs-fail"
	tok := srsStreamTokenForTest(stream, "secret")
	payload := map[string]string{
		"action": "on_publish",
		"stream": "live/" + stream,
		"param":  "app=live&stream=" + stream + "&token=" + tok,
	}
	w := postJSON(t, h, payload)
	// 黑名单存储故障必须 fail-closed：拒绝发布，而不是静默放行。
	if !strings.Contains(w.Body.String(), `"code":1`) {
		t.Fatalf("store error on_publish should deny (code 1), got %s", w.Body.String())
	}
	if hub.IsStreamActive(stream) {
		t.Fatal("stream should NOT be registered when mute store check fails")
	}
}
```

新增 import：`errors`、`time`（`failingMuteStore` 需要 `time.Duration`）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/handler/ -run TestSrsCallback_OnPublish_MuteStoreError_Denies -v`
Expected: FAIL，当前返回 `"code":0` 且 stream 被注册。

- [ ] **Step 3: 最小实现**

修改 `app/server/internal/handler/srs_callback_handler.go` 的 `on_publish` 分支：

```go
case "on_publish":
	token := params["token"]
	if token == "" || !srs.ValidateStreamToken(stream, token, secret) {
		c.JSON(http.StatusOK, gin.H{"code": 1})
		return
	}
	if h.muteStore != nil {
		blocked, err := h.muteStore.Get(c.Request.Context(), srs.PublishBlockKey(stream))
		if err != nil {
			// 黑名单读取失败时 fail-closed：拒绝发布并记录日志，
			// 避免存储抖动窗口内被禁言者重新推流绕过硬禁言。
			log.Printf("[srs-callback] publish block check failed, denying stream=%s err=%v", stream, err)
			c.JSON(http.StatusOK, gin.H{"code": 1})
			return
		}
		if blocked > 0 {
			log.Printf("[srs-callback] publish blocked by mute rule stream=%s", stream)
			c.JSON(http.StatusOK, gin.H{"code": 1})
			return
		}
	}
	// ... 其余逻辑不变 ...
```

新增 import：`log`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/handler/ -run 'TestSrsCallback' -v`
Expected: 新增用例 PASS，既有回调用例（有效 token 注册、无效 token 拒绝等）不回归。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/handler/srs_callback_handler.go app/server/internal/handler/srs_callback_handler_test.go
git commit -m "fix(srs): fail closed on publish-block store errors"
```

---

### Task 4: CachedMuteRuleStore.GetFresh 跳过 L1（修复跨实例解禁延迟 🟡）

**Files:**
- Modify: `app/server/internal/sfu/mute_rule_store.go`
- Modify: `app/server/internal/handler/srs_callback_handler.go`
- Test: `app/server/internal/sfu/mute_rule_store_test.go`
- Test: `app/server/internal/handler/srs_callback_handler_test.go`

背景：`CachedMuteRuleStore.Get` 命中 L1 回填（30s TTL）后，其他实例 unmute 只删 shared，本实例 L1 在 30s 内仍返回旧 ruleID，`on_publish` 继续拒推。SRS block 键 ruleID 恒为 1，L1 无价值，应以 shared 为权威。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/sfu/mute_rule_store_test.go` 新增：

```go
func TestCachedMuteRuleStore_GetFreshSkipsL1(t *testing.T) {
	shared := NewMemoryMuteRuleStore()
	cache := NewCachedMuteRuleStore(shared)
	ctx := context.Background()
	if err := cache.Save(ctx, "srs_pub_block:gs-abc", 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	// 模拟另一实例 unmute：只删 shared，本地 L1 仍缓存旧值。
	if err := shared.Delete(ctx, "srs_pub_block:gs-abc"); err != nil {
		t.Fatal(err)
	}
	if id, err := cache.Get(ctx, "srs_pub_block:gs-abc"); err != nil || id != 1 {
		t.Fatalf("plain Get should still hit stale L1 (id=%d err=%v)", id, err)
	}
	fresh, err := cache.GetFresh(ctx, "srs_pub_block:gs-abc")
	if err != nil {
		t.Fatal(err)
	}
	if fresh != 0 {
		t.Fatalf("GetFresh should bypass L1 and see shared delete, got id=%d", fresh)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/ -run TestCachedMuteRuleStore_GetFreshSkipsL1 -v`
Expected: 编译失败（`GetFresh` 未定义）。

- [ ] **Step 3: 最小实现**

在 `app/server/internal/sfu/mute_rule_store.go` 增加接口与实现：

```go
// FreshMuteRuleStore 由需要"以 shared 为权威、跳过 L1"的消费者实现
// （如 SRS 禁推黑名单：ruleID 恒为 1，L1 只会放大跨实例解禁延迟）。
type FreshMuteRuleStore interface {
	MuteRuleStore
	GetFresh(ctx context.Context, key string) (int, error)
}

// GetFresh 跳过本地 L1，直接读 shared 权威值；无 shared 时回退 local。
func (s *CachedMuteRuleStore) GetFresh(ctx context.Context, key string) (int, error) {
	if s == nil {
		return 0, nil
	}
	if s.shared == nil {
		return s.local.Get(ctx, key)
	}
	return s.shared.Get(ctx, key)
}
```

修改 `app/server/internal/handler/srs_callback_handler.go` 的 `on_publish` 分支：

```go
if h.muteStore != nil {
	var blocked int
	var err error
	if fresh, ok := h.muteStore.(sfu.FreshMuteRuleStore); ok {
		// 禁推黑名单以 shared 为权威：跳过 L1，避免跨实例解禁后仍被本实例 L1 拦截。
		blocked, err = fresh.GetFresh(c.Request.Context(), srs.PublishBlockKey(stream))
	} else {
		blocked, err = h.muteStore.Get(c.Request.Context(), srs.PublishBlockKey(stream))
	}
	if err != nil {
		log.Printf("[srs-callback] publish block check failed, denying stream=%s err=%v", stream, err)
		c.JSON(http.StatusOK, gin.H{"code": 1})
		return
	}
	if blocked > 0 {
		log.Printf("[srs-callback] publish blocked by mute rule stream=%s", stream)
		c.JSON(http.StatusOK, gin.H{"code": 1})
		return
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/ ./internal/handler/ -run 'CachedMuteRuleStore|TestSrsCallback' -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/sfu/mute_rule_store.go app/server/internal/sfu/mute_rule_store_test.go app/server/internal/handler/srs_callback_handler.go
git commit -m "fix(srs): bypass L1 cache for publish-block lookups"
```

---

### Task 5: NATSMuteRuleStore 支持 per-key TTL（修复黑名单 24h 失真 🟡）

**Files:**
- Modify: `app/server/internal/bus/mute_rule_store.go`
- Test: `app/server/internal/bus/mute_rule_store_test.go`

背景：`NATSMuteRuleStore.Save` 丢弃 per-key TTL（bucket 固定 24h），导致"永久禁言 24h 后自动失效、短时禁言反而撑满 24h"。修复：value 编码为 `ruleID[:unixExpires]`，`Get` 时校验过期并惰性删除。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/bus/mute_rule_store_test.go` 新增：

```go
func TestNATSMuteRuleStore_PerKeyTTL(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_mute_ttl",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	key := "srs_pub_block:gs-ttl"
	if err := store.Save(ctx, key, 1, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if id, err := store.Get(ctx, key); err != nil || id != 1 {
		t.Fatalf("before expiry id=%d err=%v, want 1", id, err)
	}
	time.Sleep(80 * time.Millisecond)
	if id, err := store.Get(ctx, key); err != nil || id != 0 {
		t.Fatalf("after per-key TTL id=%d err=%v, want 0 (bucket TTL must not override)", id, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/bus/ -run TestNATSMuteRuleStore_PerKeyTTL -v`
Expected: FAIL，80ms 后 `Get` 仍返回 1（per-key TTL 被忽略）。

- [ ] **Step 3: 最小实现**

修改 `app/server/internal/bus/mute_rule_store.go`：

```go
func (s *NATSMuteRuleStore) Save(_ context.Context, key string, ruleID int, ttl time.Duration) error {
	if s == nil || key == "" || ruleID <= 0 {
		return nil
	}
	value := strconv.Itoa(ruleID)
	if ttl > 0 {
		value = fmt.Sprintf("%d:%d", ruleID, time.Now().Add(ttl).Unix())
	}
	_, err := s.kv.Put(muteRuleKVKey(key), []byte(value))
	return err
}

// parseMuteRuleValue 解析 value "ruleID" 或 "ruleID:unixExpires"（兼容旧格式）。
func parseMuteRuleValue(value string) (ruleID int, expiresAt int64, err error) {
	if i := strings.IndexByte(value, ':'); i >= 0 {
		ruleID, err = strconv.Atoi(value[:i])
		if err != nil {
			return 0, 0, err
		}
		expiresAt, _ = strconv.ParseInt(value[i+1:], 10, 64)
		return ruleID, expiresAt, nil
	}
	ruleID, err = strconv.Atoi(value)
	return ruleID, 0, err
}

func (s *NATSMuteRuleStore) Get(_ context.Context, key string) (int, error) {
	if s == nil || key == "" {
		return 0, nil
	}
	entry, err := s.kv.Get(muteRuleKVKey(key))
	if err != nil {
		if err == nats.ErrKeyNotFound || err == nats.ErrKeyDeleted {
			return 0, nil
		}
		return 0, err
	}
	ruleID, expiresAt, err := parseMuteRuleValue(string(entry.Value()))
	if err != nil {
		return 0, nil
	}
	if expiresAt > 0 && time.Now().After(time.Unix(expiresAt, 0)) {
		_ = s.kv.Delete(muteRuleKVKey(key))
		return 0, nil
	}
	return ruleID, nil
}
```

`fmt` 已在 import 中，无需新增。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/bus/ -run 'TestNATSMuteRuleStore' -v`
Expected: 全部 PASS（含既有 SaveGetDelete）。

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/bus/mute_rule_store.go app/server/internal/bus/mute_rule_store_test.go
git commit -m "fix(bus): honor per-key TTL in NATS mute rule store"
```

---

### Task 6: 离线 unmute 黑名单残留 join 自愈（修复 🔴）

**Files:**
- Modify: `app/server/internal/signal/hub_room_join.go`（`OnRoomJoin`）
- Test: `app/server/internal/signal/hub_ws_test.go`（或新建 `hub_join_mute_test.go`）

背景：用户在线被 mute（黑名单已写）后离线，管理员 unmute 时 `enforceUserMediaMute` 因 `targets==0` 提前返回 soft，SRS 黑名单永不删除；用户重连推流被 `on_publish` 拒绝。`OnRoomJoin` 已有 fail-closed 禁言检查，被禁言用户在 join 阶段即被拒（不会走到清理逻辑），因此"禁言检查通过后清理残留"是安全的。stream 由 `streamResolver.StreamName(roomKey, identity)` 生成（`hub_room_join.go:306-308`），与 mute 时 `resolveStream` 回退的 `GenerateStreamName(roomKey, identity)` 一致。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/signal/hub_ws_test.go` 中查看现有 fake client（`connStub`）构造方式，然后新建 `app/server/internal/signal/hub_join_mute_test.go`：

```go
package signal

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/sfu"
)

// recordingMuteProvider 记录 unmute 媒体调用，验证 join 自愈清理。
type recordingMuteProvider struct {
	capsProvider
	unmuted []string // room\x00identity
}

func (p *recordingMuteProvider) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	if !muted {
		p.unmuted = append(p.unmuted, room+"\x00"+identity)
	}
	return p.capsProvider.MuteParticipantTimed(room, identity, trackSid, muted, ttlSeconds)
}

func TestOnRoomJoin_ClearsMuteResidueForUnmutedUser(t *testing.T) {
	prov := &recordingMuteProvider{caps: sfu.Capabilities{
		ServerMute: true, MuteLevel: sfu.EnforcementDegraded,
	}}
	hub := NewHub(nil, nil, &idUserStore{users: map[uint]*model.User{
		1: {ID: 1, Name: "alice"},
	}}, nil)
	hub.sfuProvider = prov

	// muteStore 为 nil 时跳过禁言检查，模拟已解禁用户重连。
	conn := fakeConn("alice") // hub_stability_test.go:76 的 ws.ClientMessenger mock
	_, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`)
	if err != nil {
		t.Fatalf("join should succeed for unmuted user: %v", err)
	}
	if len(prov.unmuted) != 1 {
		t.Fatalf("expected 1 residue-clearing unmute, got %v", prov.unmuted)
	}
	wantRoom := "dom-a:lobby"
	if prov.unmuted[0] != wantRoom+"\x00alice" {
		t.Fatalf("unmute target = %q, want %q", prov.unmuted[0], wantRoom+"\x00alice")
	}
}
```

`fakeConn` 定义于 `app/server/internal/signal/hub_stability_test.go:76`，与 `OnRoomJoin` 测试同包可用；其 `Send`/`ID` 足以支撑 fanout join 与 ack 回写，无需额外 mock。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/signal/ -run TestOnRoomJoin_ClearsMuteResidueForUnmutedUser -v`
Expected: FAIL，`len(prov.unmuted) = 0`（当前 join 不清理残留）。

- [ ] **Step 3: 最小实现**

修改 `app/server/internal/signal/hub_room_join.go` 的 `OnRoomJoin`，在禁言检查通过（未禁言）之后、fanout join 之前插入：

```go
	// 解禁后重连自愈：离线期间 unmute 无法定位 stream（registry/KV 已清），
	// SRS 禁推黑名单可能残留；join 时以 DB 禁言状态为权威重建媒体状态。
	// 被禁言用户在上面的 fail-closed 检查即被拒绝，不会走到这里。
	if h.sfuProvider != nil {
		key := roomKey(req.DomainUUID, req.Room)
		if tp, ok := h.sfuProvider.(sfu.TimedMuteProvider); ok {
			if err := tp.MuteParticipantTimed(key, identity, "", false, 0); err != nil {
				log.Printf("[signal] clear mute residue failed room=%s identity=%s err=%v", key, identity, err)
			}
		} else {
			if err := h.sfuProvider.MuteParticipant(key, identity, "", false); err != nil {
				log.Printf("[signal] clear mute residue failed room=%s identity=%s err=%v", key, identity, err)
			}
		}
	}
```

新增 import：`"GOSpeak/internal/sfu"`（`log` 已存在）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/signal/ -run 'TestOnRoomJoin_ClearsMuteResidueForUnmutedUser|TestBroadcastMute|TestBroadcastUnmute' -v`
Expected: 全部 PASS。

- [ ] **Step 5: 回归测试**

Run: `cd app/server && go test ./internal/signal/ ./internal/handler/`
Expected: 全部通过。

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/signal/hub_room_join.go app/server/internal/signal/hub_join_mute_test.go
git commit -m "fix(signal): clear stale srs publish-block on rejoin"
```

---

### Task 7: 临时禁言到期扫描定时任务（修复到期不广播 🟡）

**Files:**
- Create: `app/server/internal/jobs/mute_expiry.go`
- Create: `app/server/internal/jobs/mute_expiry_test.go`
- Modify: `app/server/server/gin.go`

背景：到期清理只在 `IsMuted`/`ListActiveMutes` 被调用时惰性触发，无定时任务；到期记录不被删除时 `onExpired`（gin.go 已接到 `BroadcastUnmute`）不会执行，订阅端 `serverMutedIdentities` 残留。

- [ ] **Step 1: 写失败测试**

新建 `app/server/internal/jobs/mute_expiry.go` 的测试 `app/server/internal/jobs/mute_expiry_test.go`：

```go
package jobs

import (
	"context"
	"sync"
	"testing"
	"time"
)

type expiryScanner interface {
	ScanExpired()
}

func TestStartMuteExpiryScanner_TriggersScan(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	scan := func() {
		mu.Lock()
		calls++
		mu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartMuteExpiryScanner(ctx, scan, 20*time.Millisecond)
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := calls
		mu.Unlock()
		if n >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected scanner to invoke scan at least twice within 2s")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/jobs/ -run TestStartMuteExpiryScanner_TriggersScan -v`
Expected: 编译失败（`StartMuteExpiryScanner` 未定义）。

- [ ] **Step 3: 最小实现**

新建 `app/server/internal/jobs/mute_expiry.go`：

```go
// Package jobs 提供后台定时任务。
package jobs

import (
	"context"
	"time"
)

// StartMuteExpiryScanner 周期调用 scan（如 MuteService.ListActiveMutes），
// 触发过期禁言的清理与 onExpired 回调（广播 member:unmuted + SFU 恢复）。
// 返回 stop 函数；ctx 取消后停止。
func StartMuteExpiryScanner(ctx context.Context, scan func(), interval time.Duration) (stop func()) {
	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				scan()
			}
		}
	}()
	return func() { close(stopCh) }
}
```

在 `app/server/server/gin.go` 的依赖装配区（`muteSvc.SetOnExpired(...)` 之后）注册：

```go
	// 临时禁言到期扫描：ListActiveMutes 会清理过期记录并触发 onExpired（广播 + SFU 恢复）。
	// 之前的到期清理依赖查询路径惰性触发，无定时任务时到期后订阅端会残留静音状态。
	_ = jobs.StartMuteExpiryScanner(context.Background(), func() {
		if _, err := muteSvc.ListActiveMutes(); err != nil {
			logger.WithComponent("MuteExpiry").Warnf("scan expired mutes: %v", err)
		}
	}, time.Minute)
```

确认 `app/server/server/gin.go` 已 import `app/server/internal/jobs`；未 import 则补充，并确认 `context`、`time`、`logger` 可用（`logger` 若不可用则用 `log.Printf`）。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/jobs/ -v`
Expected: PASS。

- [ ] **Step 5: 构建验证**

Run: `cd app/server && go build ./...`
Expected: 成功。

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/jobs/mute_expiry.go app/server/internal/jobs/mute_expiry_test.go app/server/server/gin.go
git commit -m "feat(jobs): periodic mute expiry scan to broadcast unmute on TTL end"
```

---

## 验收清单

- [ ] `cd app/server && go test ./internal/sfu/... ./internal/signal/... ./internal/handler/... ./internal/bus/... ./internal/jobs/...` 全部通过
- [ ] `cd app/server && go build ./...` 成功
- [ ] `gofmt -l app/server/internal/sfu app/server/internal/signal app/server/internal/handler app/server/internal/bus app/server/internal/jobs` 无输出
- [ ] 手工验证（需要 SRS 实例）：被禁言用户断流重推被 `on_publish` 拒；解禁后离线→重连 join→推流成功；跨实例解禁后 ≤1s 内可重推

## 残余风险

- Cloudflare 跨实例媒体静音（`getSession` 只查本进程）不在本计划，见 `2026-08-13-sfu-provider-hardening.md` T8。
- SRS `on_publish` 拒绝时 WHIP 返回的 HTTP 状态码取决于 SRS 版本，无法在单测断言，见前端计划手工验证清单。
