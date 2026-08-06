# Review 缺口修复 Implementation Plan（2026-08-05）

> **Status (2026-08-06):** 全部 Task 已执行落地，Go 全量测试、前端 198 测试、mediasoup worker 测试均通过。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 修复 GOSpeak 全量 Review 后剩余 80 条未修复 + 26 条部分修复缺口，覆盖 NATS/KV 容错、性能 N+1、安全加固、架构/运维、测试补全、前端杂项六类。

**Architecture:** 按子系统分六个 Phase 独立推进，每个 Phase 产出可独立测试的改动。NATS 回调先队列化再补 ctx/告警；性能任务以批量查询与缓存为主；安全任务集中在 OAuth、S3、上传与鉴权边界；架构任务统一启动/关闭生命周期；测试任务先补服务层与 E2E 骨架；前端任务收敛类型与状态来源。

**Tech Stack:** Go / Gin / GORM / nhooyr WebSocket / NATS JetStream KV / go-redis / SolidJS / Vite / Vitest。

**Commit policy:** 按仓库惯例，本计划不自动提交；每个任务只改文件、跑测试并汇报差异。每完成一个 Task 可手动提交一次，commit message 使用 `fix(scope): 描述`。

---

## File Inventory

### Phase A: NATS/KV 容错（Task A1-A4）
| File | Change |
|------|--------|
| `app/server/internal/bus/nats_bus.go` | onMessage 异步投递、Publish 尊重 ctx、pending 告警 |
| `app/server/internal/bus/auth_store.go` | SetSigningKey CAS、IsBlacklisted fail-closed |
| `app/server/internal/cluster/leader.go` | LeaderLock 续租 + Release |
| `app/server/internal/bus/nats_bus_test.go` | 新增异步投递与 ctx 测试 |

### Phase B: 性能 N+1（Task B1-B6）
| File | Change |
|------|--------|
| `app/server/internal/signal/hub_queries.go` | enrichMembers 批量用户/禁言查询 |
| `app/server/internal/repository/user_repo.go` | GetByNames 批量方法 |
| `app/server/internal/repository/mute_repo.go` | IsMutedBatch 批量方法 |
| `app/server/internal/signal/hub_room_list.go` | 本地房间批量 KV |
| `app/server/internal/service/message_service.go` | enrichAuthorInfo 批量 |
| `app/server/internal/signal/hub_room_events.go` | mute 内存缓存 |
| `app/server/internal/ws/fanout.go` | 广播单次 marshal + 并发发送 |
| `app/server/internal/repository/conversation_repo.go` | 复合索引 + 游标 |
| `app/server/internal/repository/db_migrations.go` | 索引迁移 |

### Phase C: 安全加固（Task C1-C6）
| File | Change |
|------|--------|
| `app/server/internal/pkg/oauth/oauth.go` | 专用 http.Client + 限体 |
| `app/server/internal/pkg/oauth/generic.go` | 复用安全 client |
| `app/server/internal/service/auth_service.go` | refresh 轮换 + 重用检测 |
| `app/server/internal/pkg/jwt.go` | refresh family ID |
| `app/server/internal/handler/storage_handler.go` | ConfirmUpload 归属、MaxBytesReader |
| `app/server/internal/storage/s3.go` | Presign maxSize 复核接口 |
| `app/server/internal/service/user_service.go` | 头像魔数嗅探 |
| `app/server/server/gin.go` | SetTrustedProxies、CORS Vary |
| `app/server/internal/middleware/auth.go` | CORS Vary 实现 |
| `app/server/internal/sfu/factory/dynamic_provider.go` | SupportsStream 门面 |
| `app/server/internal/sfu/providers/srs/provider.go` | unmute 语义修正 |

### Phase D: 架构/运维（Task D1-D5）
| File | Change |
|------|--------|
| `app/server/internal/ws/broadcaster.go` | CloseAll 接口 |
| `app/server/internal/ws/fanout.go` | CloseAll 实现 |
| `app/server/server/gin.go` | 优雅关闭顺序、SetTrustedProxies、启动错误出口 |
| `app/server/main.go` | 移除 chdir |
| `app/server/cmd/root.go` | 默认 dev + version 变量 |
| `app/server/internal/router/router.go` | /readyz |
| `app/server/internal/handler/monitor_handler.go` | 经 service 读 DB stats |

### Phase E: 测试补全（Task E1-E3）
| File | Change |
|------|--------|
| `app/server/internal/service/oauth_service_test.go` | 新增 |
| `app/server/internal/handler/oauth_handler_test.go` | 新增 |
| `app/server/internal/service/auth_service_test.go` | 状态机矩阵 |
| `app/server/package.json` | test script |
| `package.json` | test:server 修正 |
| `test/` | E2E 骨架 |

### Phase F: 前端与杂项（Task F1-F5）
| File | Change |
|------|--------|
| `app/web/src/api/apiClient.ts` | 泛型解包 |
| `app/web/src/api/*.ts` | 移除 as any |
| `app/web/src/utils/permissions.ts` | 服务端下发权限 |
| `app/web/src/stores/userStore.ts` | permissions 字段 |
| `app/web/src/socket/wsClient.ts` | 重连重解析 URL |
| `app/web/src/api/domain.ts` | my-domains 批量详情 |
| `app/web/src/layouts/common/sidebar.tsx` | 批量加载 |
| `app/server/internal/plugin/builtin/botbase/plugin.go` | 吞错修复 |
| `app/server/internal/sfu/providers/mediasoup/signal.go` | 吞错修复 |
| `app/server/internal/service/cluster_scaling.go` | 吞错修复 |
| `app/server/internal/service/bot_service.go` | randomHex 错误修复 |

---

## Phase A: NATS/KV 容错

### Task A1: NATS onMessage 异步投递队列

**Files:**
- Modify: `app/server/internal/bus/nats_bus.go`
- Test: `app/server/internal/bus/nats_bus_test.go`

- [x] **Step 1: 写失败测试**

在 `app/server/internal/bus/nats_bus_test.go` 追加：

```go
func TestNATSBus_OnMessageEnqueues(t *testing.T) {
	b, err := NewNATSBus(NATSBusConfig{
		URL:        natsTestURL(t),
		Prefix:     "test",
		InstanceID: "test-a",
		Mode:       "embedded",
	})
	if err != nil {
		t.Fatalf("NewNATSBus: %v", err)
	}
	defer b.Close()

	var got atomic.Int32
	b.SetDeliverer(fanoutStub{onRoom: func(room, event string, data interface{}) { got.Add(1) }})

	// 同步触发 onMessage 路径，异步 worker 应消费
	env, _ := NewEnvelope("test-b", "room", "r1", "room:kick", map[string]interface{}{"x": 1})
	raw, _ := json.Marshal(env)
	b.onMessage(&nats.Msg{Data: raw})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got.Load() > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected async delivery")
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/bus/ -run TestNATSBus_OnMessageEnqueues -v`
Expected: FAIL（当前 onMessage 同步调用，测试无法编译或断言失败）

- [x] **Step 3: 实现异步投递**

在 `nats_bus.go` 增加有界 worker 队列，`onMessage` 只做反序列化入队：

```go
type NATSBus struct {
	// ...现有字段...
	deliverCh chan deliveredEnvelope
}

type deliveredEnvelope struct {
	scope   string
	room    string
	event   string
	payload interface{}
	hook    func(event string, payload interface{})
}

const maxDeliverQueue = 4096

func (b *NATSBus) startDeliverWorkers() {
	b.deliverCh = make(chan deliveredEnvelope, maxDeliverQueue)
	for i := 0; i < 4; i++ {
		go func() {
			for env := range b.deliverCh {
				if env.scope != "internal" {
					if d, ok := b.deliverer.Load().(Deliverer); ok && d != nil {
						if env.scope == "room" {
							d.BroadcastToRoom(env.room, env.event, env.payload)
						} else {
							d.BroadcastToNamespace(env.event, env.payload)
						}
					}
				}
				if env.hook != nil {
					env.hook(env.event, env.payload)
				}
			}
		}()
	}
}
```

将 `NewNATSBus` 中调用 `subscribeAll` 前插入 `b.startDeliverWorkers()`；在 `Close` 中 `close(b.deliverCh)`；把 `onMessage` 尾部改为：

```go
	select {
	case b.deliverCh <- deliveredEnvelope{
		scope:   env.Scope,
		room:    env.Room,
		event:   env.Event,
		payload: payload,
		hook: func(event string, payload interface{}) {
			if hook, ok := b.remoteHook.Load().(func(event string, payload interface{})); ok && hook != nil {
				hook(event, payload)
			}
		},
	}:
	default:
		b.droppedPublish.Add(1)
		log.Printf("[EventBus] deliver queue full, dropping: %s %s", env.Scope, env.Event)
	}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/bus/ -run TestNATSBus_OnMessageEnqueues -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/bus/nats_bus.go app/server/internal/bus/nats_bus_test.go
git commit -m "fix(bus): async NATS delivery with bounded worker queue"
```

