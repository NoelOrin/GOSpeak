# Code Audit — 2026-07-07

Scope: `feature/srs-sfu` branch (49 commits ahead of main)。review 最近 5 commit + 全量冗余扫描。

---

## 1. Critical Bugs (Fixed)

### 1.1 recentClose 内存泄漏
- **文件**: `app/server/internal/mediasoup/signal.go`
- **问题**: `sync.Map` key 永不清除。identity produce 后永久离开或只 consume 不 produce，key 残留。
- **修复**: `OnParticipantLeft` 首次存储 `*dedupMarker` 后 `time.AfterFunc(5min)` 调度清理，指针相等校验防止误删 rejoin 新 marker。
- **测试**: `TestOnParticipantLeft_DedupsSecondCall`、`TestOnParticipantLeft_RejoinClearsDedup`
- **goroutine 吞 error**: `_, _` → `log.Printf`。

### 1.2 StreamInfo 空 token 无感知
- **文件**: `app/server/internal/handler/signal_handler.go`、`internal/srs/provider.go`、`internal/sfu/dynamic_provider.go`
- **问题**: SRS token 生成失败返回 `""`，handler 无条件写入响应。前端拿空 token 推流被拒。
- **修复**: 接口签名改 `(stream, token string, err error)`，handler 检查 err 走 `HandleError` 拒绝。
- **测试**: `TestStreamInfo_EmptySecretReturnsError`、`TestStreamInfo_WithSecretSucceeds`

---

## 2. 冗余扫描汇总

### 2.1 SFU Client (`packages/sfu-client/src/`) — 7 项全修

| 文件 | 问题 | 操作 |
|------|------|------|
| `types.ts` | 未用 export `ProducerReadyInfo` | 删 |
| `mediasoup-client.ts:65` | 冗余 `as Uint8Array<ArrayBuffer>` | 删 cast |
| `mediasoup-client.ts:142,327,354,390` | 4 处 `as never` 类型强转 | 改 `as unknown as T` |
| `srs-client.ts:264-266` | 死分支 `s.pc !== null` 永不真 | 删 |
| `agora-client.ts:103-104` | leaveRoom 未清 listener | 加 `removeAllListeners()` |
| `daily-client.ts:58` | 未用构造参数 `_options` | 删 |
| `factory.ts:4-10` | `preloadSFUClient` 与 `createSFUClient` 重复映射 | 合并简化 |

### 2.2 Go Backend (`app/server/internal/`) — 9 项全修

| 文件 | 问题 | 操作 |
|------|------|------|
| `mediasoup/bridge.go:164` | `CloseParticipant` 404 返 `nil,nil` | 加 `ErrParticipantNotFound` sentinel |
| `mediasoup/signal.go:187` | goroutine 吞 error | 改 `log.Printf` |
| `service/user_service.go:85` | Delete 调 GetByID 丢弃结果 | 直接 `repo.Delete` |
| `service/room_service.go:108` | 同上 | 同上 |
| `service/user_service.go:29` | `ErrUserNotFound` 拼写 4 次 | 抽 sentinel |
| `service/room_service.go:52` | `ErrRoomNotFound` 拼写 3 次 | 抽 sentinel |
| `signal/hub.go:280` | 丢弃 `IsMutedByIdentity` error | 改 `log.Printf` |
| `storage/provider.go:15` | `Name()` 接口方法未用 | 删接口+实现 |

### 2.3 Web Frontend (`app/web/src/`) — ~16 项

**Bug 修复**
| 文件 | 问题 | 操作 |
|------|------|------|
| `components/modal/settting/settingModal.tsx` | `showModal()` 组件顶层调用，渲染即弹窗 | 包 `onMount` |
| `stores/socketStore.ts` | 5 个 listener Set 断连不清理 | disconnect 加 `.clear()` |
| `stores/socketStore.ts` | `sfuEmit`/`signalEmit` 互相包装无意义 | 删 `sfuEmit`，调用点改 `signalEmit` |
| `api/storage.ts:111` | 多余 `}`（agent 删 deleteObject 遗留） | 删 |
| `packages/sfu-client/src/mediasoup-client.ts` | `Uint8Array` 缺泛型参数导致 TS 编译错 | 改 `Uint8Array<ArrayBuffer>` |
| `api/apiClient.ts:51,108,125` | `Authoriztion` 拼写（缺 t） | 实际已正，无需改 |

