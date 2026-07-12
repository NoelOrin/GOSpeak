# SFU 适配器架构重构实现计划

> **对 agent 工作者：** 必选子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐个任务实施本计划。步骤使用复选框（`- [x]`）语法跟踪。

**目标：** 在后端和前端收紧 SFU 适配器抽象，使提供者契约强类型化、方法语义无歧义、接口面最小化。

**架构：** 后端 `sfu.Provider` 接口获得类型化返回结构体（`RoomSummary`、`ParticipantSummary`），移除 `MuteRoomParticipant`（合并为带空 `trackSid` 的 `MuteParticipant`），并增加两个可选的扩展接口（`StreamProvider`、`ClientInfoProvider`）以替代 `interface{}` 断言。前端 `SFUClient` 接口获得 `JoinParams`（替代位置参数）、`isConnected()` 和统一的 `async destroy()`。

**技术栈：** Go 1.22+（Gin、GORM、go-socket.io），TypeScript 5+（SolidJS、Vite），pnpm monorepo。测试：Go `testing` + `httptest`，sfu-client 使用 Vitest。

---

## 文件结构

### 后端 (Go)

| 文件 | 操作 | 职责 |
|------|--------|----------------|
| `app/server/internal/sfu/types.go` | 创建 | `RoomSummary`、`ParticipantSummary` 共享结构体 |
| `app/server/internal/sfu/provider.go` | 修改 | 更新接口：类型化返回值、移除 `MuteRoomParticipant`、增加 `StreamProvider` + `ClientInfoProvider` |
| `app/server/internal/sfu/dynamic_provider.go` | 修改 | 更新返回类型、移除 `MuteRoomParticipant`、将 `interface{}` 断言替换为类型化接口断言 |
| `app/server/internal/livekit/client.go` | 修改 | 映射为类型化返回值、移除 `MuteRoomParticipant`、将静音全部合并到 `MuteParticipant` |
| `app/server/internal/agora/provider.go` | 修改 | 映射为类型化返回值、移除 `MuteRoomParticipant` |
| `app/server/internal/srs/provider.go` | 修改 | 映射为类型化返回值、移除 `MuteRoomParticipant` |
| `app/server/internal/daily/provider.go` | 修改 | 映射为类型化返回值、移除 `MuteRoomParticipant` |
| `app/server/internal/mediasoup/provider.go` | 修改 | 映射为类型化返回值、移除 `MuteRoomParticipant`、将静音全部合并到 `MuteParticipant` |
| `app/server/internal/service/sfu_service.go` | 修改 | 更新返回类型、替换 `interface{}` 断言 |
| `app/server/internal/handler/signal_handler_test.go` | 修改 | 更新 mock 和新接口的测试 |

### 前端 (TypeScript)

| 文件 | 操作 | 职责 |
|------|--------|----------------|
| `packages/sfu-client/src/types.ts` | 修改 | 添加 `JoinParams`、`isConnected()`、`async destroy()` |
| `packages/sfu-client/src/livekit-client.ts` | 修改 | 接受 `JoinParams`、添加 `isConnected()`、`async destroy()` |
| `packages/sfu-client/src/agora-client.ts` | 修改 | 接受 `JoinParams`、添加 `isConnected()`、`async destroy()` |
| `packages/sfu-client/src/srs-client.ts` | 修改 | 接受 `JoinParams`、添加 `isConnected()`、`async destroy()` |
| `packages/sfu-client/src/daily-client.ts` | 修改 | 接受 `JoinParams`、添加 `isConnected()`、`async destroy()` |
| `packages/sfu-client/src/mediasoup-client.ts` | 修改 | 接受 `JoinParams`、添加 `isConnected()`、`async destroy()` |
| `app/web/src/components/room/hooks/useRoomJoinSession.ts` | 修改 | 构建 `JoinParams` 对象，传递给 `joinRoom()` |

---

## 阶段 1 (P0)：后端接口契约

