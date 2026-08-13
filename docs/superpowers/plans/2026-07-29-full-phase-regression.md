# 全阶段回归测试计划 — Guild + WebSocket Migration

> **Status (2026-08-13):** ⚠️ 部分落地 — 分层测试矩阵已定义; 全量回归未系统执行

> **适用阶段:** Phase 1 (Guild 多重服务器) + Phase 2 (WS 传输层迁移)
> **测试策略:** 分层覆盖（单元/集成/E2E/性能）+ 跨阶段回归 + 自动化 CI

---

## 一、测试覆盖矩阵

| 层级 | 测试类型 | 工具 | 覆盖范围 | Phase |
|------|----------|------|----------|-------|
| L0 | 编译检查 | `go build ./...`, `npx tsc --noEmit` | 所有 Go/TS 代码无编译错误 | 1+2 |
| L1 | 单元测试 | `go test`, `vitest` | model/repo/service/handler/middleware/ws | 1+2 |
| L2 | 集成测试 | `go test` (signal pkg) | Hub + Fanout + EventBus 联合 | 1+2 |
| L3 | API 测试 | Go HTTP 测试 / Playwright | HTTP API 端点 | 1 |
| L4 | WS 协议测试 | Go WS 测试 / Playwright | WS 升级、握手、消息轮次 | 2 |
| L5 | E2E 全链路 | Playwright + 后端 | 浏览器登录→Guild→房间→WS 语音 | 1+2 |
| L6 | 性能测试 | `pprof` / `wrk` / 自定义 | 连接数/内存/goroutine | 2 |

---

## 二、Phase 1 — Guild 回归测试

### 2.1 单元测试（Go — model / repo / service / handler / middleware）

**测试文件与场景:**

#### `internal/model/guild_test.go`（新增）

| 测试 | 场景 | 预期 |
|------|------|------|
| `TestGuild_BeforeCreate` | 创建 Guild 时自动生成 UUID 和 InviteCode | UUID 非空、InviteCode 8 字符 |
| `TestGuildMember_BeforeCreate` | 创建 GuildMember 时自动生成 ID | ID 自增 |
| `TestGenerateInviteCode` | 生成 8 字符邀请码 | 只含 A-Z,2-9, 长度 8 |
| `TestGuild_TableName` | Guild 表名 | `"guilds"` |
| `TestGuildMember_TableName` | GuildMember 表名 | `"guild_members"` |

#### `internal/repository/guild_repo_test.go`（新增）

| 测试 | 场景 | 关键验证 |
|------|------|----------|
| `TestGuildRepo_Create` | 创建 Guild | DB 行存在 |
| `TestGuildRepo_GetByUUID` | 按 UUID 查找 | 返回正确 Guild |
| `TestGuildRepo_GetByInviteCode` | 按邀请码查找 | 返回正确 Guild |
| `TestGuildRepo_List` | 分页查询 | total + guilds 正确 |
| `TestGuildRepo_ListPublic` | 公开 Guild 过滤 | 只返回 is_public=true |
| `TestGuildRepo_AddMember` | 添加成员 | guild_members 行存在 |
| `TestGuildRepo_RemoveMember` | 移除成员 | 行被删除 |
| `TestGuildRepo_GetMember` | 查成员关系 | 返回正确 role |
| `TestGuildRepo_ListMembers` | 列出 Guild 成员 | 按 joined_at 排序 |
| `TestGuildRepo_ListUserGuilds` | 用户加入的 Guild | 返回 UUID 列表 |
| `TestGuildRepo_CountMembers` | 成员数统计 | 数值正确 |
| `TestGuildRepo_CountRooms` | 房间数统计 | 只计该 Guild |
| `TestGuildRepo_Delete` | 删除 Guild | 无级联删除（仅删 guilds 行） |

#### `internal/service/guild_service_test.go`（新增，mock repo）