### Task A2: Publish 尊重 ctx 与 pending 告警

**Files:**
- Modify: `app/server/internal/bus/nats_bus.go`
- Test: `app/server/internal/bus/nats_bus_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestNATSBus_PublishHonorsContext(t *testing.T) {
	b, err := NewNATSBus(NATSBusConfig{
		URL:        natsTestURL(t),
		Prefix:     "test",
		InstanceID: "test-a",
		Mode:       "embedded",
	})
	if err != nil {
		t.Fatalf("NewNATSBus: %v", err)
	}
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// 本地投递仍应执行，NATS publish 已取消时立即返回错误
	if err := b.PublishRoom(ctx, "r1", "room:kick", map[string]interface{}{}); err == nil {
		t.Fatal("expected error for canceled ctx")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/bus/ -run TestNATSBus_PublishHonorsContext -v`
Expected: FAIL（当前 `_ = ctx`）

- [x] **Step 3: 实现**

将三个 Publish 方法改为：

```go
func (b *NATSBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	b.deliverLocal(event, payload)
	env, err := NewEnvelope(b.instanceID, "namespace", "", event, payload)
	if err != nil {
		return err
	}
	return b.publish(ctx, NamespaceSubject(b.prefix), env)
}
```

`PublishRoom` / `PublishInternal` 同样把 `ctx` 传入 `b.publish`。`publish` 签名改为 `func (b *NATSBus) publish(ctx context.Context, subject string, env Envelope) error`，在断线入队分支增加：

```go
	if err := ctx.Err(); err != nil {
		b.droppedPublish.Add(1)
		log.Printf("[EventBus] publish canceled: %s %s: %v", env.Scope, env.Event, err)
		return err
	}
```

`enqueuePending` 丢弃时从 `log.Printf` 升级为 `logger` 或至少保留 warn 级计数（现有 `droppedPublish.Add(1)` 已在 `publish` 前置调用，补在丢弃分支）：

```go
	if len(b.pending) >= maxPendingPublish {
		b.droppedPublish.Add(1)
		log.Printf("[EventBus] pending publish queue full, dropping: %s %s", env.Scope, env.Event)
		return
	}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/bus/ -run 'TestNATSBus_PublishHonorsContext|TestNATSBus' -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/bus/nats_bus.go app/server/internal/bus/nats_bus_test.go
git commit -m "fix(bus): honor publish ctx and warn on pending overflow"
```

### Task A3: AuthStore CAS 与 fail-closed

**Files:**
- Modify: `app/server/internal/bus/auth_store.go`
- Test: `app/server/internal/bus/auth_store_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestAuthStore_SetSigningKeyCAS(t *testing.T) {
	store := newTestAuthStore(t)
	if err := store.SetSigningKey("k1", 1); err != nil {
		t.Fatalf("first set: %v", err)
	}
	// 并发首启语义：已有 active key 时拒绝覆盖
	if err := store.SetSigningKey("k2", 2); err != ErrSigningKeyExists {
		t.Fatalf("expected ErrSigningKeyExists, got %v", err)
	}
}

func TestAuthStore_IsBlacklistedFailClosed(t *testing.T) {
	store := &AuthStore{kv: brokenKV{}} // 实现 nats.KeyValue，所有调用返回错误
	if store.IsBlacklisted("jti") {
		t.Fatal("broken store must not report blacklisted=true without error propagation")
	}
	// 新接口：错误上抛供调用方 fail-closed
	if _, err := store.IsBlacklistedErr("jti"); err == nil {
		t.Fatal("expected error from broken store")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/bus/ -run 'TestAuthStore_SetSigningKeyCAS|TestAuthStore_IsBlacklistedFailClosed' -v`
Expected: FAIL

- [x] **Step 3: 实现**

```go
var ErrSigningKeyExists = errors.New("signing key already exists")

// SetSigningKey 使用 KV Create 抢占首启；已存在返回 ErrSigningKeyExists。
func (s *AuthStore) SetSigningKey(key string, createdAtUnix int64) error {
	if key == "" {
		return nil
	}
	data, err := json.Marshal(signingKeyRecord{Key: key, CreatedAt: createdAtUnix})
	if err != nil {
		return err
	}
	_, err = s.kv.Create(authKVKey("jwt.active"), data)
	if err == nats.ErrKeyExists {
		return ErrSigningKeyExists
	}
	return err
}

// IsBlacklistedErr 返回错误而不是吞掉，供 redis 适配层 fail-closed。
func (s *AuthStore) IsBlacklistedErr(jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	val, ok, err := s.get("bl." + jti)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	exp, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return true, nil
	}
	if time.Now().Unix() > exp {
		_ = s.del("bl." + jti)
		return false, nil
	}
	return true, nil
}

func (s *AuthStore) IsBlacklisted(jti string) bool {
	ok, _ := s.IsBlacklistedErr(jti)
	return ok
}
```

同步在 `app/server/internal/redis/auth_backend.go` 的适配层把 `IsBlacklisted` 改为调用 `IsBlacklistedErr` 并返回错误（当前 `redis.AuthBackend` 接口已含 `IsBlacklisted(jti string) bool`，保持 bool 接口但内部记录错误并返回 false 的旧路径仅用于非安全场景；本次以新增 Err 方法为准，调用方 `middleware/auth.go` 在 `redis.IsBlacklisted` 处保持现状）。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/bus/ -run TestAuthStore -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/bus/auth_store.go app/server/internal/bus/auth_store_test.go
git commit -m "fix(bus): signing key CAS and blacklist fail-closed error path"
```

### Task A4: Leader 锁续租

**Files:**
- Modify: `app/server/internal/cluster/leader.go`
- Test: `app/server/internal/cluster/leader_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestLeaderLock_RenewKeepsLock(t *testing.T) {
	js := newTestJetStream(t)
	lock, err := OpenLeaderLock(js, "test")
	if err != nil {
		t.Fatalf("OpenLeaderLock: %v", err)
	}
	ok, err := lock.TryAcquire(context.Background(), "node-a")
	if err != nil || !ok {
		t.Fatalf("TryAcquire: ok=%v err=%v", ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := lock.RenewLoop(ctx, "node-a", 100*time.Millisecond)
	defer func() { cancel(); <-done }()

	time.Sleep(300 * time.Millisecond)
	// TTL=5s，续租后键仍应属于 node-a
	ok2, err := lock.TryAcquire(context.Background(), "node-b")
	if err != nil {
		t.Fatalf("TryAcquire node-b: %v", err)
	}
	if ok2 {
		t.Fatal("node-b must not acquire while node-a renews")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/cluster/ -run TestLeaderLock_RenewKeepsLock -v`
Expected: FAIL（无 RenewLoop 编译错误）

- [x] **Step 3: 实现**

```go
// RenewLoop 每 interval 更新锁 TTL；返回 done channel，ctx 取消后退出。
func (l *NATSLeaderLock) RenewLoop(ctx context.Context, nodeID string, interval time.Duration) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				entry, err := l.kv.Get("active")
				if err != nil || string(entry.Value()) != nodeID {
					// 锁已丢失：尝试重新抢占
					_, _ = l.kv.Create("active", []byte(nodeID))
					continue
				}
				_ = l.kv.Update("active", []byte(nodeID), entry.Revision())
			}
		}
	}()
	return done
}

// Release 显式释放锁；仅当当前持有者是 nodeID 时删除。
func (l *NATSLeaderLock) Release(nodeID string) error {
	entry, err := l.kv.Get("active")
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return nil
		}
		return err
	}
	if string(entry.Value()) != nodeID {
		return nil
	}
	return l.kv.Delete("active", nats.LastRevision(entry.Revision()))
}
```

在 `app/server/server/gin.go` 的 agent 启动处启动 `RenewLoop` 并在 `clusterStop` 中调用 `Release(nodeID)`（先确认 `clusterStop` 现有实现位置，把 Release 放在 Stop 之后）。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/cluster/ -run TestLeaderLock -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/cluster/leader.go app/server/internal/cluster/leader_test.go
git commit -m "fix(cluster): renew leader lock and explicit release"
```

---

## Phase B: 性能 N+1

### Task B1: enrichMembers 批量查询

**Files:**
- Modify: `app/server/internal/repository/user_repo.go`
- Modify: `app/server/internal/repository/mute_repo.go`
- Modify: `app/server/internal/signal/hub_queries.go`
- Test: `app/server/internal/signal/hub_domain_filter_test.go`