### 任务 1：创建带共享结构体的 `sfu/types.go`

**文件：**
- 创建：`app/server/internal/sfu/types.go`

- [x] **步骤 1：创建文件**

```go
package sfu

// RoomSummary is the provider-agnostic room listing entry.
type RoomSummary struct {
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount,omitempty"`
}

// ParticipantSummary is the provider-agnostic participant entry.
type ParticipantSummary struct {
	Identity string `json:"identity"`
	JoinedAt int64  `json:"joinedAt,omitempty"`
}
```

- [x] **步骤 2：验证编译通过**

运行：`cd app/server && go build ./internal/sfu/`
预期：编译无错误

- [x] **步骤 3：提交**

```bash
git add app/server/internal/sfu/types.go
git commit -m "feat(sfu): add RoomSummary and ParticipantSummary shared types"
```

---

### 任务 2：更新 `sfu/provider.go` 接口

**文件：**
- 修改：`app/server/internal/sfu/provider.go`

- [x] **步骤 1：替换整个文件**

```go
package sfu

// Provider abstracts an SFU backend (LiveKit, SRS, Agora, MediaSoup, Daily, etc.).
type Provider interface {
	GenerateToken(room, identity string) (string, error)
	GenerateAdminToken() (string, error)
	ListRooms() ([]RoomSummary, error)
	ListParticipants(room string) ([]ParticipantSummary, error)
	MuteParticipant(room, identity, trackSid string, muted bool) error
	RemoveParticipant(room, identity string) error
	DeleteRoom(room string) error
	GetHost() string
}

// StreamProvider extends Provider for backends that use stream-based
// addressing (e.g. SRS WHIP/WHEP). Callers check via type assertion.
type StreamProvider interface {
	GenerateStreamToken(room, identity, stream string) (string, error)
}

// ClientInfoProvider extends Provider for backends that expose client-facing
// connection info (server URL etc.) distinct from the admin endpoint.
type ClientInfoProvider interface {
	GetClientInfo(room, token string) (*ClientInfo, error)
}