| 测试 | 场景 | 关键验证 |
|------|------|----------|
| `TestCreate_EmptyName` | 空名称创建 | 返回 `INVALID_PARAMS` |
| `TestCreate_NameTooLong` | 名称超 100 字符 | 返回 `INVALID_PARAMS` |
| `TestCreate_Success` | 正常创建 + 自动加入 owner | Guild 和 Member 均创建 |
| `TestGetByUUID_NotFound` | UUID 不存在 | 返回 `ErrGuildNotFound` |
| `TestGetByInviteCode_NotFound` | 邀请码不存在 | 返回 `ErrGuildNotFound` |
| `TestJoin_NotExists` | 邀请码无效 | 返回 `ErrGuildNotFound` |
| `TestJoin_AlreadyMember` | 重复加入 | 返回 `ErrAlreadyMember` |
| `TestJoin_Success` | 正常加入 | RoleName 为 "member" |
| `TestLeave_OwnerCannotLeave` | owner 离开 | 返回 `FORBIDDEN` |
| `TestLeave_Success` | 成员正常离开 | 成员关系删除 |
| `TestKick_Owner` | 踢 owner | 返回 `FORBIDDEN` |
| `TestKick_Success` | 踢普通成员 | 成员关系删除 |
| `TestTransferOwnership_NotOwner` | 非 owner 转让 | 返回 `FORBIDDEN` |
| `TestTransferOwnership_Success` | 正常转让 | OwnerUUID 交换、RoleName 交换 |
| `TestHasGuildRole` | 角色层级 | owner≥admin≥member≥guest |
| `TestCheckRoomLimit_NoLimit` | MaxRooms=0 | 不限制 |
| `TestCheckRoomLimit_Reached` | 房间数已达上限 | 返回 `ErrGuildRoomLimit` |

#### `internal/handler/guild_handler_test.go`（新增，HTTP 测试）

| 测试 | 场景 | 请求体 | 预期 code |
|------|------|--------|-----------|
| `TestGuildCreate_InvalidJSON` | JSON 解析失败 | 非法 JSON | 2001 |
| `TestGuildCreate_MissingName` | name 为空 | `{}` | 2001 |
| `TestGuildCreate_Success` | 正常创建 | `{"name":"Test"}` | 0 |
| `TestGuildGet_NotFound` | UUID 不存在 | `{"uuid":"fake"}` | 3001 |
| `TestGuildGet_Success` | 正常查询 | 有效 UUID | 0 |
| `TestGuildJoin_InvalidCode` | 邀请码为空 | `{}` | 2001 |
| `TestGuildJoin_NotFound` | 邀请码无效 | `{"invite_code":"XXXX"}` | 3001 |
| `TestGuildLeave_NotOwnerSuccess` | 成员正常离开 | 有效 UUID | 0 |
| `TestGuildKick_NotAdmin` | 普通成员踢人 | — | 1013 |
| `TestGuildDelete_NotOwner` | 非 owner 删除 | — | 1013 |
| `TestGuildDelete_OwnerSuccess` | owner 删除 | — | 0 |
| `TestGuildListPagination` | 分页 | `{"page":1,"page_size":10}` | 0 |

#### `internal/middleware/guild_test.go`（新增）

| 测试 | 场景 | 关键验证 |
|------|------|----------|
| `TestRequireGuildMember_NoUUID` | 请求无 guild_uuid | 返回 2001 |
| `TestRequireGuildMember_NotMember` | 非成员访问 | 返回 1013 |
| `TestRequireGuildMember_Success` | 成员访问 | c.Next() |

### 2.2 集成测试（信号层 Guild 隔离）

#### `internal/signal/hub_guild_test.go`（新增）

| 测试 | 场景 | 关键验证 |
|------|------|----------|
| `TestHub_GuildRoomIsolation` | 两个 Guild 创建同名房间 | roomKey 不同，互不干扰 |
| `TestHub_GuildRoomCreate` | 带 GuildUUID 创建房间 | 使用 roomKey 存储 |
| `TestHub_GuildMemberVisibility` | 成员只看到自己 Guild 的房间 | ListRooms 隔离 |
| `TestHub_PlatformRoomCompat` | 无 GuildUUID 的房间（向后兼容） | 使用 `"platform:"` 前缀 |
| `TestHub_RoomKey_Format` | roomKey 格式 | `guildUUID:roomName` / `platform:roomName` |
| `TestHub_GuildMemberKick` | Guild 内踢人 | 仅踢出该 Guild 房间的成员 |
| `TestHub_GuildRoomBroadcast` | 广播只发到同一 roomKey | 其他 Guild 同名房间不收到 |

### 2.3 数据迁移测试