**死代码删除**
| 文件 | 操作 |
|------|------|
| `api/mute.ts:getMuteStatus` | 删 |
| `api/storage.ts:deleteObject` | 删 |
| `api/email.ts:verifyEmailCode` | 删 |
| `api/storage.ts:ConfirmResult/UploadResult` | 删类型定义 |
| `components/room/roomDetail.tsx:64-74` | 删注释 JSX |
| `components/funcButton.tsx:61-65` | 删僵死 Label A/B 按钮 |
| `api/apiClient.ts:197` | 删注释 export |
| `components/modal/settting/tab_item/room.tsx` | 写空（需手动 `rm`） |
| `components/modal/settting/tab_item/general.tsx` | 写空（需手动 `rm`） |

**未删的 icon import**
- `manage/users/index.tsx`: 仅 `ShieldCheck` 未用已删，其余 7 个在 JSX 中有使用
- `manage/mute/index.tsx`: 5 个 icon 均有使用
- `manage/ban/index.tsx`: 2 个 icon 均有使用
- `manage/permission/index.tsx`: `ShieldX` 未用已删

---

## 3. 未修项记录 (Minor/可搁置)

**Reviewer 发现但用户要求只修上述范围**:

| 文件 | 问题 | 建议 |
|------|------|------|
| `srs/client.go:145-156` | `KickByStreams` remaining 计数可能不准 | 区分请求失败 vs API 返回 |
| `srs/provider.go:109-119` | `DeleteRoom` partial failure 仍 `ClearRoom` | 移入 err==nil 分支 |
| `srs/stream.go:60` | `var _ = hex.EncodeToString` 无用 import 保留 | 删 |
| `srs/client.go:96-115` | `ListParticipants` 返 `[]map[string]interface{}` | 定义 SRSParticipant struct |
| `service/mute_service.go:121` | `GetMuteStatus` 返 `nil,nil` | 加 boolean 或 sentinel |
| `storage/local.go:36` | `PresignUpload` 返 `nil,nil` | 加 NOT_SUPPORTED error |
| `handler/monitor_handler.go:64` | `_, _ = fmt.Fprintf` 吞 SSE write error | 检查并 break |

---