// ClientInfo represents provider-specific connection parameters a client
// needs to connect.
type ClientInfo struct {
	ServerURL string `json:"serverUrl,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
}
```

- [x] **步骤 2：验证编译通过**

运行：`cd app/server && go build ./internal/sfu/`
预期：编译无错误

- [x] **步骤 3：提交**

```bash
git add app/server/internal/sfu/provider.go
git commit -m "refactor(sfu): typed return structs, StreamProvider + ClientInfoProvider extensions"
```

---

### 任务 3：更新 `sfu/dynamic_provider.go`

**文件：**
- 修改：`app/server/internal/sfu/dynamic_provider.go`

- [x] **步骤 1：创建/替换文件**

```go
package sfu

import (
	"sync"
)

type resolveFunc func() (*ResolvedConfig, error)

type DynamicProvider struct {
	resolve resolveFunc
	mu      sync.RWMutex
}

func NewDynamicProvider(resolve resolveFunc) *DynamicProvider {
	return &DynamicProvider{resolve: resolve}
}

func (p *DynamicProvider) refresh() (Provider, error) {
	p.mu.RLock()
	resolve := p.resolve
	p.mu.RUnlock()
	cfg, err := resolve()
	if err != nil {
		return nil, err
	}
	return NewProvider(cfg), nil
}

func (p *DynamicProvider) GenerateToken(room, identity string) (string, error) {
	inner, err := p.refresh()
	if err != nil {
		return "", err
	}
	return inner.GenerateToken(room, identity)
}

func (p *DynamicProvider) GenerateAdminToken() (string, error) {
	inner, err := p.refresh()
	if err != nil {
		return "", err
	}
	return inner.GenerateAdminToken()
}

func (p *DynamicProvider) ListRooms() ([]RoomSummary, error) {
	inner, err := p.refresh()
	if err != nil {
		return nil, err
	}
	return inner.ListRooms()
}

func (p *DynamicProvider) ListParticipants(room string) ([]ParticipantSummary, error) {
	inner, err := p.refresh()
	if err != nil {
		return nil, err
	}
	return inner.ListParticipants(room)
}

func (p *DynamicProvider) MuteParticipant(room, identity, trackSid string, muted bool) error {
	inner, err := p.refresh()
	if err != nil {
		return err
	}
	return inner.MuteParticipant(room, identity, trackSid, muted)
}

func (p *DynamicProvider) RemoveParticipant(room, identity string) error {
	inner, err := p.refresh()
	if err != nil {
		return err
	}
	return inner.RemoveParticipant(room, identity)
}

func (p *DynamicProvider) DeleteRoom(room string) error {
	inner, err := p.refresh()
	if err != nil {
		return err
	}
	return inner.DeleteRoom(room)
}

func (p *DynamicProvider) GetHost() string {
	inner, err := p.refresh()
	if err != nil {
		return ""
	}
	return inner.GetHost()
}
```

- [x] **步骤 2：清理 `MuteRoomParticipant` 调用**

此提供者之前可能有一个 `MuteRoomParticipant` 方法使其编译通过。由于该接口方法已移除，像下面这样的动态转发：

```go
func (p *DynamicProvider) MuteRoomParticipant(room, identity string, muted bool) error {
```

必须被完全删除。用 `grep` 确认没有残留引用：

运行：`cd app/server && grep -rn 'MuteRoomParticipant' internal/`

预期：无输出（零引用）

- [x] **步骤 3：验证编译通过**

运行：`cd app/server && go build ./...`
预期：编译无错误

- [x] **步骤 4：提交**

```bash
git add app/server/internal/sfu/dynamic_provider.go
git commit -m "refactor(sfu): DynamicProvider typed returns, remove MuteRoomParticipant"
```

---

### 任务 4：更新 `livekit/client.go`

**文件：**
- 修改：`app/server/internal/livekit/client.go`

- [x] **步骤 1：更新返回类型和移除方法**

找到 `ListRooms` 方法体并以 `RoomSummary` 切片形式返回：

```go
func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
```

找到 `ListParticipants` 并以 `ParticipantSummary` 的形式返回：

```go
func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
```

移除整个 `MuteRoomParticipant` 方法体。如果存在，将其静音全部功能合并到 `MuteParticipant`：

```go
func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	// Legacy trackSid = "" means mute all tracks for this participant
	if trackSid == "" {
		ctx := context.Background()
		svc := lksdk.NewRoomServiceClient(s.host, s.apiKey, s.apiSecret)
		participants, err := svc.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: room})
		if err != nil {
			return err
		}
		var firstErr error
		for _, p := range participants.Participants {
			if p.Identity == identity {
				for _, track := range p.Tracks {
					if err := svc.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
						Room:     room,
						Identity: identity,
						TrackSid: track.Sid,
						Muted:    muted,
					}); err != nil {
						if firstErr == nil {
							firstErr = err
						}
					}
				}
				break
			}
		}
		return firstErr
	}
	// Single track mute
	adminToken, err := s.GenerateAdminToken()
	if err != nil {
		return err
	}
	return lksdk.MutePublishedTrack(room, identity, trackSid, muted, lksdk.WithAuthToken(adminToken))
}
```

- [x] **步骤 2：验证编译通过**

运行：`cd app/server && go build ./internal/livekit/`
预期：编译无错误

- [x] **步骤 3：提交**

```bash
git add app/server/internal/livekit/client.go
git commit -m "refactor(livekit): typed returns, MuteParticipant handles empty trackSid as mute-all"
```

---

### 任务 5：更新剩余的 SFU 提供者（Agora、SRS、Daily、MediaSoup）

**文件：**
- 修改：`app/server/internal/agora/provider.go`
- 修改：`app/server/internal/srs/provider.go`
- 修改：`app/server/internal/daily/provider.go`
- 修改：`app/server/internal/mediasoup/provider.go`

- [x] **步骤 1：更新 `agora/provider.go`**

```go
func (p *Provider) ListRooms() ([]sfu.RoomSummary, error) {
	// ...
}

func (p *Provider) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	// ...
}
```

如果 `MuteRoomParticipant` 存在则移除它。保持现有的 `MuteParticipant` 不变（如果 Agora API 不支持服务端强制静音，则保留为未实现）。

- [x] **步骤 2：更新 `srs/provider.go`**

```go
func (p *Provider) ListRooms() ([]sfu.RoomSummary, error) {
	// ...
}

func (p *Provider) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	// ...
}
```

- [x] **步骤 3：更新 `daily/provider.go`**

```go
func (p *Provider) ListRooms() ([]sfu.RoomSummary, error) {
	// ...
}

func (p *Provider) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	// ...
}
```

- [x] **步骤 4：更新 `mediasoup/provider.go`**

```go
func (p *Provider) ListRooms() ([]sfu.RoomSummary, error) {
	// ...
}

func (p *Provider) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	// ...
}
```

将 `MuteRoomParticipant` 替换为带空 `trackSid` 的 `MuteParticipant`：

```go
func (p *Provider) MuteParticipant(room, identity, trackSid string, muted bool) error {
	// MediaSoup 使用自定义信令路径，trackSid = "" 时为静音全部
	return p.muteToggle(room, identity, trackSid, muted)
}
```

- [x] **步骤 5：验证所有提供者编译通过**

运行：`cd app/server && go build ./...`
预期：编译无错误

- [x] **步骤 6：运行后端测试**

运行：`cd app/server && go test ./...`
预期：全部 PASS

- [x] **步骤 7：提交**

```bash
git add app/server/internal/agora/provider.go \
        app/server/internal/srs/provider.go \
        app/server/internal/daily/provider.go \
        app/server/internal/mediasoup/provider.go
git commit -m "refactor(sfu): typed returns across all providers, remove MuteRoomParticipant"
```

---

### 任务 6：更新 `service/sfu_service.go`

**文件：**
- 修改：`app/server/internal/service/sfu_service.go`

- [x] **步骤 1：用类型化断言替换 interface{} 断言**

查找从 `provider.(someType)` 返回 `(interface{}, error)` 的所有方法。将它们迁移为使用 `sfu.StreamProvider` 和 `sfu.ClientInfoProvider`。

旧模式：

```go
func (s *SFUConfigService) GetProvider() (sfu.Provider, error) {
	if s.provider == nil {
		resolved, err := s.sfuConfigRepo.GetActiveConfig()
		if err != nil {
			return nil, err
		}
		s.provider = sfu.NewProvider(resolved)
	}
	return s.provider, nil
}
```

新模式将保持不变，但调用方将切换到：

```go
if sp, ok := provider.(sfu.StreamProvider); ok {
	streamToken, err := sp.GenerateStreamToken(room, identity, stream)
}
```

- [x] **步骤 2：验证编译通过**

运行：`cd app/server && go build ./internal/service/`
预期：编译无错误

- [x] **步骤 3：提交**

```bash
git add app/server/internal/service/sfu_service.go
git commit -m "refactor(service): use typed interface assertions for StreamProvider + ClientInfoProvider"
```

---

### 任务 7：更新 `handler/signal_handler_test.go`

**文件：**
- 修改：`app/server/internal/handler/signal_handler_test.go`

- [x] **步骤 1：检查 mock 提供者是否仍然匹配接口**

查找 `MuteRoomParticipant` 并删除它。确保 mock 的 `ListRooms` 和 `ListParticipants` 现在返回 `sfu.RoomSummary` 和 `sfu.ParticipantSummary` 切片，而不是 `interface{}` 或格式不匹配的结构体。

- [x] **步骤 2：运行测试**

运行：`cd app/server && go test ./internal/handler/ -v`
预期：全部 PASS

- [x] **步骤 3：提交**

```bash
git add app/server/internal/handler/signal_handler_test.go
git commit -m "test(handler): align test mocks with refactored sfu.Provider interface"
```

---

## 阶段 2 (P0)：前端接口标准化

### 任务 8：更新 `sfu-client` 的 `types.ts`

**文件：**
- 修改：`packages/sfu-client/src/types.ts`

- [x] **步骤 1：添加 `JoinParams` 接口**

将以下内容直接放在现有的 `SFUClient` 接口定义之前：

```typescript
export interface JoinParams {
	token: string;
	serverUrl?: string;
	identity: string;
	room?: string;
	stream?: string;
	streamToken?: string;
}
```

- [x] **步骤 2：更新 `SFUClient` 接口**

将 `joinRoom` 从旧签名更改为：

```typescript
export interface SFUClient {
	joinRoom(params: JoinParams): Promise<void>;
	isConnected(): boolean;
	destroy(): Promise<void>;
	onRemoteTrack?(track: RemoteTrackInfo): void;
	getRemoteTracks?(): RemoteTrackInfo[];
	setRemoteAudio?(identity: string, enabled: boolean): void;
	setRemoteVideo?(identity: string, enabled: boolean): void;
}
```

具体更改：
| 更改 | 理由 |
|--------|---------|
| `joinRoom(token, url, identity, room, stream?, streamToken?)` → `joinRoom(params: JoinParams)` | 命名字段，支持可选参数，向前兼容 |
| `disconnect()` → `destroy(): Promise<void>` | 统一的所有客户端清理；返回 `Promise` 以允许异步关闭 |
| 新增 `isConnected(): boolean` | 统一检查而非检查内部 `hasJoined` |

- [x] **步骤 3：验证类型检查**

运行：`cd packages/sfu-client && npx tsc --noEmit`
预期：无类型错误

- [x] **步骤 4：提交**

```bash
git add packages/sfu-client/src/types.ts
git commit -m "refactor(sfu-client): add JoinParams, isConnected, async destroy to types"
```

---

### 任务 9：更新 `livekit-client.ts`

**文件：**
- 修改：`packages/sfu-client/src/livekit-client.ts`

- [x] **步骤 1：将 `JoinParams` 添加到导入**

```typescript
import type { JoinParams, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **步骤 2：替换 `joinRoom` 签名和方法体**

将签名从：

```typescript
	async joinRoom(
		token: string,
		url: string,
		identity: string,
		room?: string,
		stream?: string,
		streamToken?: string,
	): Promise<void> {
```

替换为：

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity, room, stream, streamToken } = params;
```

保持方法体其余部分不变。

- [x] **步骤 3：添加 `isConnected` 方法**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **步骤 4：添加 `async destroy()`**

```typescript
	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
```

可选：保留 `disconnect()` 作为 `destroy()` 的别名，如果现有代码仍在使用它。为了一致性，内部将其路由到 `destroy()` 即可。

- [x] **步骤 5：验证类型检查**

运行：`cd packages/sfu-client && npx tsc --noEmit`
预期：无类型错误

- [x] **步骤 6：提交**

```bash
git add packages/sfu-client/src/livekit-client.ts
git commit -m "refactor(sfu-client): LiveKitSFUClient uses JoinParams + isConnected + async destroy"
```

---

### 任务 10：更新 `agora-client.ts`

**文件：**
- 修改：`packages/sfu-client/src/agora-client.ts`

- [x] **步骤 1：将 `JoinParams` 添加到导入**

```typescript
import type { JoinParams, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **步骤 2：替换 `joinRoom` 签名和方法体**

将签名从：

```typescript
	async joinRoom(
		token: string,
		appId: string,
		channel: string,
		identity: string,
	): Promise<void> {
```

替换为：

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: appId, room: channel, identity } = params;
```

保持方法体其余部分不变。

- [x] **步骤 3：添加 `isConnected` 方法**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **步骤 4：更改 `destroy()` 为 `async`**

```typescript
	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
```

- [x] **步骤 5：验证类型检查**

运行：`cd packages/sfu-client && npx tsc --noEmit`
预期：无类型错误

- [x] **步骤 6：提交**

```bash
git add packages/sfu-client/src/agora-client.ts
git commit -m "refactor(sfu-client): AgoraSFUClient uses JoinParams + isConnected + async destroy"
```

---

### 任务 11：更新 `srs-client.ts`

**文件：**
- 修改：`packages/sfu-client/src/srs-client.ts`

- [x] **步骤 1：将 `JoinParams` 添加到导入**

```typescript
import type { JoinParams, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **步骤 2：替换 `joinRoom` 签名和方法体**

将签名从：

```typescript
	async joinRoom(
		token: string,
		url: string,
		identity: string,
		room?: string,
		stream?: string,
		streamToken?: string,
	): Promise<void> {
```

替换为：

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity, room, stream, streamToken } = params;
```

保持方法体其余部分不变。

- [x] **步骤 3：添加 `isConnected` 方法**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **步骤 4：更改 `destroy()` 为 `async`**

```typescript
	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
```

- [x] **步骤 5：验证类型检查**

运行：`cd packages/sfu-client && npx tsc --noEmit`
预期：无类型错误

- [x] **步骤 6：提交**

```bash
git add packages/sfu-client/src/srs-client.ts
git commit -m "refactor(sfu-client): SRSSFUClient uses JoinParams + isConnected + async destroy"
```

---

### 任务 12：更新 `daily-client.ts`

**文件：**
- 修改：`packages/sfu-client/src/daily-client.ts`

- [x] **步骤 1：将 `JoinParams` 添加到导入**

```typescript
import type { JoinParams, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **步骤 2：替换 `joinRoom` 签名和方法体**

将签名从：

```typescript
	async joinRoom(
		token: string,
		url: string,
		identity: string,
		room?: string,
	): Promise<void> {
		if (!this.callObject) this.callObject = DailyIframe.createCallObject();
		const resolvedURL = this.resolveRoomURL(url, room);
		await this.callObject.join({
			url: resolvedURL,
			token,
			userName: identity,
		});
		this.callObject.setLocalAudio(true);
		this.hasJoined = true;
	}
```

替换为：

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: url, identity, room } = params;
		if (!this.callObject) this.callObject = DailyIframe.createCallObject();
		const resolvedURL = this.resolveRoomURL(url, room);
		await this.callObject.join({
			url: resolvedURL,
			token,
			userName: identity,
		});
		this.callObject.setLocalAudio(true);
		this.hasJoined = true;
	}
```

- [x] **步骤 3：添加 `isConnected` 方法**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **步骤 4：将 `destroy()` 改为 `async`**

```typescript
	async destroy(): Promise<void> {
		await this.leaveRoom();
	}
```

- [x] **步骤 5：验证类型检查**

运行：`cd packages/sfu-client && npx tsc --noEmit`
预期：仅 mediasoup-client.ts 中有类型错误

- [x] **步骤 6：提交**

```bash
git add packages/sfu-client/src/daily-client.ts
git commit -m "refactor(sfu-client): DailySFUClient uses JoinParams + isConnected + async destroy"
```

---

### 任务 13：更新 `mediasoup-client.ts`

**文件：**
- 修改：`packages/sfu-client/src/mediasoup-client.ts`

- [x] **步骤 1：将 `JoinParams` 添加到导入**

```typescript
import type { JoinParams, RemoteAudioTrackLike, RemoteTrackInfo, SFUClient, SFUClientOptions } from "./types";
```

- [x] **步骤 2：替换 `joinRoom` 签名和方法体**

将方法签名从：

```typescript
	async joinRoom(
		token: string,
		_url: string,
		identity: string,
		room?: string,
	): Promise<void> {
		if (!this.socket) throw new Error("mediasoup client requires a socket.io socket");

		const [tokenRoom, tokenIdentity] = token.split(":", 2);
		this.roomId = room || tokenRoom;
		this.identity = tokenIdentity || identity;
```

替换为：

```typescript
	async joinRoom(params: JoinParams): Promise<void> {
		const { token, serverUrl: _url, identity, room } = params;
		if (!this.socket) throw new Error("mediasoup client requires a socket.io socket");

		const [tokenRoom, tokenIdentity] = token.split(":", 2);
		this.roomId = room || tokenRoom;
		this.identity = tokenIdentity || identity;
```

保持方法体其余部分不变。

- [x] **步骤 3：添加 `isConnected` 方法**

```typescript
	isConnected(): boolean {
		return this.hasJoined;
	}
```

- [x] **步骤 4：`destroy()` 已经是 `async` — 无需更改**

- [x] **步骤 5：验证类型检查通过**

运行：`cd packages/sfu-client && npx tsc --noEmit`
预期：无类型错误

- [x] **步骤 6：运行 sfu-client 测试**

运行：`cd packages/sfu-client && npx vitest run`
预期：全部 PASS

- [x] **步骤 7：提交**

```bash
git add packages/sfu-client/src/mediasoup-client.ts
git commit -m "refactor(sfu-client): MediaSoupSFUClient uses JoinParams + isConnected"
```

---

### 任务 14：更新调用方 `useRoomJoinSession.ts`

**文件：**
- 修改：`app/web/src/components/room/hooks/useRoomJoinSession.ts`

- [x] **步骤 1：将 `JoinParams` 添加到从 `@gospeak/sfu-client/types` 的导入中**

更新现有的导入行，包含 `JoinParams`：

```typescript
import type { SFUClient, JoinParams } from "@gospeak/sfu-client/types";
```

- [x] **步骤 2：替换 `joinRoom` 调用**

找到现有的调用（大约在 200 行左右）：

```typescript
						await raceAbort(
							createdClient.joinRoom(
								data.token,
								sessionMeta.connectTarget,
								data.identity,
								data.room,
								data.stream,
								data.streamToken,
							),
							signal,
						);
```

替换为：

```typescript
						const joinParams: JoinParams = {
							token: data.token,
							serverUrl: sessionMeta.connectTarget,
							identity: data.identity,
							room: data.room,
							stream: data.stream,
							streamToken: data.streamToken,
						};
						await raceAbort(
							createdClient.joinRoom(joinParams),
							signal,
						);
```

- [x] **步骤 3：验证 web 类型检查**

运行：`cd app/web && npx tsc --noEmit`
预期：无类型错误

- [x] **步骤 4：提交**

```bash
git add app/web/src/components/room/hooks/useRoomJoinSession.ts
git commit -m "refactor(web): use JoinParams for SFUClient.joinRoom()"
```

---

## 最终验证

### 任务 15：完整构建和测试覆盖

- [x] **步骤 1：运行 Go 测试**

运行：`cd app/server && go test ./...`
预期：全部 PASS

- [x] **步骤 2：运行 sfu-client 类型检查和测试**

运行：`cd packages/sfu-client && npx tsc --noEmit && npx vitest run`
预期：无类型错误，全部测试 PASS

- [x] **步骤 3：运行 web 类型检查**

运行：`cd app/web && npx tsc --noEmit`
预期：无类型错误

- [x] **步骤 4：提交（如有任何剩余修复）**

```bash
git add -A
git commit -m "chore: final verification fixes for SFU adapter refactor"
```

---

## 不包含范围

以下来自架构审查的项特意排除在本计划之外：

- **P3：`SFUSignalHandler` 信号库解耦** — 将 `RegisterRoutes(server *socketio.Server)` 替换为抽象的 `SignalConnection` 接口是一个更大的架构变更，会触及信号 hub、mediasoup 信号处理器，并需要一个新的适配器层。在阶段 1 和阶段 2 稳定后，可以作为单独的计划推进。当前没有功能因保持 socket.io 耦合而被阻塞。