| 测试 | 场景 | 验证 |
|------|------|------|
| `TestMigrateDefaultGuild_NoExisting` | 无旧数据 | 创建默认 Guild |
| `TestMigrateDefaultGuild_HasRooms` | 有旧 room | 所有 `guild_uuid=""` 的 room 归入默认 Guild |
| `TestMigrateDefaultGuild_AlreadyMigrated` | 已有 Guild | 不重复创建、不覆盖 |

### 2.4 前端测试（Playwright / Vitest）

| 测试 | 文件 | 场景 |
|------|------|------|
| `GuildIcon.spec.ts` | 组件测试 | 渲染首字母、传入 icon_url 渲染 img、active 状态样式 |
| `GuildList.spec.ts` | 组件测试 | 加载我的 Guild 列表、选中 Guild |
| `CreateGuildModal.spec.ts` | 组件测试 | 表单校验、API 调用、成功回调 |
| `guildStore.spec.ts` | Store 测试 | loadMyGuilds、setCurrentGuild、ensureGuildLoaded |
| `guildApi.spec.ts` | API 测试 | create/get/join/leave/kick 调用参数正确 |

---

## 三、Phase 2 — WS 迁移回归测试

### 3.1 ws 包单元测试

#### `internal/ws/client_test.go`（新增）

| 测试 | 场景 | 关键验证 |
|------|------|----------|
| `TestNewClient` | 创建 Client | ID/Claims 正确 |
| `TestClient_Send` | 发送消息 | 写入 writeCh |
| `TestClient_SendACK` | 发送 ACK | ACK 格式正确 |
| `TestClient_SendErrorACK` | 发送错误 ACK | ACKError 格式正确 |
| `TestClient_WriteLoop_Drain` | 写循环消费 writeCh | 直到关闭 |
| `TestClient_StartReadLoop_ValidMsg` | 读取有效消息 | handler 被调用 |
| `TestClient_StartReadLoop_InvalidJSON` | 读取非法 JSON | 跳过，不 panic |
| `TestClient_StartReadLoop_EmptyEvent` | event 为空 | 跳过 |
| `TestClient_SendAfterClose` | 关闭后发送 | 返回 false |
| `TestClient_Close` | 关闭连接 | 关闭正常 |
| `TestClientConcurrentSend` | 并发写入 | 无 data race |

#### `internal/ws/fanout_test.go`（新增）

| 测试 | 场景 |
|------|------|
| `TestFanout_Add_Remove` | 注册/注销客户端 |
| `TestFanout_Join_Leave` | 加入/离开房间 |
| `TestFanout_BroadcastToRoom` | 房间广播（只发给该房间成员） |
| `TestFanout_BroadcastToNamespace` | 全局广播（发给所有连接） |
| `TestFanout_ForEach` | 遍历房间成员，fn 返回 false 停止 |
| `TestFanout_RoomCount` | 房间成员计数 |
| `TestFanout_RoomExists` | 空房间返回 false |
| `TestFanout_Remove_CleansRooms` | 移除 client 自动清理房间 |
| `TestFanout_EmptyRoom_AutoDelete` | 最后成员离开时房间 key 被删除 |
| `TestFanout_ConcurrentAccess` | 并发读写无 race |

#### `internal/ws/handler_test.go`（新增）

| 测试 | 场景 |
|------|------|
| `TestHandlerRegistry_Dispatch_NoAck` | 无应答事件分发 |
| `TestHandlerRegistry_Dispatch_Ack` | 有应答事件分发（client 收到 ACK） |
| `TestHandlerRegistry_Dispatch_UnknownEvent` | 未知事件（静默忽略） |
| `TestHandlerRegistry_Dispatch_PanicRecover` | handler panic 时 recover + 发错误 ACK |
| `TestHandlerRegistry_Dispatch_NullData` | data=null 转空字符串 |

#### `internal/ws/upgrader_test.go`（新增）

| 测试 | 场景 |
|------|------|
| `TestExtractToken_Header` | Authorization header |
| `TestExtractToken_Cookie` | gospeak_token cookie |
| `TestExtractToken_Query` | ?token=xxx |
| `TestExtractToken_Empty` | 无 token |
| `TestUpgrader_JWTRejected` | JWT 无效 → 401 |
| `TestUpgrader_Success` | 正常升级 → client 注册到 fanout |