## 4. 验证结果

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./...` ✅ (全绿)
- `pnpm build` ✅ (4/4 tasks, 全成功)

---

## 5. 后续建议

1. 手动删 `room.tsx`、`general.tsx` 文件（已写空内容）
2. 检查 `apiClient.ts:51,108,125` `Authorization` 拼写在修改版本中是否确实正确
3. 确认无 `SRS_SECRET` 的环境（dev/CI）能处理 `StreamInfo` 返回 error 后的降级逻辑

---

# 架构审计（补充）— 2026-07-07

维度：SFU 抽象层 / 后端分层 / 前端 sfu-client + 状态层 / 未提交 diff 架构影响
方法：4 并行子 agent 扫描 + 主线程逐条亲读代码验证。所有 finding 已验。
严重度图例：🔴 bug / 🟡 risk / ❓ 架构 disagreement / 🔵 nit

> 注：下文部分 finding 与上文 §1-§3 已修项重叠（如 `storage.ts:111` 多余 `}`、`hub.go:280` mute fail-open、`bridge.go` 404 改动）。重叠项标注 `[上文已记]`，新增架构维度分析为本次补充重点。

---

## 6. 🔴 bug（架构维度）

### 6.1 SRS 客户端订阅 race 守卫被删

`packages/sfu-client/src/srs-client.ts:263`

```diff
 .catch(() => {
   const s = this.peerSubs.get(identity);
   if (!s) { pc.close(); return; } // cleanup already happened (unsubscribePeer)
-  if (s.pc !== null) { pc.close(); return; } // a successful sub completed; this catch is stale
+  pc.close();
   const prevRetryCount = s.retryCount;
   ...
   this.peerSubs.delete(identity);
```

> ⚠️ 与上文 §2.1 `srs-client.ts:264-266 死分支 s.pc !== null 永不真 删` 冲突——上文判定为死代码删除，本次审计判定为 race 守卫。**待复核**：若 `exchangeSdp` 成功路径确会置 `s.pc`，则守卫非死代码，删除引入 race。

**问题**：守卫移除。订阅流程：`exchangeSdp(pc1)` reject 时，`exchangeSdp(pc2)` 可能已成功并 `s.pc = pc2`。旧守卫经 `s.pc !== null` 判定 stale catch 跳过。新代码直接 `pc.close()` + `peerSubs.delete(identity)`（删含 pc2 条目）+ `scheduleRetry()`。pc2 孤立、用户无谓断连重连。

**修复**：恢复守卫，或比较 raceToken/同一 pc 引用后跳过 delete。

### 6.2 mediasoup GenerateToken 带 HTTP 副作用

`app/server/internal/mediasoup/provider.go:35`

```go
func (s *Service) GenerateToken(room, identity string) (string, error) {
	if err := s.Bridge.CreateRouter(room); err != nil {
		return "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return fmt.Sprintf("%s:%s", room, identity), nil
}
```

**问题**：`GenerateToken` 调 `s.Bridge.CreateRouter(room)`——纯签发凭证带建路由器 HTTP 副作用。LiveKit/SRS/Daily/Agora 同接口无此行为。换 provider 调用方无感知行为差异，且每次取 token 打一次 bridge HTTP。

**修复**：room 创建移出 `GenerateToken`，独立 `EnsureRoom(room)`，token 只返 `room:identity`。

### 6.3 DynamicProvider data race

`app/server/internal/sfu/dynamic_provider.go:21-22,150`

`SetRoomRegistry` 写 `p.roomRegistry` 无锁，`current()` 并发读同字段。Hub 启动调 setter 与请求触发 current() 并发，`go test -race` 必报。Go 内存模型下无同步读写是 UB。

**修复**：加 `sync.RWMutex`，setter 写锁、current 读锁。

### 6.4 socketStore 重连事件 handler 翻倍

`app/web/src/stores/socketStore.ts:127-291`

`connect()` 每次 `adapter.onServerEvent()` 注册 handler，`disconnect()` 仅清内部 listener Set 不解绑 adapter 层绑定。`disconnect()` + `connect()` 循环导致 handler 翻倍累积，重复响应、toast 弹窗泛滥。

> 上文 §2.3 记 "5 个 listener Set 断连不清理 → disconnect 加 `.clear()`"，但 `.clear()` 只清内部 Set，未解绑 adapter 层 `onServerEvent`。**本条补充**：需额外 `adapter.offServerEvent()` 解绑。

**修复**：`disconnect()` 调 `adapter.offServerEvent()`，或 `connect()` 开头先清旧绑。

---

## 7. 🟡 risk（架构维度）

### 7.1 mediasoup CloseParticipant 404 改返 error `[上文已记 §2.2]`

`app/server/internal/mediasoup/bridge.go:166`

上游 `mediasoup/provider.go` `RemoveParticipant` 将任何非 nil error 转为 `SFU_ERROR`，故 `RemoveParticipant("room","ghost")` 现返 500 而非成功。HTTP DELETE 预期幂等——"已不在"不应报内部错误。

**修复**：`RemoveParticipant` 对 `ErrParticipantNotFound` 返 nil，或 bridge 404 仍返 nil。

### 7.2 Delete 移除 pre-check 静默成功 `[上文已记 §2.2]`

`app/server/internal/service/room_service.go:93` + `user_service.go:79`

GORM `Delete` 对不存在行返 nil error，删不存在的 ID 现静默成功而非报 `NOT_FOUND`。

**修复**：repo 层 `RowsAffected==0` 返 `NotFound`，或保留 service pre-check。

### 7.3 mute 检查 fail-open 无告警 `[上文已记 §2.2]`

`app/server/internal/signal/hub.go:280`

muteStore 因 DB 挂返 error 时，所有禁言检查静默旁路（mute=false 放行），用户可随意进禁言房。`log.Printf` 无告警。

**修复**：DB 错误 fail-closed 直接 block，或加结构化日志/告警指标。

### 7.4 ListRooms/ListParticipants 返 interface{} ⭐

`app/server/internal/sfu/provider.go:13,17`

```go
ListRooms() (interface{}, error)
ListParticipants(room string) (interface{}, error)
```

各 provider 返异构类型：LiveKit `[]*livekit.Room`，SRS/Daily `[]map[string]interface{}`，Agora `[]string`，mediasoup `[]ParticipantInfo`。`signal_handler.go:182` 直接透传前端，前端须按 provider 解析。接口退化为无契约，换 provider 前端静默崩——**抽象失败根因**：Provider 接口本意隔离 provider 差异，结果差异全透传到响应层。

**修复**：定义 `RoomInfo`/`ParticipantInfo` 公共结构，各 provider 做映射。

### 7.5 DynamicProvider 每次调用重建 provider

`app/server/internal/sfu/dynamic_provider.go:140-156`

`current()` 每次调 `NewProvider(cfg)`。LiveKit/mediasoup 建 gRPC/HTTP 客户端，每个 SFU 调用都重建，开销在高频下暴露。

**修复**：缓存 provider + 配置版本/TTL 失效。

### 7.6 SRS RemoveParticipant 依赖 stream 命名约定

`app/server/internal/srs/provider.go:93-105`

`RemoveParticipant` 靠 `GenerateStreamName(room, identity)` 反算 stream 名踢人。命名约定改（当前 `gs-` + sha256 base36 截断）即静默失效。room+identity→stream 双射假设泄漏。

**修复**：SRS server 侧登记 client→stream 映射反查踢人，或 provider 维护映射。

### 7.7 SRS ListRooms 降级返 stream 名

`app/server/internal/srs/provider.go:61-75`

`registry==nil` 时 `ListRooms` 调 `/api/v1/streams` 返 stream 名（如 `gs-xxxx`）非 room 名。接口名误导，调用方拿到错语义数据。

**修复**：registry 缺失返 error，不返错语义数据。

### 7.8 HTTP handler 跨 service 直连 SFU

`app/server/internal/handler/signal_handler.go:14,82,152,176`

HTTP handler 直持 `sfu.Provider`，`GenerateToken`/`ListRooms`/`ListParticipants` 无 service 中间层。分层契约 handler→service→repository/SFU 被破，业务规则（鉴权、限流、计费）无处落脚。

**修复**：抽 `SFUService`，handler 只调 service。

### 7.9 Hub 跨 service 持 repository

`app/server/internal/signal/hub.go:83-85`

Hub 持 `roomStore`/`muteStore`/`userStore` repository，signal 层跨 service 直查 DB。

**修复**：Hub 持 service 接口，不持 repository。

### 7.10 mediasoup 在 DI 容器硬编码分支

`app/server/server/gin.go:125-129`

```go
if resolvedSFUCfg.SFUProvider == "mediasoup" {
    msService := mediasoup.NewService(resolvedSFUCfg)
    ...
}
```

provider 类型泄漏到组装层。新增 provider 改 factory.go + gin.go + sfu_config_service 白名单三处，开闭原则破。

**修复**：provider 自注册（`init()` `Register(name, factory)`），signal 事件注册走 provider 接口回调。

### 7.11 全项目零事务

`app/server/internal/auth_service.go:164-168` + `oauth_service.go:165,175`

全项目无一处 `db.Transaction`。改密 `Update` + `IncrementTokenVersion` 两独立写无事务——Update 成功版本未增，旧 token 仍有效。建 user + oauth_account 无事务，第二写失败产孤儿用户。SQLite 下并发低不易显，Postgres/MySQL 生产必踩数据不一致。

**修复**：service 跨 repo 操作包 `db.Transaction(func(tx) {...})`，repo 接 `tx` 参数。

### 7.12 apiClientAuth 缺省 no-op 静默失败

`app/web/src/api/apiClientAuth.ts:8-13`

`bindings` 缺省全 no-op。初始化前发请求 token 恒空，无错误提示静默失败。

**修复**：缺省 `getAccessToken` 改 `throw new Error("APIClientAuth not initialized")`。

### 7.13 前端 factory default 静默回落 LiveKit

`packages/sfu-client/src/factory.ts:52-57`

未知 provider 走 `default` 分支静默回落 LiveKit，不报错挂错 SFU，无声通话难排查。

**修复**：`default: throw new Error("unknown provider: " + provider)`。

### 7.14 SRS stream.go 保留无用 hex import hack `[上文已记 §3]`

`app/server/internal/srs/stream.go:60`

```go
var _ = hex.EncodeToString // keep encoding/hex import only if used elsewhere
```

`hex` 未在文件使用，hack 避编译错误。

**修复**：删多余 `encoding/hex` import。

---

## 8. ❓ 架构 disagreement

### 8.1 Hub 上帝对象

`app/server/internal/signal/hub.go`（26KB）

单文件五职责：信令路由 + 业务校验(禁言/人数/密码) + 内存状态管理 + SFU 清理 + stream 聚合。五职责耦合使任一改动风险扩散。`OnRoomJoin` 同时验密码、查禁言、限人数，业务规则与传输协议同层。Hub 难测——mock SFU + DB + Socket 三者才能测一条校验。

**修复**：拆 `RoomPolicyService`(校验) + `RoomRegistry`(内存状态) + Hub 只留事件分发。

### 8.2 mediasoup 多入口清理未统一

`app/server/internal/mediasoup/signal.go`

`OnParticipantLeft` 被 3 入口触发（disconnect + ws `close-transport` + webhook）。最近 3 commit（`dea03f0`/`21565fa`/`47b711f`）周期加锁/去重/过滤，治标。根因多入口未统一到单一仲裁点。当前 TTL cleanup（`signal.go:175` `time.AfterFunc`）是第 4 层补丁。

**修复**：统一清理入口到单一仲裁函数，所有触发源经它去重。

### 8.3 GetHost 跨 provider 语义异构

`app/server/internal/sfu/provider.go:31`

`GetHost()` 各 provider 返异构 URL：LiveKit 返 WebSocket host，SRS 返 WHIP HTTP URL，Daily 返 HTTPS domain，Agora 返 host 或 fallback。`signal_handler.go:90` 当 `serverUrl` 发前端，前端须按 provider 选连接方式。同名方法不同语义比无方法更糟，接口未隔离差异反放大。

**修复**：从接口移除，并入 `ClientInfo()` map，前端按 provider 自行构造。

### 8.4 StreamName/StreamInfo/ClientInfo 类型断言绕接口

`app/server/internal/sfu/dynamic_provider.go:105-138`

三方法不在 `Provider` 接口，靠运行时类型断言 `provider.(interface{...})` 分派。编译时无法检查 provider 实现，调用方（`signal_handler.go:94,101`、`gin.go:118`）各自断言。未实现时静默返空（`dynamic_provider.go:126`），调用方无法区分"不支持"与"正常空"。

**修复**：提升到 `Provider` 接口，或统一类型断言逻辑到一处。

---

## 9. 🔵 nit

- `app/server/internal/signal/hub.go:78`：`sfuProviderName` 存后从未读。死字段。删。
- `app/web/src/api/{mute,email,storage}.ts`：每函数 `if (result.code !== 0) throw` 重复 12+ 处。抽 `apiClient.request` 统一拦截。
- `packages/sfu-client/src/*.ts`：5 client 重复 8 callback 方法约 250 行。提 mixin/基类。
- `packages/sfu-client/src/daily-client.ts:55`：构造器删 `options` 参数，与其余 4 client 不一致。统一签名。[上文已记 §2.1]
- `app/web/src/components/modal/settting/tab_item/audio.tsx:6`：深 rel path `../../../../stores`。配 `@/` alias。
- `app/server/internal/sfu/dynamic_provider.go:19-20`：注释承认 `current()` 每次重建使 `SetRoomRegistry` 必重放，脆弱——某天 `current()` 缓存后 `SetRoomRegistry` 重新生效，行为悄然改变。

---

## 10. 驳回（agent 误报）

`app/server/internal/service/room_service.go:13` + `user_service.go:16` "自引用 sentinel error 恒 nil"——假阳性。实际：

```go
var ErrRoomNotFound = pkg.NewAppError(pkg.NOT_FOUND, "room not found")
```

正常初始化，非 `var ErrX = ErrX` 自引用。haiku 子 agent 误读 `=` 右侧。已亲读确认 `GetByID`/`GetByUUID` 命中 `gorm.ErrRecordNotFound` 正确返 `ErrRoomNotFound`。

---

## 11. 最重 5 条（按修复价值排序）

| # | 位置 | 理由 |
|---|------|------|
| 1 | `srs-client.ts:263` | 用户直接感知断连，回归风险高。⚠️ 与上文 §2.1 死代码判定冲突，待复核 |
| 2 | `dynamic_provider.go` data race | `-race` 必报，生产 UB |
| 3 | `storage.ts:111` [上文已记] | 编译崩，阻塞构建 |
| 4 | `mediasoup GenerateToken` 副作用 | 抽象泄漏根因 |
| 5 | `ListRooms/ListParticipants` 返 `interface{}` | 抽象失败根因 |

---

## 12. 统计（本次架构维度补充）

| 严重度 | 数量 |
|--------|------|
| 🔴 bug | 4 |
| 🟡 risk | 14 |
| ❓ 架构 | 4 |
| 🔵 nit | 6 |
| 驳回 | 1 |

---

*架构审计补充于 2026-07-07，分支 `feature/srs-sfu`。所有 finding 经主线程亲读代码验证。*

---

# 批量修复日志 — 2026-07-07

复核 §3/§6/§7/§9 全部"未修"项，逐条亲读代码验证现状。**发现工作树已超前于本文档"未修"标签**——多条 §6/§7 项实际已修。本批仅对真·开放项动手。

## 已验证为"工作树已修"（文档滞后，无需动）

| 项 | 现状证据 |
|----|---------|
| §6.3 DynamicProvider data race | `dynamic_provider.go` 已有 `sync.RWMutex`，setter 写锁/current 读锁 |
| §6.4 socketStore handler 翻倍 | `socket/client.ts` disconnect 调 `offAllServerEvents()` 解绑 adapter 层；socketStore disconnect 同调 |
| §7.1 mediasoup RemoveParticipant 404 | `provider.go:95` `errors.Is(err, ErrParticipantNotFound)` 返 nil（幂等） |
| §7.2 Delete 移除 pre-check | `room_service.go:110` + `user_service.go:87` 仍保留 GetByID pre-check，返 NOT_FOUND（§2.2 描述与现状不符） |
| §7.12 apiClientAuth no-op | `apiClientAuth.ts:9` `getAccessToken` 已 throw "not initialized" |
| §7.13 factory default | `factory.ts:59` default 已 throw "unknown SFU provider" |
| §7.14 / §3.3 hex hack | `stream.go` 已删 `encoding/hex` import + `var _ = hex...` |
| §9.1 sfuProviderName 死字段 | `hub.go` struct 无此字段（仅 AGENTS.md 文档残留） |

## §6.1 裁决：假阳性，不修

审计 §6.1 判 `srs-client.ts:263` 删 `if (s.pc !== null)` 守卫引入 race。**复核驳回**：

- `subscribePeer` line 187 守卫 `if (existing && (existing.pc !== null || existing.connecting)) return` 早存于 HEAD（git diff 确认仅删守卫一行，未动 connecting flag）。
- 单次 `subscribePeer` 只创 1 个 pc + 调 1 次 `exchangeSdp`。`.then`/`.catch` 对同一 promise 互斥——`.then` 成功置 `s.pc` 后 `.catch` 永不触发，反之亦然。
- 故 `.catch` 内 `s.pc !== null` 在同一 exchangeSdp 上不可能为真；跨 exchangeSdp 的双 pc 并发被 connecting 守卫阻断。

§6.1 描述的 pc1/pc2 双并发场景在当前架构结构性不可达。§2.1 删除正确，无 race。

## 本批真·修复

### B1. §6.2 mediasoup GenerateToken 副作用移除致回归 🔴

`GenerateToken` 移除 `CreateRouter` 后，`EnsureRoom`（包装 CreateRouter）**零调用方**——grep 仅见定义。mediasoup-worker `api.ts:27` `GET /rooms/:id/rtp-capabilities` 不懒建 router（不存在返 404），router 仅 `POST /rooms` 创建。故 **mediasoup router 永不建 → get-router-capabilities 404 → 流程全挂**。真回归。

修：`get-router-capabilities`（mediasoup 客户端首调）handler 内先 `bridge.CreateRouter(req.Room)`（worker.createRouter 幂等，存在返 existing）再取 caps。`CreateRouter` 加入 `participantBridge` 接口。删死码 `Service.EnsureRoom`。

### B2. §3.6 local PresignUpload nil 解引用 panic 🔴

`local.go` PresignUpload 返 `nil,nil`；`storage_handler.go:93` `result.ObjectKey` 在 line 95 nil 检查**之前**解引用 → local 模式 `/storage/presign` 必 panic 500。前端 `useUpload.ts` local 模式必调 presign 拿 object_key。真 bug。

修：返 `&PresignedResult{ObjectKey: key}`（UploadURL 空），handler line 95 跳过 UploadURL，前端走 `/storage/upload` 中转。

### B3. §3.2 srs DeleteRoom ClearRoom 时序

`ClearRoom` 原在 `KickByStreams` err 检查之前调 → partial failure 仍清聚合视图，重试无 stream 可踢。修：移入 err==nil 成功分支（NOT_FOUND 不清，len(streams)==0 时无害）。

### B4. §3.7 monitor SSE 吞 write error

`monitor_handler.go:64` `_, _ = fmt.Fprintf` 客户端断连后 write error 被吞，循环空转到下个 ticker（≤2s）。修：检查 err，非 nil 即 return。

### B5. §7.7 srs ListRooms 降级返错语义数据

registry==nil 时回退 `/api/v1/streams` 返 stream 名（非 room 名）。registry 由 Hub 始终注入，该路径运行时不可达但为潜在 footgun。修：返 `SFU_ERROR "srs room registry not configured"`。删随之死去的 `Client.ListRooms` + `streamsResponse` + 测试 `/api/v1/streams` handler；翻转 `TestListRooms_NoRegistry_Fallback` → `TestListRooms_NoRegistry_ReturnsError`。

## 搁置（低价值 / 被 §7.4 阻塞 / 属 §8 大重构）

§3.1 KickByStreams remaining 计数（当前可辩护）、§3.4 ListParticipants struct（被 §7.4 interface{} 阻塞）、§3.5 GetMuteStatus nil（handler 契约清晰 null=未禁言）、§7.3 mute fail-open（可用性权衡 + 已加 WARNING log）、§7.5/7.6/7.8/7.9/7.10/7.11（大重构）、§9.2-9.6（nit）。

## 验证

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./...` ✅（mediasoup/srs/signal/service/repository/middleware 全绿）
- `pnpm build` ✅（4/4 tasks）