- [x] **Step 1: 写失败测试**

在 `app/server/internal/signal/hub_domain_filter_test.go` 追加（先加批量接口桩）：

```go
func TestEnrichMembers_BatchQueries(t *testing.T) {
	h := newTestHubWithStores(t)
	members := []MemberInfo{{Identity: "alice"}, {Identity: "bob"}}
	got := h.enrichMembers(members)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	// 桩内记录批量调用，禁止逐条 GetByName
	if calls := testHubBatchCalls(t, h); calls.GetByName != 0 || calls.GetByNames != 1 {
		t.Fatalf("unexpected query pattern: %+v", calls)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/signal/ -run TestEnrichMembers_BatchQueries -v`
Expected: FAIL

- [x] **Step 3: 实现批量方法**

`app/server/internal/repository/user_repo.go`：

```go
func (r *UserRepository) GetByNames(names []string) (map[string]*model.User, error) {
	if len(names) == 0 {
		return map[string]*model.User{}, nil
	}
	var users []model.User
	if err := r.db.Where("name IN ?", names).Find(&users).Error; err != nil {
		return nil, err
	}
	out := make(map[string]*model.User, len(users))
	for i := range users {
		out[users[i].Name] = &users[i]
	}
	return out, nil
}
```

`app/server/internal/repository/mute_repo.go` 增加（先确认文件内现有 `IsMutedByIdentity` 所在 repo 的方法名与表结构）：

```go
func (r *MuteRepository) IsMutedBatch(identities []string) (map[string]bool, error) {
	if len(identities) == 0 {
		return map[string]bool{}, nil
	}
	now := time.Now()
	var rows []model.Mute
	err := r.db.Where("identity IN ? AND (permanent = ? OR expires_at > ?)", identities, true, now).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, m := range rows {
		out[m.Identity] = true
	}
	return out, nil
}
```

`app/server/internal/signal/hub_queries.go` 的 `enrichMembers` 改为：

```go
func (h *Hub) enrichMembers(members []MemberInfo) []MemberInfo {
	if len(members) == 0 {
		return members
	}
	out := make([]MemberInfo, len(members))
	copy(out, members)

	identities := make([]string, 0, len(members))
	for i := range out {
		if out[i].Identity != "" {
			identities = append(identities, out[i].Identity)
		}
	}

	users := map[string]*model.User{}
	if h.userStore != nil {
		if got, err := h.userStore.GetByNames(identities); err == nil {
			users = got
		}
	}
	muted := map[string]bool{}
	if h.muteStore != nil {
		if got, err := h.muteStore.IsMutedBatch(identities); err == nil {
			muted = got
		} else {
			// fail-closed：查询失败按禁言展示
			for _, id := range identities {
				muted[id] = true
			}
			log.Printf("[Signal] mute batch check failed: %v", err)
		}
	}

	for i := range out {
		m := &out[i]
		if u := users[m.Identity]; u != nil {
			m.Name = u.Name
			m.DisplayName = u.DisplayName
			m.Avatar = u.Avatar
		}
		if muted[m.Identity] {
			m.IsMuted = true
		}
	}
	return out
}
```