### 3.2 协议一致性测试（新文件：`internal/ws/protocol_test.go`）

WS 线格式的黑盒测试，使用 nhooyr 实际连接 Hub：

| 测试 | 场景 | 验证 |
|------|------|------|
| `TestProtocol_RoomCreate` | `{"id":"1","event":"room:create","data":{...}}` | 收到 `{"id":"1","event":"room:created",...}` |
| `TestProtocol_RoomJoin` | 同上 | ACK 带 members 列表 |
| `TestProtocol_RoomLeave` | 同上 | ACK 确认 |
| `TestProtocol_RoomList` | `{"event":"room:list"}` | 收到 `{"event":"room:list:result",...}` |
| `TestProtocol_Kick` | 同上 | 被踢方收到 `room:kicked` |
| `TestProtocol_MessageSend` | 同上 | ACK + 房间广播 |
| `TestProtocol_NoACK_OnPush` | 推送事件无 id | 服务器不返回 ACK |
| `TestProtocol_ACKError` | 非法操作 | 收到 `{"id":"...","event":"...","error":{...}}` |
| `TestProtocol_BotCommand` | bot:command | Hub 发布到 NATS |
| `TestProtocol_MemberSpeaking` | member:speaking | 房间广播 |

### 3.3 集成测试（信号层 + Fanout + EventBus）

#### `internal/signal/hub_ws_test.go`（新增）

替换现有的 `hub_test.go` 中 mockServer 为 mockBroadcaster：

| 测试 | 场景 | 关键验证 |
|------|------|----------|
| `TestHub_WithBroadcaster_RoomCreate` | Hub + Fanout 联合 | fanout.BroadcastToRoom 被调用 |
| `TestHub_WithBroadcaster_RoomJoin` | 同上 | fanout.Join 被调用 + MemberJoined 广播 |
| `TestHub_WithBroadcaster_ClientDisconnect` | OnClientDisconnect | fanout.Remove + 房间清理 |
| `TestHub_WithBroadcaster_NamespaceBroadcast` | 全局广播 | fanout.BroadcastToNamespace |
| `TestHub_EventBus_WithFanout` | NATS→Fanout 事件流 | 远程事件投递到本地 fanout |
| `TestHub_Fanout_ACL_Isolation` | Fanout 房间隔离 | 两个 Guild 同名房间不串 |

### 3.4 性能回归基线

| 指标 | 当前（socket.io） | 目标（nhooyr） | 测试方法 |
|------|-------------------|----------------|----------|
| 单连接内存 | ~200KB | < 50KB | `go test -bench=BenchmarkClientMemory` |
| 1000 连接 goroutine 数 | ~4000+ | ~2000+ | `pprof goroutine` profile |
| 房间广播延迟 P99 | baseline | 不退化 | `BenchmarkBroadcastToRoom` |
| 最大并发连接 | baseline | 2x+ | wrk/scalability |
| JSON 解析吞吐 | baseline | 持平或更好 | `BenchmarkMessageDispatch` |

```go
// bench/ws_bench_test.go（新增）
func BenchmarkBroadcastToRoom(b *testing.B) {
	fanout := ws.NewFanout()
	// 添加 N 个 client 到 room
	for i := 0; i < 100; i++ {
		// ... setup
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fanout.BroadcastToRoom("testroom", "event:test", map[string]string{"data": "payload"})
	}
}
```

---

## 四、跨阶段回归场景（Phase 1 + 2 联合）

这些测试覆盖两个阶段组合后的全链路行为，是最高优先级的回归测试。

### 4.1 全信号流：Guild → WS

| # | 场景 | 步骤 | 预期 |
|---|------|------|------|
| 1 | 用户在 Guild A 创建房间`"lobby"` | WS 连接 Guild A，发送 `room:create`（含 guild_uuid） | 房间键为 `guildA_uuid:lobby`，fanout 中 room 正确 |
| 2 | 同名房间在不同 Guild 不冲突 | 用户 B 在 Guild B 创建 `"lobby"` | 房间键为 `guildB_uuid:lobby`，独立 |
| 3 | 平台级房间兼容 | 无 guild_uuid 的 `room:create` | 键为 `platform:roomName` |
| 4 | Guild 内房间列表通过 WS | `room:list`（含 guild_uuid 过滤） | 只返回该 Guild 的房间 |
| 5 | 踢人跨 Guild 隔离 | Guild A admin 踢 Guild B 成员 | 只踢 Guild A，B 不受影响 |
| 6 | 禁言跨 Guild 隔离 | 用户跨多个 Guild 的房间时禁言 | 当前禁言作用域（全局/Guild 级）正确 |

### 4.2 全链路状态一致性

| # | 场景 | 验证 |
|---|------|------|
| 7 | WS 断连 → Hub.OnClientDisconnect → Hub.rooms 清理 → Fanout 清理 | 三者状态一致 |
| 8 | Guild 被删除 → 所有关联 room 标记清理 → WS 房间广播关闭 | 无残留连接 |
| 9 | Guild member 被踢 → OnClientDisconnect → fanout.Leave → Hub.room 成员移除 | 被踢用户收不到后续广播 |
| 10 | TransferOwnership → JWT 不失效（权限校验在请求时查 GuildMember role） | 新 owner 即时生效 |

### 4.3 向后兼容回归

| # | 场景 | 验证 |
|---|------|------|
| 11 | HTTP API（Guild CRUD）与 WS 共存 | 所有 `/api/v1/guild/*` 路由正常 |
| 12 | 旧前端用 socket.io client 连接 → 返回 404（路由已删） | 友好错误（可加中间件提示） |
| 13 | DB 迁移前后数据一致性 | 升级后房间仍在、Guild 归属正确 |

### 4.4 安全回归

| # | 场景 | 验证 |
|---|------|------|
| 14 | WS 连接无 JWT → 拒绝升级 | 401 |
| 15 | WS 连接过期 JWT → 拒绝 | 401 |
| 16 | 非 Guild 成员通过 WS 加入 Guild 房间 → 拒绝 | 收到错误 ACK |
| 17 | Guild owner/admin 不能通过 WS 伪造身份 | handler 从 `c.Claims()` 取 identity |

---

## 五、CI 自动化建议

### 5.1 GitHub Actions 工作流

```yaml
# .github/workflows/guild-ws-regression.yml
name: Guild + WS Regression
on: [push, pull_request]

jobs:
  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: cd app/server && go test ./internal/model/... -v -count=1 -short
      - run: cd app/server && go test ./internal/repository/... -v -count=1 -short
      - run: cd app/server && go test ./internal/service/... -v -count=1 -short
      - run: cd app/server && go test ./internal/handler/... -v -count=1 -short

  ws-package-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: cd app/server && go test ./internal/ws/... -v -count=1 -race

  integration-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - uses: actions/setup-node@v4
        with: { node-version: '22' }
      - run: cd app/server && go test ./internal/signal/... -v -count=1 -race -timeout 180s

  full-build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: cd app/server && go build ./...

  frontend-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
      - run: cd app/web && pnpm install && pnpm tsc --noEmit

  perf-baseline:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: cd app/server && go test -bench=. -benchmem ./internal/ws/...
      - run: cd app/server && go test -bench=. -benchmem ./internal/signal/...
```

### 5.2 测试通过门槛

| 门槛 | 要求 | 阻断？ |
|------|------|--------|
| 全量编译 | Go + TS 零错误 | 是 |
| L1 单元测试 | 100% 通过，新代码覆盖率 > 70% | 是 |
| L2 集成测试 | 100% 通过 | 是 |
| L3-L4 协议测试 | 100% 通过 | 是 |
| L5 E2E | 核心流程（创建 Guild → 连接 WS → 加入房间 → 语音）通过 | 是 |
| L6 性能 | 不退化超过 10%（CI 对比基线） | 警告 |
| Data Race | `go test -race` 全零 | 是 |

---

## 六、增量回归策略（按阶段执行）

### Phase 1 执行期间

```
每日 CI:
├── 编译: Go build ✅
├── 单元: model/repo/service/handler/middleware ✅
├── 集成: signal (Guild 隔离测试) ✅
└── 前端: tsc --noEmit ✅

合并前必须:
├── 新增 guild_repo_test.go 全部通过
├── 新增 guild_service_test.go 全部通过
├── 新增 guild_handler_test.go 全部通过
└── hub_guild_test.go 全部通过
```

### Phase 2 执行期间