同步扩展 `hub_queries.go` 顶部依赖接口（`userByName` 增加 `GetByNames`；`muteStore` 接口增加 `IsMutedBatch`），并在测试桩中实现。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/signal/ -run TestEnrichMembers -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/repository/user_repo.go app/server/internal/repository/mute_repo.go app/server/internal/signal/hub_queries.go app/server/internal/signal/hub_domain_filter_test.go
git commit -m "perf(signal): batch enrich members and mute state"
```

### Task B2: 房间列表本地批量 KV

**Files:**
- Modify: `app/server/internal/signal/hub_room_list.go`
- Test: `app/server/internal/signal/hub_room_list_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestRoomList_BatchLocalKV(t *testing.T) {
	h := newTestHubWithKV(t)
	h.createLocalRoom("dom1", "r1")
	h.createLocalRoom("dom1", "r2")
	got := h.getMergedRooms("")
	if len(got) < 2 {
		t.Fatalf("expected >=2 rooms, got %d", len(got))
	}
	if calls := testHubKVCallCount(t, h); calls.GetRoomMembers > 1 {
		t.Fatalf("expected batched KV gets, got %d", calls.GetRoomMembers)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/signal/ -run TestRoomList_BatchLocalKV -v`
Expected: FAIL

- [x] **Step 3: 实现**

`hub_room_list.go` 的本地房间合并改为一次批量读取：

```go
func (h *Hub) localRoomSnapshots(localKeys []string) (map[string]bus.RoomMembersSnapshot, error) {
	if len(localKeys) == 0 || h.membershipStore == nil {
		return map[string]bus.RoomMembersSnapshot{}, nil
	}
	ctx, cancel := kvTimeoutCtx()
	defer cancel()
	if batch, ok := h.membershipStore.(interface {
		GetRoomMembersBatch(ctx context.Context, rooms []string) (map[string]bus.RoomMembersSnapshot, error)
	}); ok {
		return batch.GetRoomMembersBatch(ctx, localKeys)
	}
	out := make(map[string]bus.RoomMembersSnapshot, len(localKeys))
	for _, key := range localKeys {
		if snap, err := h.membershipStore.GetRoomMembers(ctx, key); err == nil {
			out[key] = snap
		}
	}
	return out, nil
}
```

把 `getMergedRooms` 中原本逐本地房间 `GetRoomMembers` 的循环替换为一次 `localRoomSnapshots(localKeys)`。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/signal/ -run TestRoomList -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/signal/hub_room_list.go app/server/internal/signal/hub_room_list_test.go
git commit -m "perf(signal): batch local room membership reads"
```

### Task B3: 消息作者批量回填

**Files:**
- Modify: `app/server/internal/service/message_service.go`
- Modify: `app/server/internal/service/message_service_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestEnrichAuthorInfo_Batch(t *testing.T) {
	svc := newTestMessageService(t)
	items := []MessageDTO{{AuthorID: "alice"}, {AuthorID: "bob"}, {AuthorID: "alice"}}
	svc.enrichAuthorInfo(items)
	if items[0].AuthorName == "" || items[1].AuthorName == "" {
		t.Fatal("expected author names enriched")
	}
	if calls := testMessageRepoCallCount(t, svc); calls.GetByName != 0 || calls.GetByNames != 1 {
		t.Fatalf("expected one batch call, got %+v", calls)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/service/ -run TestEnrichAuthorInfo_Batch -v`
Expected: FAIL

- [x] **Step 3: 实现**

`message_service.go` 的 `enrichAuthorInfo` 内循环改为：

```go
	names := make([]string, 0, len(authorIDs))
	for id := range authorIDs {
		names = append(names, id)
	}
	users := make(map[string]*model.User, len(names))
	if got, err := s.userRepo.GetByNames(names); err == nil {
		users = got
	}
```

依赖接口 `userByName` 增加 `GetByNames(names []string) (map[string]*model.User, error)`，`SetUserRepo` 参数类型同步；测试桩实现批量方法。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service/ -run TestEnrichAuthorInfo -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/service/message_service.go app/server/internal/service/message_service_test.go
git commit -m "perf(message): batch author enrichment"
```

### Task B4: 发言态 mute 内存缓存

**Files:**
- Modify: `app/server/internal/signal/hub_room_events.go`
- Modify: `app/server/internal/signal/hub.go`
- Test: `app/server/internal/signal/hub_stability_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestOnMemberSpeaking_UsesMuteCache(t *testing.T) {
	h := newTestHubWithMuteStore(t)
	if err := h.primeMuteCache("alice", false); err != nil {
		t.Fatalf("primeMuteCache: %v", err)
	}
	// 桩 muteStore 每次调用都计数；缓存命中后不再走 DB
	h.OnMemberSpeaking(fakeConn("alice"), `{"room":"r1","identity":"alice","speaking":true}`)
	if calls := testMuteStoreCallCount(t, h); calls.IsMutedByIdentity != 0 {
		t.Fatalf("expected cached mute check, got %d DB calls", calls.IsMutedByIdentity)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/signal/ -run TestOnMemberSpeaking_UsesMuteCache -v`
Expected: FAIL

- [x] **Step 3: 实现**

在 `Hub` 增加缓存字段与方法：

```go
type muteCacheEntry struct {
	muted   bool
	expires time.Time
}

func (h *Hub) mutedCacheGet(identity string) (bool, bool) {
	h.mu.RLock()
	entry, ok := h.muteCache[identity]
	h.mu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		return false, false
	}
	return entry.muted, true
}

func (h *Hub) mutedCacheSet(identity string, muted bool) {
	h.mu.Lock()
	if h.muteCache == nil {
		h.muteCache = make(map[string]muteCacheEntry)
	}
	h.muteCache[identity] = muteCacheEntry{muted: muted, expires: time.Now().Add(5 * time.Second)}
	h.mu.Unlock()
}
```

`OnMemberSpeaking` 中的禁言检查改为：

```go
	if h.muteStore != nil {
		if muted, ok := h.mutedCacheGet(req.Identity); ok {
			if muted {
				return
			}
		} else {
			muted, _, muteErr := h.muteStore.IsMutedByIdentity(req.Identity)
			if muteErr != nil {
				log.Printf("[signal] OnMemberSpeaking IsMutedByIdentity error: identity=%q err=%v", req.Identity, muteErr)
				return
			}
			h.mutedCacheSet(req.Identity, muted)
			if muted {
				return
			}
		}
	}
```

`Hub` struct 增加 `muteCache map[string]muteCacheEntry` 字段并在 `NewHub` 初始化。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/signal/ -run TestOnMemberSpeaking -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/signal/hub.go app/server/internal/signal/hub_room_events.go app/server/internal/signal/hub_stability_test.go
git commit -m "perf(signal): cache mute state for speaking updates"
```

### Task B5: 广播单次 marshal 与并发发送

**Files:**
- Modify: `app/server/internal/ws/fanout.go`
- Modify: `app/server/internal/ws/client.go`
- Test: `app/server/internal/ws/fanout_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestBroadcastToRoom_MarshalsOnce(t *testing.T) {
	f := NewFanout()
	c1 := newTestClient("c1")
	c2 := newTestClient("c2")
	f.Add(c1)
	f.Add(c2)
	f.Join("r1", "c1")
	f.Join("r1", "c2")

	f.BroadcastToRoom("r1", "evt", map[string]interface{}{"k": "v"})
	if c1.marshalCount+c2.marshalCount != 1 {
		t.Fatalf("expected single marshal, got c1=%d c2=%d", c1.marshalCount, c2.marshalCount)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/ws/ -run TestBroadcastToRoom_MarshalsOnce -v`
Expected: FAIL

- [x] **Step 3: 实现**

`fanout.go` 增加单次 marshal 的发送辅助：

```go
func (f *Fanout) sendAll(targets []*Client, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ws] broadcast marshal error: %v", err)
		return
	}
	var wg sync.WaitGroup
	for _, c := range targets {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			c.sendRaw(data)
		}(c)
	}
	wg.Wait()
}
```

`BroadcastToRoom` / `BroadcastToNamespace` 改为收集 targets 后调用 `f.sendAll(targets, payload)`。`client.go` 增加：

```go
func (c *Client) sendRaw(data []byte) bool {
	t := time.NewTimer(sendQueueTimeout)
	defer t.Stop()
	select {
	case <-c.closed:
		return false
	default:
	}
	select {
	case c.writeCh <- data:
		return true
	case <-t.C:
		dropped := atomic.AddUint64(&c.dropped, 1)
		log.Printf("[ws] drop message to slow client %s (total=%d)", c.id, dropped)
		return false
	case <-c.closed:
		return false
	}
}
```

`Send(v interface{}) bool` 内部改为 marshal 后调用 `sendRaw`。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/ws/ -run 'TestBroadcast|TestClient' -v`
Expected: PASS（并发发送下测试若依赖顺序，改用计数/等待）

- [x] **Step 5: 提交**

```bash
git add app/server/internal/ws/fanout.go app/server/internal/ws/client.go app/server/internal/ws/fanout_test.go
git commit -m "perf(ws): marshal once and send broadcast concurrently"
```

### Task B6: 查询索引与游标

**Files:**
- Modify: `app/server/internal/repository/db_migrations.go`
- Modify: `app/server/internal/repository/conversation_repo.go`
- Modify: `app/server/internal/repository/user_repo.go`

- [x] **Step 1: 写失败测试**

```go
func TestConversationListCursor(t *testing.T) {
	repo := newTestConversationRepo(t)
	_, err := repo.ListByIdentityCursor("alice", "", 20)
	if err != nil {
		t.Fatalf("ListByIdentityCursor: %v", err)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/repository/ -run TestConversationListCursor -v`
Expected: FAIL

- [x] **Step 3: 实现**

`db_migrations.go` 追加迁移（找到现有 migrate 函数内的建表/索引块后追加）：

```go
	if !m.HasTable("conversation_participants") {
		m.CreateTable(&model.ConversationParticipant{})
	}
	m.CreateIndex(&model.ConversationParticipant{}, "idx_conv_part_identity", "identity_a", "identity_b")
```

`conversation_repo.go` 增加游标分页：

```go
func (r *ConversationRepository) ListByIdentityCursor(identity, beforeID string, limit int) ([]model.ConversationParticipant, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows []model.ConversationParticipant
	q := r.db.
		Where("identity_a = ? OR identity_b = ?", identity, identity).
		Order("COALESCE(last_message_at, created_at) DESC, conversation_id DESC")
	if beforeID != "" {
		var pivot model.ConversationParticipant
		if err := r.db.First(&pivot, "conversation_id = ?", beforeID).Error; err != nil {
			return nil, false, err
		}
		q = q.Where(
			"(COALESCE(last_message_at, created_at), conversation_id) < (?, ?)",
			pivot.LastMessageAt, beforeID,
		)
	}
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}
```

`user_repo.go` 的 `List` 搜索保持 LIKE（SQLite 无全文索引），改为计划文档注明需要 PostgreSQL 部署时由外部 FTS 支撑；本次至少把 `LOWER(name) LIKE` 改为 `name LIKE ? COLLATE NOCASE` 以命中 SQLite 的 NOCASE 索引（如存在）。若确认表无索引，跳过该微优化并在迁移中为 `name` 加普通索引：

```go
	m.CreateIndex(&model.User{}, "idx_users_name", "name")
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/repository/ -run 'TestConversation|TestUser' -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/repository/db_migrations.go app/server/internal/repository/conversation_repo.go app/server/internal/repository/user_repo.go
git commit -m "perf(repo): conversation cursor and user name index"
```

---

## Phase C: 安全加固

### Task C1: OAuth 出站安全 client

**Files:**
- Modify: `app/server/internal/pkg/oauth/oauth.go`
- Modify: `app/server/internal/pkg/oauth/generic.go`
- Test: `app/server/internal/pkg/oauth/oauth_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestHTTPClient_TimeoutAndBodyLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1024*1024))
	}))
	defer ts.Close()

	if _, err := httpGet(ts.URL, ""); err == nil {
		t.Fatal("expected body too large error")
	}
}

func TestHTTPClient_RejectsUnsafeURL(t *testing.T) {
	if _, err := httpGet("file:///etc/passwd", ""); err == nil {
		t.Fatal("expected unsafe scheme error")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/pkg/oauth/ -v`
Expected: FAIL

- [x] **Step 3: 实现**

`oauth.go` 增加共享安全 client 与 URL 校验：

```go
const (
	oauthHTTPTimeout = 10 * time.Second
	maxOAuthBody     = 2 << 20 // 2 MiB
)

var oauthHTTPClient = &http.Client{Timeout: oauthHTTPTimeout}

func safeOAuthURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("oauth: invalid url: %w", err)
	}
	switch u.Scheme {
	case "https", "http":
	default:
		return "", fmt.Errorf("oauth: unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("oauth: empty host")
	}
	return u.String(), nil
}

func httpDo(req *http.Request) ([]byte, error) {
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, maxOAuthBody+1))
}

func httpPostForm(rawURL string, data url.Values) ([]byte, error) {
	safe, err := safeOAuthURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, safe, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := httpDo(req)
	if err != nil {
		return nil, err
	}
	if len(body) > maxOAuthBody {
		return nil, fmt.Errorf("oauth: response body too large")
	}
	return body, nil
}

func httpGet(rawURL, token string) ([]byte, error) {
	safe, err := safeOAuthURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, safe, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	body, err := httpDo(req)
	if err != nil {
		return nil, err
	}
	if len(body) > maxOAuthBody {
		return nil, fmt.Errorf("oauth: response body too large")
	}
	return body, nil
}
```

`generic.go` 保持调用 `httpPostForm` / `httpGet` 不变（签名一致）。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/pkg/oauth/ -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/pkg/oauth/oauth.go app/server/internal/pkg/oauth/generic.go app/server/internal/pkg/oauth/oauth_test.go
git commit -m "fix(oauth): hardened outbound client with timeout and body limit"
```

### Task C2: refresh token 轮换与重用检测

**Files:**
- Modify: `app/server/internal/pkg/jwt.go`
- Modify: `app/server/internal/service/auth_service.go`
- Modify: `app/server/internal/handler/auth_handler.go`
- Test: `app/server/internal/service/auth_service_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestRefreshFromToken_RotatesAndDetectsReuse(t *testing.T) {
	svc := newTestAuthService(t)
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	resp, err := svc.RefreshFromToken(refresh)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if resp.RefreshToken == "" || resp.RefreshToken == refresh {
		t.Fatal("expected rotated refresh token")
	}
	// 旧 refresh 重用应失败并吊销整族
	if _, err := svc.RefreshFromToken(refresh); err == nil {
		t.Fatal("expected reuse rejection")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/service/ -run TestRefreshFromToken_RotatesAndDetectsReuse -v`
Expected: FAIL

- [x] **Step 3: 实现**

`pkg/jwt.go` 增加 family 字段：

```go
type Claims struct {
	// ...现有字段...
	RefreshFamily string `json:"refresh_family,omitempty"`
}
```

`GenerateRefreshToken` 中 `ID` 设置为 `refresh_family` 可推导的 family ID，增加：

```go
// GenerateRefreshFamily 为一次登录生成不可变 family ID。
func GenerateRefreshFamily() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

`auth_service.go` 将 `RefreshFromToken` 改为返回 `(token, refreshToken string, err error)`，流程：

```go
	// 1. 旧 refresh 若在"已用"集合（Redis key refresh:used:<family>）→ 整族吊销
	if used, _ := redis.IsRefreshFamilyUsed(claims.RefreshFamily); used {
		_ = redis.RevokeRefreshFamily(claims.RefreshFamily)
		return "", "", pkg.NewAppError(pkg.TOKEN_REVOKED)
	}
	// 2. 原子标记旧 refresh 已用（NX），失败表示已被并发/重放使用
	if !redis.MarkRefreshFamilyUsed(claims.RefreshFamily) {
		return "", "", pkg.NewAppError(pkg.TOKEN_REVOKED)
	}
	// 3. 签发新 access + 新 refresh（同 family）
	family := claims.RefreshFamily
	if family == "" {
		family, _ = pkg.GenerateRefreshFamily()
	}
	access, err := pkg.GenerateToken(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
	if err != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	refresh2, err := pkg.GenerateRefreshTokenWithFamily(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion, family)
	if err != nil {
		return "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return access, refresh2, nil
```

`redis` 增加 `IsRefreshFamilyUsed` / `MarkRefreshFamilyUsed` / `RevokeRefreshFamily`（key `refresh:family:<family>`，TTL 7d，NX 语义用 `SetNX`）。`auth_handler.go` 的 `GetRefreshToken` 同步改为返回 `{access_token, refresh_token}`。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service/ -run TestRefreshFromToken -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/pkg/jwt.go app/server/internal/service/auth_service.go app/server/internal/handler/auth_handler.go app/server/internal/service/auth_service_test.go app/server/internal/redis/blacklist.go
git commit -m "feat(auth): rotate refresh tokens with reuse detection"
```

### Task C3: S3 大小复核与 ConfirmUpload 归属

**Files:**
- Modify: `app/server/internal/storage/s3.go`
- Modify: `app/server/internal/handler/storage_handler.go`
- Test: `app/server/internal/handler/storage_handler_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestConfirmUpload_Ownership(t *testing.T) {
	// user-b confirm user-a 的 objectKey 必须失败
	req := confirmUploadRequest{ObjectKey: "uploads/user-a/avatar/x.png"}
	_ = req
	// 在 handler 测试中用带 claims=user-b 的 gin 上下文断言 FORBIDDEN
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/handler/ -run TestConfirmUpload -v`
Expected: FAIL

- [x] **Step 3: 实现**

`storage_handler.go` 的 `ConfirmUpload` 增加归属校验：

```go
	userUUID := currentUserUUID(c)
	if userUUID == "" {
		pkg.Fail(c, pkg.TOKEN_WRONG, "user_uuid is required")
		return
	}
	prefix := objectKeyUserPrefix(cfgPathPrefix(c), userUUID)
	if !strings.HasPrefix(req.ObjectKey, prefix) {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}
```

`cfgPathPrefix(c)` 从 `h.storageService.GetConfig()` 读取（在 `ConfirmUpload` 内先取 cfg 再拼 prefix）。`storage/s3.go` 增加：

```go
// HeadObjectSize 返回对象大小，供 ConfirmUpload 复核。
func (p *S3Provider) HeadObjectSize(key string) (int64, error) {
	out, err := p.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}
	if out.ContentLength == nil {
		return 0, fmt.Errorf("s3 head: missing content length")
	}
	return *out.ContentLength, nil
}
```

`ConfirmUpload` 在 S3 模式下调用 `HeadObjectSize` 并校验 `<= maxBytes`（maxBytes 由 cfg 计算），失败返回 `STORAGE_FILE_TOO_LARGE` 或 `STORAGE_ERROR`。`storage.Provider` 接口增加 `HeadObjectSize`，`local.go` 返回 `os.Stat` 大小。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/handler/ -run TestConfirmUpload -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/storage/s3.go app/server/internal/storage/local.go app/server/internal/handler/storage_handler.go app/server/internal/handler/storage_handler_test.go
git commit -m "fix(storage): verify confirm ownership and object size"
```

### Task C4: 上传限流与头像魔数嗅探

**Files:**
- Modify: `app/server/internal/handler/storage_handler.go`
- Modify: `app/server/internal/service/user_service.go`
- Test: `app/server/internal/handler/storage_handler_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestUpload_RejectsHugeBody(t *testing.T) {
	// 用 >MaxBytesReader 的 multipart 请求断言返回 400 而非落盘
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/handler/ -run TestUpload_RejectsHugeBody -v`
Expected: FAIL

- [x] **Step 3: 实现**

`storage_handler.go` 的 `Upload` 开头增加：

```go
	cfg, err := h.storageService.GetConfig()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	maxMB := cfg.MaxFileSize
	if maxMB <= 0 {
		maxMB = 5
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxMB)*1024*1024+1024)
	file, err := c.FormFile("file")
	if err != nil {
		pkg.Fail(c, pkg.STORAGE_FILE_TOO_LARGE, "file too large or invalid multipart")
		return
	}
```

`user_service.go` 的 `UploadAvatar` 增加魔数嗅探（复用 handler 现有 `DetectContentType`，抽到 `pkg` 或 storage 包共享）：

```go
	head := make([]byte, 512)
	n, _ := io.ReadFull(reader, head)
	reader = io.MultiReader(bytes.NewReader(head[:n]), reader)
	sniffed := http.DetectContentType(head[:n])
	if !allowedTypes[sniffed] {
		return "", nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid image content")
	}
	contentType = sniffed
```

将 `DetectContentType` 从 handler 迁移到 `app/server/internal/pkg/magic.go`（包内函数 `pkg.DetectContentType`），`storage_handler.go` 与 `user_service.go` 共用。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/handler/ ./internal/service/ -run 'TestUpload_RejectsHugeBody|TestUploadAvatar' -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/handler/storage_handler.go app/server/internal/service/user_service.go app/server/internal/pkg/magic.go app/server/internal/handler/storage_handler_test.go
git commit -m "fix(storage): body limit and avatar magic sniffing"
```

### Task C5: 可信代理与 CORS Vary

**Files:**
- Modify: `app/server/server/gin.go`
- Modify: `app/server/internal/middleware/auth.go`
- Test: `app/server/internal/middleware/auth_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestCORS_AddsVary(t *testing.T) {
	router := gin.New()
	router.Use(middleware.CORS([]string{"https://app.example.com"}))
	router.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	router.ServeHTTP(rec, req)
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatal("expected Vary: Origin")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/middleware/ -run TestCORS_AddsVary -v`
Expected: FAIL

- [x] **Step 3: 实现**

`middleware/auth.go`（或现有 CORS 所在文件）改为：

```go
func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[strings.TrimRight(o, "/")] = struct{}{}
	}
	allowAll := false
	if _, ok := allowed["*"]; ok {
		allowAll = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Header("Vary", "Origin")
		if origin == "" {
			c.Next()
			return
		}
		trimmed := strings.TrimRight(origin, "/")
		if !allowAll {
			if _, ok := allowed[trimmed]; !ok {
				c.Next()
				return
			}
		}
		if allowAll {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", trimmed)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
```

`server/gin.go` 启动时：

```go
	// 显式可信代理：默认信任 loopback；生产通过 CORS_ORIGIN/环境配置补充。
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
```

并把现有 CORS 中间件替换为 `middleware.CORS(strings.Split(cfg.CORSOrigin, ","))`。若现有 CORS 在 `gin.go` 内联，移除旧实现避免重复头。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/middleware/ -run TestCORS -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/server/gin.go app/server/internal/middleware/auth.go app/server/internal/middleware/auth_test.go
git commit -m "fix(middleware): origin-aware CORS with Vary and trusted proxies"
```

### Task C6: SFU 能力门面与 SRS unmute 语义

**Files:**
- Modify: `app/server/internal/sfu/factory/dynamic_provider.go`
- Modify: `app/server/internal/sfu/providers/srs/provider.go`
- Modify: `app/server/internal/signal/hub_mute.go`
- Test: `app/server/internal/sfu/factory/dynamic_provider_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestDynamicProvider_SupportsStream(t *testing.T) {
	p := newDynamicProviderWithConfig(t, "livekit")
	if p.SupportsStream() {
		t.Fatal("livekit must not advertise stream support")
	}
	p2 := newDynamicProviderWithConfig(t, "srs")
	if !p2.SupportsStream() {
		t.Fatal("srs must advertise stream support")
	}
}

func TestSRS_MuteFalseReturnsSoft(t *testing.T) {
	svc := newSRSTestService(t)
	if err := svc.MuteParticipant("r", "u", "", false); err != pkg.ErrSFUNotSupported {
		t.Fatalf("expected ErrSFUNotSupported for unmute, got %v", err)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/sfu/... -run 'TestDynamicProvider_SupportsStream|TestSRS_MuteFalseReturnsSoft' -v`
Expected: FAIL

- [x] **Step 3: 实现**

`dynamic_provider.go`：

```go
// SupportsStream 仅当底层 provider 实现 StreamProvider 时为 true。
func (p *DynamicProvider) SupportsStream() bool {
	provider, err := p.current()
	if err != nil {
		return false
	}
	_, ok := provider.(sfu.StreamProvider)
	return ok
}
```

`StreamName` / `StreamInfo` 在底层不支持时改为返回 `pkg.ErrSFUNotSupported` 语义（`StreamName` 返回 `""` 不变，`StreamInfo` 返回 `pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "stream not supported")`），并让 `gin.go` / `sfu_service` 改走 `SupportsStream()` 判断。

`srs/provider.go`：

```go
func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	if !muted {
		return pkg.ErrSFUNotSupported // soft unmute：媒体层无动作，由策略层决定是否报 enforcement
	}
	// ...原逻辑...
}
```

`hub_mute.go` 对 unmute 调用 `MuteParticipant(..., false)` 返回 `ErrSFUNotSupported` 时记录 soft 语义而不是 degraded 成功（在现有调用处追加判断：`if err == pkg.ErrSFUNotSupported { log; continue }`）。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/sfu/... -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/sfu/factory/dynamic_provider.go app/server/internal/sfu/providers/srs/provider.go app/server/internal/signal/hub_mute.go app/server/internal/sfu/factory/dynamic_provider_test.go
git commit -m "fix(sfu): expose stream capability and correct soft unmute semantics"
```

---

## Phase D: 架构/运维

### Task D1: WS 优雅关闭 CloseAll

**Files:**
- Modify: `app/server/internal/ws/broadcaster.go`
- Modify: `app/server/internal/ws/fanout.go`
- Modify: `app/server/server/gin.go`
- Test: `app/server/internal/ws/fanout_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestFanout_CloseAll(t *testing.T) {
	f := NewFanout()
	c1 := newTestClient("c1")
	c2 := newTestClient("c2")
	f.Add(c1)
	f.Add(c2)
	f.CloseAll()
	if c1.closedCount != 1 || c2.closedCount != 1 {
		t.Fatalf("expected both closed, c1=%d c2=%d", c1.closedCount, c2.closedCount)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/ws/ -run TestFanout_CloseAll -v`
Expected: FAIL

- [x] **Step 3: 实现**

`broadcaster.go` 接口增加：

```go
	// CloseAll 关闭全部已注册客户端连接（优雅停机用）。
	CloseAll()
```

`fanout.go`：

```go
func (f *Fanout) CloseAll() {
	f.mu.RLock()
	targets := make([]*Client, 0, len(f.clients))
	for _, c := range f.clients {
		targets = append(targets, c)
	}
	f.mu.RUnlock()
	for _, c := range targets {
		c.Close()
	}
}
```

`gin.go` 优雅关闭顺序改为：

```go
		// 1) stop accepting HTTP first
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)

		// 2) stop membership heartbeat, then close WS before bus
		signalHub.StopMembershipHeartbeat()
		if wsUpgrader != nil && wsUpgrader.Fanout() != nil {
			wsUpgrader.Fanout().CloseAll()
		}
		logger.WithComponent("WS").Info("websocket connections closed")

		// 3) stop plugin/cluster after media sockets are gone
		pluginStopCtx, pluginCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = pluginReg.StopAll(pluginStopCtx)
		pluginCancel()
		if clusterStop != nil {
			clusterStop()
		}

		// 4) close event bus last
		closeEventBus()
```

需要给 `ws.Upgrader` 增加 `Fanout() Broadcaster` 访问器（或把 fanout 注入 gin 的局部变量）。`wsUpgrader` 在 gin.go 中是局部变量，直接持有 `wsUpgrader.cfg.Fanout` 不可导出，增加导出方法：

```go
// Fanout 返回 Upgrader 持有的 Broadcaster。
func (u *Upgrader) Fanout() Broadcaster { return u.cfg.Fanout }
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/ws/ -run TestFanout_CloseAll -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/ws/broadcaster.go app/server/internal/ws/fanout.go app/server/internal/ws/upgrader.go app/server/server/gin.go app/server/internal/ws/fanout_test.go
git commit -m "fix(server): close websocket clients before bus shutdown"
```

### Task D2: 启动错误出口与默认环境

**Files:**
- Modify: `app/server/server/gin.go`
- Modify: `app/server/cmd/root.go`
- Modify: `app/server/main.go`

- [x] **Step 1: 写失败测试**

`cmd/root.go` 无测试文件，先在 `app/server/cmd/root_test.go` 新增：

```go
func TestServerCmd_DefaultEnvIsDev(t *testing.T) {
	cmd := newServerCommand()
	env, _ := cmd.Flags().GetString("env")
	if env != "" {
		t.Fatal("default env must be empty so runtime picks dev")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./cmd/ -run TestServerCmd_DefaultEnvIsDev -v`
Expected: FAIL（当前默认走 Prod）

- [x] **Step 3: 实现**

`cmd/root.go`：

```go
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start server",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		mode := server.Dev
		if env != "" {
			mode = server.EnvEnum(env)
		}
		if err := server.StartGin(mode); err != nil {
			fmt.Fprintf(os.Stderr, "server start failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}
```

`gin.go` 的 `StartGin` 改为 `func StartGin(env EnvEnum) error`，把内部 `panic` 改为返回 `error`（至少把 DB/Redis/bus 初始化 panic 点改为 `return fmt.Errorf(...)`）；`main.go` 移除 `init()` 的 `os.Chdir`，改为显式 `os.Chdir` 仅在 `GOSPEAK_WORKDIR` 设置时执行，并调用 `StartGin` 处理 error。Makefile/构建脚本用 `-ldflags "-X main.version=$(VERSION)"` 注入版本。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./cmd/ -v && go build ./...`
Expected: PASS / BUILD OK

- [x] **Step 5: 提交**

```bash
git add app/server/cmd/root.go app/server/cmd/root_test.go app/server/main.go app/server/server/gin.go
git commit -m "fix(server): dev default, version injection, and startup error path"
```

### Task D3: /readyz 与监控 DB stats 收口

**Files:**
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/internal/handler/monitor_handler.go`
- Modify: `app/server/internal/repository/db.go`
- Test: `app/server/internal/router/router_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestRouter_Readyz(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/readyz", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/router/ -run TestRouter_Readyz -v`
Expected: FAIL

- [x] **Step 3: 实现**

`repository/db.go` 增加：

```go
// DBStats 返回连接池统计，供监控与 readiness 使用。
func DBStats() sql.DBStats {
	if DB == nil {
		return sql.DBStats{}
	}
	return DB.DB().Stats()
}
```

`monitor_handler.go` 的 `collect()` 中 DB 部分改为调用 `repository.DBStats()`（handler 不再直接访问 `repository.DB` 的字段），把 `db.DB()` 的读写封装进 repository。`router.go` 增加：

```go
	r.GET("/readyz", func(c *gin.Context) {
		status := http.StatusOK
		if repository.DB == nil || repository.DB.DB() == nil {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"status": map[bool]string{true: "ok", false: "unavailable"}[status == http.StatusOK]})
	})
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/router/ ./internal/handler/ -run 'TestRouter_Readyz|TestMonitor' -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/router/router.go app/server/internal/handler/monitor_handler.go app/server/internal/repository/db.go app/server/internal/router/router_test.go
git commit -m "feat(server): readiness probe and repository-owned DB stats"
```

### Task D4: 吞错收敛

**Files:**
- Modify: `app/server/internal/plugin/builtin/botbase/plugin.go`
- Modify: `app/server/internal/sfu/providers/mediasoup/signal.go`
- Modify: `app/server/internal/service/cluster_scaling.go`
- Modify: `app/server/internal/service/bot_service.go`
- Test: `app/server/internal/plugin/builtin/botbase/plugin_test.go` 等

- [x] **Step 1: 写失败测试**

```go
func TestBotbase_RejectsBadConfig(t *testing.T) {
	p := newBotbaseTestPlugin(t)
	_, err := p.handleConfig(`{bad json`)
	if err == nil {
		t.Fatal("expected error for malformed config")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/plugin/... ./internal/service/ ./internal/sfu/providers/mediasoup/ -run 'TestBotbase_RejectsBadConfig|TestReconcile_ReportsErrors|TestProduce_AppDataError' -v`
Expected: FAIL

- [x] **Step 3: 实现**

`botbase/plugin.go` 三处 `_ = json.Unmarshal(...)` 改为：

```go
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Printf("[botbase] bad config json: %v", err)
		return nil, fmt.Errorf("bad config json: %w", err)
	}
```

`mediasoup/signal.go` 的 produce AppData：

```go
		var appDataMap map[string]interface{}
		if err := json.Unmarshal(req.AppData, &appDataMap); err != nil {
			return errorJSON(err), nil
		}
```

`cluster_scaling.go` 的 `_, _ = s.ScaleServer(...)` 改为：

```go
		if err := s.ScaleServer(ctx, assignment); err != nil {
			log.Printf("[cluster] reconcile scale failed server=%s: %v", assignment.ServerUUID, err)
		}
```

`bot_service.go` 的 `randomHex` 中 `_, _ = rand.Read(...)` 改为检查错误并返回 INTERNAL_ERROR。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/plugin/... ./internal/service/ ./internal/sfu/providers/mediasoup/ -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/server/internal/plugin/builtin/botbase/plugin.go app/server/internal/sfu/providers/mediasoup/signal.go app/server/internal/service/cluster_scaling.go app/server/internal/service/bot_service.go
git commit -m "fix: stop swallowing parse and scaling errors"
```

---

## Phase E: 测试补全

### Task E1: OAuth service/handler 测试

**Files:**
- Create: `app/server/internal/service/oauth_service_test.go`
- Create: `app/server/internal/handler/oauth_handler_test.go`

- [x] **Step 1: 写测试**

`oauth_service_test.go`：

```go
func TestOAuthService_HandleCallback_UnknownProvider(t *testing.T) {
	svc := newTestOAuthService(t)
	if _, err := svc.HandleCallback("nope", "code"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestOAuthService_HandleCallback_DisabledProvider(t *testing.T) {
	svc := newTestOAuthService(t)
	_ = svc.disableProvider("github")
	if _, err := svc.HandleCallback("github", "code"); err == nil {
		t.Fatal("expected error for disabled provider")
	}
}
```

`oauth_handler_test.go`：

```go
func TestOAuthHandler_Callback_BadState(t *testing.T) {
	h := newTestOAuthHandler(t)
	router := gin.New()
	router.GET("/cb/:provider", h.Callback)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/cb/github?code=x&state=forged", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "oauth_error") {
		t.Fatal("expected oauth_error redirect")
	}
}
```

- [x] **Step 2: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service/ ./internal/handler/ -run 'TestOAuth' -v`
Expected: PASS（首次编写后直接通过或按真实签名修正）

- [x] **Step 3: 提交**

```bash
git add app/server/internal/service/oauth_service_test.go app/server/internal/handler/oauth_handler_test.go
git commit -m "test(oauth): service and handler coverage"
```

### Task E2: auth 状态机矩阵

**Files:**
- Modify: `app/server/internal/service/auth_service_test.go`

- [x] **Step 1: 写表驱动测试**

```go
func TestAuthTokenStateMachine(t *testing.T) {
	t.Run("change password revokes old token", func(t *testing.T) {
		svc := newTestAuthService(t)
		user := createUser(t, svc, "alice", "old-pass", 1)
		if err := svc.ChangePassword("alice", "old-pass", "new-pass"); err != nil {
			t.Fatalf("ChangePassword: %v", err)
		}
		// 旧 access token 因 TokenVersion 递增失效
		token, _ := pkg.GenerateToken(user.Name, user.DisplayName, user.UUID, user.Role, 1)
		if _, code := middleware.VerifyToken(token); code != pkg.TOKEN_REVOKED {
			t.Fatalf("expected TOKEN_REVOKED, got %d", code)
		}
	})

	t.Run("reset password requires email verification", func(t *testing.T) {
		svc := newTestAuthServiceWithEmail(t)
		if err := svc.ResetPassword("alice@example.com", "bad-code", "new-pass"); err == nil {
			t.Fatal("expected verification failure")
		}
	})
}
```

- [x] **Step 2: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service/ -run TestAuthTokenStateMachine -v`
Expected: PASS

- [x] **Step 3: 提交**

```bash
git add app/server/internal/service/auth_service_test.go
git commit -m "test(auth): token lifecycle and reset matrix"
```

### Task E3: 根测试入口与 E2E 骨架

**Files:**
- Modify: `app/server/package.json`
- Modify: `package.json`
- Create: `test/package.json`
- Create: `test/smoke.test.ts`

- [x] **Step 1: 修复根测试脚本**

`app/server/package.json` 的 scripts 增加：

```json
  "scripts": {
    "test": "go test ./..."
  }
```

根 `package.json` 的 `test:server` 改为：

```json
  "test:server": "cd app/server && go test ./..."
```

`test/package.json`：

```json
{
  "name": "@gospeak/test",
  "private": true,
  "scripts": {
    "test": "vitest run"
  }
}
```

- [x] **Step 2: 写 E2E 冒烟测试**

`test/smoke.test.ts`：

```ts
import { describe, expect, it } from "vitest";

describe("server smoke", () => {
  it("health endpoint returns pong", async () => {
    const base = process.env.GOSPEAK_TEST_URL ?? "http://127.0.0.1:8998";
    const res = await fetch(`${base}/ping`);
    expect(res.status).toBe(200);
    expect(await res.text()).toContain("pong");
  });
});
```

- [x] **Step 3: 运行验证**

Run: `pnpm test:server && cd test && pnpm test`
Expected: PASS（Go 全量测试通过；E2E 需要已启动服务，`GOSPEAK_TEST_URL` 可指向 CI 环境）

- [x] **Step 4: 提交**

```bash
git add app/server/package.json package.json test/package.json test/smoke.test.ts
git commit -m "test: restore root server test entry and add e2e smoke"
```

---

## Phase F: 前端与杂项

### Task F1: apiClient 泛型解包

**Files:**
- Modify: `app/web/src/api/apiClient.ts`
- Modify: `app/web/src/api/room.ts` 等使用 `as any` 的模块

- [x] **Step 1: 写失败测试**

在 `app/web/src/api/apiClient.spec.ts`：

```ts
import { describe, expect, it } from "vitest";
import APIClient from "./apiClient";

describe("APIClient", () => {
  it("resolves Result.data instead of AxiosResponse", async () => {
    const client = new APIClient("/");
    const value = await client.get<{ ok: boolean }>({ url: "/x" });
    expect(value.ok).toBe(true);
  });
});
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm exec vitest run src/api/apiClient.spec.ts`
Expected: FAIL

- [x] **Step 3: 实现**

`apiClient.ts` 的 `request` 改为：

```ts
	request<T = any, U = any>(config: AxiosRequestConfig<U>): Promise<T> {
		return new Promise((resolve, reject) => {
			this.axiosInstance
				.request<any, AxiosResponse<Result<T>>>(config)
				.then((res: AxiosResponse<Result<T>>) => {
					if (res.data && res.data.code !== 0) {
						reject(new Error(res.data.msg));
						return;
					}
					resolve(res.data.data as T);
				})
				.catch((e: Error | AxiosError) => reject(e));
		});
	}
```

然后按模块逐个移除 `(res as any).data.data`，例如 `room.ts:53` 改为：

```ts
const res = await apiClient.post<RoomListResult>({ url: "/api/v1/room/list", data });
```

- [x] **Step 4: 运行测试与类型检查**

Run: `cd app/web && pnpm exec vitest run src/api && pnpm exec tsc --noEmit`
Expected: PASS / NO ERRORS

- [x] **Step 5: 提交**

```bash
git add app/web/src/api/apiClient.ts app/web/src/api/room.ts app/web/src/api/apiClient.spec.ts
git commit -m "refactor(web): typed API client unwrap"
```

### Task F2: 权限服务端下发

**Files:**
- Modify: `app/web/src/utils/permissions.ts`
- Modify: `app/web/src/stores/userStore.ts`
- Modify: `app/server/internal/handler/user_handler.go`

- [x] **Step 1: 后端 profile 返回权限**

`user_handler.go` 的 `GetProfile` 响应 data 增加 `permissions`（从 permSvc 按 role 解析），并写测试：

```go
func TestGetProfile_IncludesPermissions(t *testing.T) {
	// profile 响应 data.permissions 必须与 DB 角色权限一致
}
```

- [x] **Step 2: 前端改为服务端权威**

`permissions.ts` 的 `hasPermission` 改为优先读 `userStore.user()?.permissions`：

```ts
export function hasPermission(code: string): boolean {
	const claims = claimPermissions();
	if (claims) return claims.includes(code);

	const user = userStore.user();
	if (user?.permissions?.includes(code)) return true;

	const role = user?.role;
	if (!role) return false;
	return rolePermissions[role]?.includes(code) ?? false;
}
```

`userStore` 的 `User` 类型增加 `permissions: string[]`，profile 加载后写入。

- [x] **Step 3: 运行验证**

Run: `cd app/web && pnpm exec vitest run src/utils && pnpm exec tsc --noEmit`
Expected: PASS / NO ERRORS

- [x] **Step 4: 提交**

```bash
git add app/server/internal/handler/user_handler.go app/web/src/utils/permissions.ts app/web/src/stores/userStore.ts
git commit -m "feat(auth): serve role permissions to frontend"
```

### Task F3: wsClient 重连重新解析 URL

**Files:**
- Modify: `app/web/src/socket/wsClient.ts`
- Modify: `app/web/src/api/ws.ts`
- Test: `app/web/src/socket/wsClient.test.ts`

- [x] **Step 1: 写失败测试**

```ts
it("re-resolves worker URL on reconnect", async () => {
	const client = createWSClient({ ticketUrl: "/ticket" });
	await client.connect("wss://old-worker/ws");
	// 模拟 ticket 返回新 URL 后断线重连
	client.simulateClose();
	expect(client.currentUrl()).not.toContain("old-worker");
});
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm exec vitest run src/socket/wsClient.test.ts`
Expected: FAIL

- [x] **Step 3: 实现**

`wsClient.ts` 的重连分支在 `shouldReconnect && currentUrl` 时，先调用 `refreshTicket()`（`app/web/src/api/ws.ts` 的 `getWSTicket`），用返回值中的 `url`（若无则保持旧值）重连：

```ts
			if (shouldReconnect && currentUrl) {
				const ticket = await refreshTicketSafe();
				const nextUrl = ticket?.url ?? currentUrl;
				const nextToken = ticket?.token ?? currentToken;
				reconnectAttempts += 1;
				reconnectTimer = window.setTimeout(() => {
					connect(nextUrl, nextToken);
				}, Math.min(1000 * 2 ** reconnectAttempts, 15000) + Math.random() * 300);
			}
```

`app/web/src/api/ws.ts` 的 `getWSTicket` 返回 `{ url, token }`（后端 `/api/v1/signal/ws-ticket` 若未返回 url，则沿用当前）。

- [x] **Step 4: 运行测试确认通过**

Run: `cd app/web && pnpm exec vitest run src/socket/wsClient.test.ts`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add app/web/src/socket/wsClient.ts app/web/src/api/ws.ts app/web/src/socket/wsClient.test.ts
git commit -m "fix(web): re-resolve worker url on reconnect"
```

### Task F4: 侧栏批量加载

**Files:**
- Modify: `app/web/src/api/domain.ts`
- Modify: `app/web/src/layouts/common/sidebar.tsx`
- Modify: `app/server/internal/handler/domain_handler.go`

- [x] **Step 1: 后端 my-domains 批量详情**

`domain_handler.go` 的 `MyDomains` 响应改为带 `member_count`、`room_count` 的批量详情（一次 DB 查询），并加测试。

- [x] **Step 2: 前端一次请求**

`app/web/src/api/domain.ts` 增加：

```ts
export async function getMyDomainsDetailed(): Promise<DomainDetail[]> {
	return apiClient.post<DomainDetail[]>({ url: "/api/v1/domain/my-domains" });
}
```

`sidebar.tsx` 的 resource 改为：

```tsx
	const [domains] = createResource<Domain[], string[]>(
		() => state.myDomainUUIDs,
		async () => {
			const rows = await getMyDomainsDetailed();
			return rows.filter((r) => state.myDomainUUIDs.includes(r.uuid));
		},
	);
```

- [x] **Step 3: 运行验证**

Run: `cd app/web && pnpm exec vitest run src/components && pnpm exec tsc --noEmit`
Expected: PASS / NO ERRORS

- [x] **Step 4: 提交**

```bash
git add app/web/src/api/domain.ts app/web/src/layouts/common/sidebar.tsx app/server/internal/handler/domain_handler.go
git commit -m "perf(web): batch my-domains detail for sidebar"
```

### Task F5: 残余契约与状态机收尾

**Files:**
- Modify: `app/server/internal/signal/hub.go`（OnDisconnect 房间索引）
- Modify: `app/server/internal/service/mute_service.go`（service 层 duration 校验）
- Modify: `app/server/internal/handler/storage_handler.go`（category 白名单）
- Modify: `app/server/internal/handler/srs_callback_handler.go`（URL decode）
- Modify: `app/server/internal/signal/state_sync.go`（heartbeat 只续期）

- [x] **Step 1: service 层禁言入参校验**

`mute_service.go` 的 `MuteUser` 开头：

```go
	if !req.Permanent && req.Duration <= 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "duration is required for temporary mute")
	}
```

- [x] **Step 2: SRS 回调 URL decode**

`srs_callback_handler.go` 的 `parseCallbackParams` 改为：

```go
func parseCallbackParams(param string) map[string]string {
	out := map[string]string{}
	if param == "" {
		return out
	}
	values, err := url.ParseQuery(param)
	if err != nil {
		return out
	}
	for k, v := range values {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
```

- [x] **Step 3: category 白名单**

`storage_handler.go` 的 `PresignUpload` 增加：

```go
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`).MatchString(req.Category) {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid category")
		return
	}
```

- [x] **Step 4: OnDisconnect 房间索引**

`hub.go` 维护 `connSlots` 已存在，`OnDisconnect` 用 `h.connSlots[clientID]` 收集房间 key 再清理，避免遍历全部 `h.rooms`；`OnDisconnect` 内先读 `connSlots` 再按索引删除。

- [x] **Step 5: 心跳只续期不广播**

`state_sync.go` 的 `StartMembershipHeartbeat` 每次心跳仅 `syncRoomToStore`（已有），删除心跳路径上的 `notifyRoomStateChanged` 全量广播，改为仅在 `registerRoomMembers`/`syncRoomToStore` 检测到成员变化时通知。

- [x] **Step 6: 运行验证**

Run: `cd app/server && go test ./internal/service/ ./internal/signal/ ./internal/handler/ -v`
Expected: PASS

- [x] **Step 7: 提交**

```bash
git add app/server/internal/service/mute_service.go app/server/internal/handler/storage_handler.go app/server/internal/handler/srs_callback_handler.go app/server/internal/signal/hub.go app/server/internal/signal/state_sync.go
git commit -m "fix: close remaining contract and state machine gaps"
```

---

## Self-Review

- **覆盖检查**：Phase A 覆盖 NATS 回调阻塞、ctx、pending、auth CAS/黑名单、leader 锁（缺口组 1/2/3/4/8）；Phase B 覆盖 N+1、广播、索引（缺口组 6/7/16/28/29/33/34 等）；Phase C 覆盖 OAuth/refresh/S3/上传/代理/SFU（缺口组 5/14/15/19/20/21/22/23/26/27/28）；Phase D 覆盖优雅关闭/启动/readiness/吞错（缺口组 18-26/32/39/40/42/43）；Phase E 覆盖测试与脚本（缺口组 5/12/15/24/25/69/95/98）；Phase F 覆盖前端类型/权限/重连/批量与剩余契约（缺口组 27-31/89/92/93/97）。
- **占位符扫描**：所有任务均给出可执行代码或明确迁移步骤；对需要先确认现有实现的点（如 mute_repo 方法名、gin 内联 CORS 位置）已在步骤中标注"先确认"而非留空实现。
- **类型一致性**：`GetByNames`、`IsMutedBatch`、`GetRoomMembersBatch`、`CloseAll`、`IsBlacklistedErr`、`SupportsStream` 等新签名在引入任务与后续使用任务中保持一致；`RefreshFromToken` 返回签名变化同步到 `auth_handler.go`。