```
每日 CI:
├── 编译: Go build (无 socket.io 引用) ✅
├── ws 包: client/fanout/handler/upgrader 单元 ✅
├── 协议: protocol_test.go 全部通过 ✅
└── 信号: hub_ws_test.go 全部通过 ✅

合并前必须:
├── 旧 signal 测试全部迁移通过
├── 新增 ws 测试覆盖率 > 80%
├── benchmark 性能基线录制
└── -race 测试零 data race
```

### 跨阶段回归（Phase 2 合并后）

```
全量回归触发条件:
├── Phase 2 首个 PR 合并到 main
├── 之后每次合并前均执行

全量回归包含:
├── Phase 1 全部测试
├── Phase 2 全部测试
├── 跨阶段集成测试 (hub_guild_ws_test.go)
└── 性能基线对比 Phase 1 vs Phase 2
```

---

## 七、测试数据与隔离

### 数据库隔离

- 单元测试: SQLite `:memory:`，每个测试独立 DB
- 集成测试: SQLite `:memory:`，共享 DB 但表级隔离（测试间 `TRUNCATE`）
- E2E 测试: Docker Compose SQLite 实例，独立 `test.db`

### WS 测试基础设施

```go
// ws_testutil.go（新增，ws 包测试辅助）
// 提供 in-memory nhooyr conn pair 用于测试，不依赖真实 TCP

func Pipe(ctx context.Context) (*nhooyrws.Conn, *nhooyrws.Conn) {
	return nhooyrws.NetConn(context.Background(), nhooyrws.Options{})
}
```

### 模拟用户工厂

```go
// testutil/user_factory.go（新增）
func CreateTestUser(db *gorm.DB, name string) *model.User { ... }
func CreateTestGuild(db *gorm.DB, name, ownerUUID string) *model.Guild { ... }
func AddTestGuildMember(db *gorm.DB, guildUUID, userUUID, role string) { ... }
```

---

## 八、测试文件清单总结

### Phase 1 新增测试文件

| 文件 | 测试用例数（预估） |
|------|-------------------|
| `app/server/internal/model/guild_test.go` | 5 |
| `app/server/internal/repository/guild_repo_test.go` | 12 |
| `app/server/internal/service/guild_service_test.go` | 16 |
| `app/server/internal/handler/guild_handler_test.go` | 10 |
| `app/server/internal/middleware/guild_test.go` | 3 |
| `app/server/internal/signal/hub_guild_test.go` | 7 |
| `app/web/src/components/guild/GuildIcon.spec.tsx` | 3 |
| `app/web/src/components/guild/GuildList.spec.tsx` | 2 |
| `app/web/src/stores/guildStore.spec.ts` | 4 |
| **小计** | **62** |

### Phase 2 新增测试文件

| 文件 | 测试用例数（预估） |
|------|-------------------|
| `app/server/internal/ws/client_test.go` | 9 |
| `app/server/internal/ws/fanout_test.go` | 10 |
| `app/server/internal/ws/handler_test.go` | 5 |
| `app/server/internal/ws/upgrader_test.go` | 5 |
| `app/server/internal/ws/protocol_test.go` | 9 |
| `app/server/internal/ws/ws_testutil.go` | (辅助) |
| `app/server/internal/signal/hub_ws_test.go` | 6 |
| `app/server/bench/ws_bench_test.go` | 3 |
| **小计** | **47** |

### 跨阶段新增文件

| 文件 | 测试用例数（预估） |
|------|-------------------|
| `app/server/internal/signal/hub_guild_ws_test.go` | 8（全链路联合测试） |

### 修改的测试文件

| 文件 | 范围 |
|------|------|
| `app/server/internal/signal/hub_test.go` | mockConn → mockClientMessenger |
| `app/server/internal/signal/hub_kick_test.go` | 同上 |
| `app/server/internal/signal/hub_integration_test.go` | 同上 |
| `app/server/internal/signal/hub_event_bus_test.go` | 同上 |
| `app/server/internal/signal/bot_bridge_test.go` | 同上 |
| `app/server/internal/signal/message_bridge_test.go` | 同上 |

### 总计

| 维度 | 数量 |
|------|------|
| 新增测试文件 | ~15 |
| 修改测试文件 | ~6 |
| 新增测试用例 | ~117 |
| 测试辅助文件 | ~3 |
