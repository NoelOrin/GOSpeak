> **Execution order superseded by** [`2026-07-16-bot-platform-unified.md`](./2026-07-16-bot-platform-unified.md). Keep this file as detailed appendix; do not run in parallel as a second source of truth.

# Bot Capability Router + Command Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `@gospeak/bot` 通过后端真实能力执行踢人/建房/列表/禁言，并补上最小 **Socket 文本桥**，使现有命令插件与欢迎插件真正可运行。

**Architecture:** 按 AstrBot 的 Context 模式，把插件对后端的访问收敛到 Capability Router。房间列表/建房/用户/服务端禁言走既有业务 REST；**房内互动（命令、发言、踢人）只走 Socket 信令**。Hub 提供 `bot:command`（入）与 `bot:message`（出）；踢人统一 `room:kick`。Bot Runtime 入房驻留后收发事件，不引入 bot 专用 REST 桥，不引入完整 IM 存储。

**Tech Stack:** Go (Gin + Socket.IO hub), TypeScript (`packages/bot`), vitest, go test

---

## Scope

### In Scope
1. Capability Router 纠偏（踢人 / 建房 / 列表 / 成员 / 服务端禁言 / 文本桥）
2. 事件适配修正（joined / left / kicked / user muted）
3. 后端最小文本桥：**仅 Socket**
   - `bot:command`（命令/文本注入）
   - `bot:message`（bot 发言）
   - 踢人继续 `room:kick`（不新增 `/bot/kick` REST）
4. 内置插件改走正确能力（moderation / room-manager / welcome / mute-manager）
5. 权限：Bot 建房可选 `room:create`；插件权限检查改为可识别 claims permcode
6. Capability Router 文本/踢人默认走 Socket（不再做 rest/auto 运输层切换）

### Out of Scope
- **Bot 专用 REST 文本桥**（`/api/v1/bot/command|message|kick`）——已明确不做
- 完整聊天存储 / 历史消息 / 前端聊天气泡 UI
- Bot Runtime 管理面（install/reload 的后端 Admin API）
- SFU 旁听 / ASR（已有独立计划）
- Go `internal/plugin` 与 TS bot 插件体系统一

---

## Current Gaps (must fix)

| 插件/API 调用 | 现状 | 后端真相 |
|---|---|---|
| `POST /api/v1/chat/send` | bot 调用 | **不存在** |
| `POST /api/v1/sfu/remove-participant` | bot 踢人 | **不存在**；应为 Socket `room:kick` |
| `POST /api/v1/sfu/mute` | bot 房间静音 | **不存在** |
| `createRoom` → `/signal/token` | 误用 | 应为 `POST /room/create` |
| `listRooms` 读 `data.rooms` | 错 | `data` 是 `[]RoomSummary` |
| `getMembers` 读 `data.participants` + role | 错 | `data` 是 `[]ParticipantSummary{identity,joinedAt}` |
| 命令插件依赖 `AdapterMessage` | 无来源 | 需 `bot:command` 桥 |

---

## File Map

### Create
- `packages/bot/src/runtime/capabilityRouter.ts` — 把 REST + Socket 组装成 `ChatClient`/`RoomClient`/`VoiceClient`
- `packages/bot/src/runtime/capabilityRouter.test.ts`
- `packages/bot/src/runtime/eventAdapter.ts` — 原始 socket payload → 标准 `BotEvent`（可从 socketClient 抽出）
- `packages/bot/src/runtime/eventAdapter.test.ts`
- `app/server/internal/signal/bot_bridge.go` — Hub 文本桥方法（仅 Socket 使用，便于单测）
- `app/server/internal/signal/bot_bridge_test.go` — Socket 桥测试
- `packages/bot/src/runtime/socketClient.command.test.ts` — 收 `bot:command` 转 `AdapterMessage`

### Modify
- `app/server/internal/signal/events.go` — 新增事件常量
- `app/server/internal/signal/hub.go` — 注册/处理 `bot:command`、`bot:message`；kick 逻辑可抽到 `bot_bridge.go`
- `app/server/internal/model/permission.go` — Bot 白名单增加 `room:create`（仅当保留建房命令时）
- `packages/bot/src/core/types.ts` — 事件类型拆分
- `packages/bot/src/core/context.ts` — Chat/Voice/权限接口微调
- `packages/bot/src/runtime/apiClient.ts` — 只保留真实 REST
- `packages/bot/src/runtime/socketClient.ts` — kick/message/command 监听
- `packages/bot/src/runtime/botRunner.ts` — 注入 Capability Router
- `packages/bot/src/filters/permissionFilter.ts` — 支持 permcode 或保留 level 但文档化
- `packages/bot/src/plugins/builtin/moderation/index.ts`
- `packages/bot/src/plugins/builtin/room-manager/index.ts`
- `packages/bot/src/plugins/builtin/welcome/index.ts`
- `packages/bot/src/plugins/builtin/mute-manager/index.ts`
- `packages/bot/README.md` — 契约与用法

---

## Protocol Spec

### Socket: `bot:command`（Client → Server → Room）

请求（任意已入房的已鉴权连接可发）：

```json
{
  "room": "lobby",
  "text": "/kick alice"
}
```

服务端校验：
1. JWT 已鉴权（`claimsIdentity` 非空）
2. 发送者在该 room 的 `Members` 中
3. `text` 非空，trim 后长度 ≤ 500

服务端广播到房间（含发送者）：

```json
{
  "room": "lobby",
  "text": "/kick alice",
  "from": {
    "identity": "admin",
    "displayName": "Admin",
    "role": "admin"
  },
  "messageId": "uuid",
  "timestamp": 1710000000000
}
```

Bot Runtime 收到后转为：

```ts
{
  eventType: EventType.AdapterMessage,
  messageId,
  room: { id: room, name: room },
  sender: { identity, name: displayName, role },
  content: text,
  isCommand: false, // CommandFilter 再解析
  timestamp
}
```

同时再 dispatch 一份 `OnMessageReceived`（content 相同），供 keyword-reply 使用。

### Socket: `bot:message`（Bot/Client → Server → Room）

请求：

```json
{
  "room": "lobby",
  "content": "欢迎 alice",
  "replyTo": "alice" // optional
}
```

服务端校验同 `bot:command`（必须在房内）。  
广播事件名：`bot:message`，payload 同结构 + `content` + optional `replyTo`。

> 前端可不渲染；本阶段只保证 bot 插件 `ctx.chat.send/reply` 有真实出口，集成测试可断言 socket 广播。

### 明确不做：Bot 专用 REST 桥

以下接口 **不在本计划范围**（用户已确认不要 RESTful 文本/踢人桥）：

- `POST /api/v1/bot/command`
- `POST /api/v1/bot/message`
- `POST /api/v1/bot/kick`

房内主动/被动互动统一 Socket。业务资源（房间 CRUD、用户查询、服务端禁言记录）仍用现有 REST，那是资源 API，不是 bot 文本桥。

### REST 契约（仅既有业务资源，非 bot 文本桥）

| 能力 | Method | Path | Body / Query | 成功 `data` |
|---|---|---|---|---|
| 列表房间 | GET | `/api/v1/signal/rooms` | — | `[{ name, memberCount? }]` |
| 列表成员 | GET | `/api/v1/signal/participants?room=` | query `room` | `[{ identity, joinedAt? }]` |
| 建房 | POST | `/api/v1/room/create` | `{ name, limit?, description? }` | `Room{ id, uuid, name, ... }` |
| 用户查询 | POST | `/api/v1/user/info` | `{ identity }` | `User` |
| 禁言 | POST | `/api/v1/mute/create` | `{ user_id, duration, permanent, reason }` | `Mute` |
| 解禁 | POST | `/api/v1/mute/cancel` | `{ user_id }` | `null` |
| 禁言列表 | POST | `/api/v1/mute/list` | — | `Mute[]` |
| SFU token | POST | `/api/v1/signal/token` | `{ room, password? }` | `{ token, serverUrl, stream?, ... }` identity 由 JWT 强制 |

### Socket 契约（Voice / Room presence）

| 能力 | Event | Payload |
|---|---|---|
| 加入 | `room:join` | `{ room, identity }` identity 服务端会忽略伪造，以 JWT 为准 |
| SFU 加入 | `room:join:sfu` | `{ room, identity, stream? }` + ack |
| 离开 | `room:leave` | `{ room }` |
| 踢人 | `room:kick` | `{ room, targetIdentity }` 需 `signal:kick` |
| 文本入 | `bot:command` | 房内命令/文本注入（唯一入口） |
| 文本出 | `bot:message` | bot/用户轻量发言（唯一出口） |
| 踢人 | `room:kick` | 唯一踢人入口 |

---

## Event Type Mapping (Bot)

| 后端事件 | Bot `EventType` | 备注 |
|---|---|---|
| `member:joined` | `OnMemberJoined` | **新增**；welcome 监听它 |
| `member:left` | `OnMemberLeft` | **新增** |
| `room:kicked` | `OnMemberKicked` | **新增**；勿再映射成 OnRoomLeft |
| `member:updated` | `OnMemberStateChanged` | mic 状态；`muted = isMicMuted` |
| `user:muted` | `OnUserMuted` | **新增**；member.identity 用查表或 `user:<id>` 前缀，payload 保留 `userId` |
| `user:unmuted` | `OnUserUnmuted` | **新增** |
| `bot:command` | `AdapterMessage` + `OnMessageReceived` | 命令/关键词入口 |
| `room:created` | `OnRoomCreated` | 保持 |
| `room:updated` | `OnRoomUpdated` | 保持 |
| self join ack | `OnRoomJoined` | 仅表示 bot 自己加入成功时使用（可选） |

保留旧名 `OnRoomJoined`/`OnRoomLeft` 一个小版本：  
- `OnRoomJoined` = bot 自己加入成功  
- 成员进出改用 `OnMemberJoined`/`OnMemberLeft`  
- 更新 builtin welcome/room-manager 监听新事件

---

### Task 1: 扩展 Bot 事件类型

**Files:**
- Modify: `packages/bot/src/core/types.ts`
- Modify: `packages/bot/src/core/index.ts`（若需导出新类型）
- Test: `packages/bot/src/core/types` 由后续 adapter 测试覆盖；本任务先改类型并保证 `pnpm test` 不因类型编译挂掉

- [ ] **Step 1: 更新 `EventType` 与 payload 类型**

在 `packages/bot/src/core/types.ts` 中改为（完整替换枚举与相关 interface，保持旧枚举值字符串兼容处见注释）：

```ts
export enum EventType {
	OnBotLoaded = "OnBotLoaded",
	AdapterMessage = "AdapterMessage",
	OnMessageReceived = "OnMessageReceived",
	OnMessageSent = "OnMessageSent",
	OnRoomCreated = "OnRoomCreated",
	OnRoomJoined = "OnRoomJoined", // bot 自己加入成功
	OnRoomUpdated = "OnRoomUpdated",
	OnRoomLeft = "OnRoomLeft", // bot 自己离开
	OnMemberJoined = "OnMemberJoined",
	OnMemberLeft = "OnMemberLeft",
	OnMemberKicked = "OnMemberKicked",
	OnMemberStateChanged = "OnMemberStateChanged",
	OnUserMuted = "OnUserMuted",
	OnUserUnmuted = "OnUserUnmuted",
	OnPluginLoaded = "OnPluginLoaded",
	OnPluginUnloaded = "OnPluginUnloaded",
	OnPluginError = "OnPluginError",
}

export type PermissionLevel =
	| "owner"
	| "admin"
	| "moderator"
	| "member"
	| "guest";

export interface RoomRef {
	id: string;
	name: string;
}

export interface MemberRef {
	identity: string;
	name: string;
	role: PermissionLevel;
}

export interface MessageEvent {
	eventType: EventType.AdapterMessage | EventType.OnMessageReceived | EventType.OnMessageSent;
	messageId: string;
	room: RoomRef;
	sender: MemberRef;
	content: string;
	rawCommand?: ParsedCommand;
	isCommand: boolean;
	replyTo?: string;
	timestamp: number;
}

export interface ParsedCommand {
	name: string;
	args: string[];
	raw: string;
	alias?: string;
}

export interface RoomEvent {
	eventType:
		| EventType.OnRoomCreated
		| EventType.OnRoomJoined
		| EventType.OnRoomUpdated
		| EventType.OnRoomLeft
		| EventType.OnMemberJoined
		| EventType.OnMemberLeft
		| EventType.OnMemberKicked;
	room: RoomRef;
	actor?: MemberRef;
	timestamp: number;
}

export interface MemberStateEvent {
	eventType: EventType.OnMemberStateChanged;
	room: RoomRef;
	member: MemberRef;
	muted: boolean;
	volume?: number;
	timestamp: number;
}

export interface UserMuteEvent {
	eventType: EventType.OnUserMuted | EventType.OnUserUnmuted;
	userId: number;
	member: MemberRef; // identity 暂用 String(userId) 若未知用户名
	duration?: number;
	permanent?: boolean;
	reason?: string;
	expiresAt?: string;
	timestamp: number;
}

export interface PluginErrorEvent {
	eventType: EventType.OnPluginError;
	pluginName: string;
	handler: string;
	error: Error;
	timestamp: number;
}

export interface LifecycleEvent {
	eventType:
		| EventType.OnBotLoaded
		| EventType.OnPluginLoaded
		| EventType.OnPluginUnloaded
		| EventType.OnPluginError;
	pluginName?: string;
	timestamp: number;
}

export type BotEvent =
	| MessageEvent
	| RoomEvent
	| MemberStateEvent
	| UserMuteEvent
	| PluginErrorEvent
	| LifecycleEvent;

export function createBotEvent(
	eventType: LifecycleEvent["eventType"],
	pluginName?: string,
): LifecycleEvent {
	return {
		eventType,
		pluginName,
		timestamp: Date.now(),
	};
}
```

- [ ] **Step 2: 跑 bot 测试看类型破坏面**

Run:

```bash
cd packages/bot && pnpm test
```

Expected: 可能有插件/测试因事件名仍用旧语义失败；先记录失败列表，后续任务修。不要在本任务大改插件。

- [ ] **Step 3: Commit**

```bash
git add packages/bot/src/core/types.ts packages/bot/src/core/index.ts
git commit -m "feat(bot): split member/user mute event types"
```

---

### Task 2: 后端 Socket 文本桥（不做 REST）

**Files:**
- Create: `app/server/internal/signal/bot_bridge.go`
- Create: `app/server/internal/signal/bot_bridge_test.go`
- Modify: `app/server/internal/signal/events.go`
- Modify: `app/server/internal/signal/hub.go`

- [ ] **Step 1: 写失败测试**

`bot_bridge_test.go`：

```go
func TestPublishBotCommand_Broadcasts(t *testing.T) { /* 在房成员 → 广播 bot:command */ }
func TestPublishBotCommand_NotInRoom(t *testing.T)  { /* 不在房 → 无广播 */ }
func TestPublishBotMessage_Broadcasts(t *testing.T) { /* content + replyTo */ }
func TestOnBotCommand_SocketHandler(t *testing.T)   { /* OnBotCommand 解析 JSON 后广播 */ }
```

复用 `hub_test.go` / `hub_kick_test.go` mock。

- [ ] **Step 2: 跑测试确认失败**

```bash
cd app/server && go test ./internal/signal -run 'TestPublishBot|TestOnBotCommand|TestOnBotMessage' -count=1
```

Expected: FAIL

- [ ] **Step 3: 事件常量**

```go
EventBotCommand = "bot:command"
EventBotMessage = "bot:message"
```

- [ ] **Step 4: `bot_bridge.go` 核心方法（仅供 Socket handler 调用）**

```go
type BotTextResult struct {
	MessageID string
	Room      string
	Timestamp int64
}

func (h *Hub) PublishBotCommand(room, callerIdentity, displayName, role, text string) (*BotTextResult, error)
func (h *Hub) PublishBotMessage(room, callerIdentity, displayName, role, content, replyTo string) (*BotTextResult, error)
```

规则：
- caller 必须在 `h.rooms[room].Members`（按 identity 或 socket 查找）
- text/content trim，空或 >500 → error
- 广播 payload：

```go
map[string]interface{}{
  "room": room,
  "messageId": uuid.NewString(),
  "timestamp": time.Now().UnixMilli(),
  "from": map[string]interface{}{
    "identity": callerIdentity,
    "displayName": displayName,
    "role": role,
  },
  // command: "text"
  // message: "content", optional "replyTo"
}
```

- [ ] **Step 5: Socket handler + 注册**

```go
server.OnEvent("/", EventBotCommand, safeOnEventData(h.OnBotCommand))
server.OnEvent("/", EventBotMessage, safeOnEventData(h.OnBotMessage))
```

```go
func (h *Hub) OnBotCommand(s socketio.Conn, data string) {
	// parse {room,text}; identity from JWT claims; PublishBotCommand
}
func (h *Hub) OnBotMessage(s socketio.Conn, data string) {
	// parse {room,content,replyTo?}; PublishBotMessage
}
```

踢人：**不**新增 REST；保持/整理现有 `OnRoomKick` → 可选抽 `Hub.Kick(...)` 仅给 Socket 用。

- [ ] **Step 6: 跑测试通过**

```bash
cd app/server && go test ./internal/signal -run 'TestPublishBot|TestOnBotCommand|TestOnBotMessage|TestOnRoomKick' -count=1
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add app/server/internal/signal/events.go \
  app/server/internal/signal/hub.go \
  app/server/internal/signal/bot_bridge.go \
  app/server/internal/signal/bot_bridge_test.go
git commit -m "feat(signal): add socket-only bot command/message bridge"
```

### Task 3: 修正 REST `apiClient` 契约

**Files:**
- Modify: `packages/bot/src/runtime/apiClient.ts`
- Create/Modify: `packages/bot/src/runtime/apiClient.test.ts`

- [ ] **Step 1: 写失败测试**

`packages/bot/src/runtime/apiClient.test.ts`：

```ts
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { GOSpeakApiClient } from "./apiClient";

const logger = { debug() {}, info() {}, warn() {}, error() {} };

describe("GOSpeakApiClient contracts", () => {
	const fetchMock = vi.fn();
	beforeEach(() => {
		vi.stubGlobal("fetch", fetchMock);
	});
	afterEach(() => {
		vi.unstubAllGlobals();
		fetchMock.mockReset();
	});

	it("listRooms parses array data", async () => {
		fetchMock.mockResolvedValue({
			json: async () => ({
				code: 0,
				msg: "ok",
				data: [{ name: "lobby", memberCount: 2 }],
			}),
		});
		const api = new GOSpeakApiClient({
			baseUrl: "http://localhost:8998",
			accessToken: "t",
			logger,
		});
		const rooms = await api.listRooms();
		expect(rooms).toEqual([{ id: "lobby", name: "lobby" }]);
		expect(fetchMock.mock.calls[0][0]).toContain("/api/v1/signal/rooms");
	});

	it("getMembers parses array data without role", async () => {
		fetchMock.mockResolvedValue({
			json: async () => ({
				code: 0,
				msg: "ok",
				data: [{ identity: "alice", joinedAt: 1 }],
			}),
		});
		const api = new GOSpeakApiClient({
			baseUrl: "http://x",
			accessToken: "t",
			logger,
		});
		const members = await api.getMembers("lobby");
		expect(members[0]).toMatchObject({
			identity: "alice",
			name: "alice",
			role: "member",
		});
	});

	it("createRoom posts /room/create", async () => {
		fetchMock.mockResolvedValue({
			json: async () => ({
				code: 0,
				msg: "ok",
				data: { id: 1, uuid: "u1", name: "lobby" },
			}),
		});
		const api = new GOSpeakApiClient({
			baseUrl: "http://x",
			accessToken: "t",
			logger,
		});
		const room = await api.createRoom("lobby", 10);
		expect(room).toEqual({ id: "u1", name: "lobby" });
		const [url, init] = fetchMock.mock.calls[0];
		expect(String(url)).toContain("/api/v1/room/create");
		expect(JSON.parse(init.body)).toMatchObject({ name: "lobby", limit: 10 });
	});
});
```

- [ ] **Step 2: 跑测试失败**

Run:

```bash
cd packages/bot && pnpm exec vitest run src/runtime/apiClient.test.ts
```

Expected: FAIL（当前解析/路径错误）

- [ ] **Step 3: 改 `apiClient.ts`**

关键实现要点：

```ts
// listRooms
const data = await this.request<Array<{ name: string; memberCount?: number }>>(
  "GET",
  "/api/v1/signal/rooms",
);
return (Array.isArray(data) ? data : []).map((r) => ({
  id: r.name,
  name: r.name,
}));

// getMembers
const data = await this.request<Array<{ identity: string; joinedAt?: number }>>(
  "GET",
  `/api/v1/signal/participants?room=${encodeURIComponent(roomId)}`,
);
return (Array.isArray(data) ? data : []).map((p) => ({
  identity: p.identity,
  name: p.identity,
  role: "member" as const,
}));

// createRoom
const data = await this.request<{ id: number; uuid: string; name: string }>(
  "POST",
  "/api/v1/room/create",
  { name, limit: limit ?? 0 },
);
return { id: data.uuid || String(data.id), name: data.name };

// 删除或标记废弃：send/reply 内对 /chat/send 的实现（改由 Capability Router + socket）
// muteMember/removeMember：改为 throw 明确错误，提示应走 CapabilityRouter/socket
async muteMember(): Promise<void> {
  throw new Error("room mic mute via HTTP is not supported; use server muteUser or signal path");
}
async removeMember(): Promise<void> {
  throw new Error("removeMember must use socket room:kick via CapabilityRouter");
}
```

保留：
- `muteUser` / `unmuteUser` / `listMutes` / `getMuteStatus`
- `getUserByIdentity` → `POST /user/info` `{ identity }`
- `getSFUToken` → `POST /signal/token` **不要传伪造 identity**（后端会覆盖）；body 仅 `{ room, password? }`

```ts
async getSFUToken(room: string, _identity?: string, password?: string) {
  return this.request("POST", "/api/v1/signal/token", {
    room,
    ...(password ? { password } : {}),
  });
}

// 不做 postBotCommand/postBotMessage/postBotKick。
// 文本与踢人一律 Socket：socketClient.sendBotMessage / kickMember。
```

- [ ] **Step 4: 测试通过**

Run:

```bash
cd packages/bot && pnpm exec vitest run src/runtime/apiClient.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/bot/src/runtime/apiClient.ts packages/bot/src/runtime/apiClient.test.ts
git commit -m "fix(bot): align apiClient with real backend REST contracts"
```

---

### Task 4: 实现 Capability Router

**Files:**
- Create: `packages/bot/src/runtime/capabilityRouter.ts`
- Create: `packages/bot/src/runtime/capabilityRouter.test.ts`
- Modify: `packages/bot/src/core/context.ts`（如需扩展 VoiceClient 注释/server mute）
- Modify: `packages/bot/src/runtime/botRunner.ts`
- Modify: `packages/bot/src/runtime/index.ts`

- [ ] **Step 1: 写失败测试**

```ts
// capabilityRouter.test.ts
import { describe, expect, it, vi } from "vitest";
import { createCapabilityRouter } from "./capabilityRouter";

describe("createCapabilityRouter", () => {
	it("removeMember emits room:kick via socket", async () => {
		const kickMember = vi.fn();
		const sendBotMessage = vi.fn(async () => {});
		const api = {
			// minimal stubs
			listRooms: vi.fn(),
			getMembers: vi.fn(),
			createRoom: vi.fn(),
			muteUser: vi.fn(),
			unmuteUser: vi.fn(),
			getUserByIdentity: vi.fn(),
			getSFUToken: vi.fn(),
		} as any;
		const socket = { kickMember, sendBotMessage } as any;
		const caps = createCapabilityRouter({ api, socket, logger: console });
		await caps.voice.removeMember("lobby", "alice");
		expect(kickMember).toHaveBeenCalledWith("lobby", "alice");
	});

	it("chat.send uses socket bot:message", async () => {
		const sendBotMessage = vi.fn(async () => {});
		const caps = createCapabilityRouter({
			api: {} as any,
			socket: { kickMember: vi.fn(), sendBotMessage, isConnected: true } as any,
			logger: console,
			roomsExtra: { join: async () => {}, leave: () => {}, joined: () => [] },
		});
		await caps.chat.send("lobby", "hi");
		expect(sendBotMessage).toHaveBeenCalledWith("lobby", "hi", undefined);
	});

	it("removeMember uses socket room:kick", async () => {
		const kickMember = vi.fn();
		const caps = createCapabilityRouter({
			api: {} as any,
			socket: { kickMember, sendBotMessage: vi.fn(), isConnected: true } as any,
			logger: console,
			roomsExtra: { join: async () => {}, leave: () => {}, joined: () => [] },
		});
		await caps.voice.removeMember("lobby", "alice");
		expect(kickMember).toHaveBeenCalledWith("lobby", "alice");
	});

	it("chat.reply passes replyTo", async () => {
		const sendBotMessage = vi.fn(async () => {});
		const caps = createCapabilityRouter({
			api: {} as any,
			socket: { kickMember: vi.fn(), sendBotMessage } as any,
			logger: console,
		});
		await caps.chat.reply(
			{
				room: { id: "lobby", name: "lobby" },
				sender: { identity: "alice", name: "Alice", role: "member" },
			} as any,
			"welcome",
		);
		expect(sendBotMessage).toHaveBeenCalledWith("lobby", "welcome", "alice");
	});
});
```

- [ ] **Step 2: 跑测试失败**

Run:

```bash
cd packages/bot && pnpm exec vitest run src/runtime/capabilityRouter.test.ts
```

Expected: FAIL module not found

- [ ] **Step 3: 实现 `capabilityRouter.ts`**

```ts
import type {
	BotContext,
	ChatClient,
	Logger,
	RoomClient,
	VoiceClient,
} from "../core/context";
import type { GOSpeakApiClient } from "./apiClient";
import type { GOSpeakSocketClient } from "./socketClient";

export interface CapabilityRouterDeps {
	api: GOSpeakApiClient;
	socket: GOSpeakSocketClient;
	logger: Logger;
	/** join/leave 由 runner 注入，避免循环依赖 */
	roomsExtra: Pick<RoomClient, "join" | "leave" | "joined">;
}

export interface CapabilityClients {
	chat: ChatClient;
	rooms: RoomClient;
	voice: VoiceClient;
}

export function createCapabilityRouter(
	deps: CapabilityRouterDeps,
): CapabilityClients {
	const { api, socket, logger, roomsExtra } = deps;

	const chat: ChatClient = {
		async send(roomId, content) {
			await socket.sendBotMessage(roomId, content);
		},
		async reply(event, content) {
			await socket.sendBotMessage(
				event.room.id,
				content,
				event.sender.identity,
			);
		},
	};

	const rooms: RoomClient = {
		listRooms: () => api.listRooms(),
		getMembers: (id) => api.getMembers(id),
		createRoom: (name, limit) => api.createRoom(name, limit),
		join: roomsExtra.join,
		leave: roomsExtra.leave,
		joined: roomsExtra.joined,
	};

	const voice: VoiceClient = {
		async muteMember(roomId, identity, muted) {
			// 房间 mic 软提示：无硬 API 时，降级为 server mute/unmute 需 user id
			// MVP：解析 identity → user 后走 muteUser；unmute 同理
			const user = await api.getUserByIdentity(identity);
			if (muted) {
				await api.muteUser(user.id ?? user.ID, 0, true, "bot-mute");
			} else {
				await api.unmuteUser(user.id ?? user.ID);
			}
			logger.info(
				`voice.muteMember degraded to server mute: ${identity} muted=${muted} room=${roomId}`,
			);
		},
		async removeMember(roomId, identity) {
			socket.kickMember(roomId, identity);
		},
		async setMemberVolume() {
			logger.warn("setMemberVolume is client-local only");
		},
	};

	return { chat, rooms, voice };
}
```

> `getUserByIdentity` 返回类型在 apiClient 中统一为 camelCase：`{ id: number; name: string; role: string; uuid: string }`，在 apiClient 内把后端 `ID`/`Name` 映射掉，避免插件关心 Go json 大小写。

- [ ] **Step 4: `socketClient` 增加 `sendBotMessage`**

```ts
sendBotMessage(room: string, content: string, replyTo?: string): Promise<void> {
  if (!this.connected || !this.socket) {
    return Promise.reject(new Error("socket not connected"));
  }
  this.socket.emit("bot:message", {
    room,
    content,
    ...(replyTo ? { replyTo } : {}),
  });
  return Promise.resolve();
}

sendBotCommand(room: string, text: string): void {
  this.socket?.emit("bot:command", { room, text });
}
```

- [ ] **Step 5: `botRunner.buildPluginCtx` 改用 Capability Router**

在 `start()` 里 socket/api 就绪后：

```ts
this.caps = createCapabilityRouter({
  api: this.api,
  socket: this.socket,
  logger: this.logger,
  roomsExtra: {
    join: (name, o) => this.joinRoom(name, o),
    leave: (name) => this.leaveRoom(name),
    joined: () => this.joinedRooms,
  },
});
```

`buildPluginCtx`：

```ts
return {
  logger: this.logger,
  config: this.config.pluginConfigs?.[pluginName] ?? {},
  pluginName,
  chat: this.caps.chat,
  rooms: this.caps.rooms,
  voice: this.caps.voice,
  kv: createKVStore(),
  hasPermission: () => true, // Task 6 再收紧
};
```

- [ ] **Step 6: 测试通过 + bot 单测**

Run:

```bash
cd packages/bot && pnpm exec vitest run src/runtime/capabilityRouter.test.ts src/runtime/apiClient.test.ts src/runtime/botRunner.test.ts
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add packages/bot/src/runtime/capabilityRouter.ts packages/bot/src/runtime/capabilityRouter.test.ts packages/bot/src/runtime/socketClient.ts packages/bot/src/runtime/botRunner.ts packages/bot/src/runtime/index.ts packages/bot/src/runtime/apiClient.ts
git commit -m "feat(bot): add capability router over real signal/REST"
```

---

### Task 5: Event Adapter — socket 原始事件 → 标准 BotEvent

**Files:**
- Create: `packages/bot/src/runtime/eventAdapter.ts`
- Create: `packages/bot/src/runtime/eventAdapter.test.ts`
- Modify: `packages/bot/src/runtime/socketClient.ts`

- [ ] **Step 1: 写 adapter 单测**

```ts
import { describe, expect, it } from "vitest";
import { adaptSocketEvent } from "./eventAdapter";
import { EventType } from "../core/types";

describe("adaptSocketEvent", () => {
	it("maps member:joined to OnMemberJoined", () => {
		const ev = adaptSocketEvent("member:joined", {
			room: "lobby",
			identity: "alice",
			id: "s1",
		});
		expect(ev?.eventType).toBe(EventType.OnMemberJoined);
		expect((ev as any).actor.identity).toBe("alice");
	});

	it("maps room:kicked to OnMemberKicked", () => {
		const ev = adaptSocketEvent("room:kicked", {
			room: "lobby",
			targetIdentity: "alice",
		});
		expect(ev?.eventType).toBe(EventType.OnMemberKicked);
		expect((ev as any).actor.identity).toBe("alice");
	});

	it("maps bot:command to AdapterMessage + exposes content", () => {
		const evs = adaptSocketEvent("bot:command", {
			room: "lobby",
			text: "/kick alice",
			messageId: "m1",
			timestamp: 1,
			from: { identity: "admin", displayName: "Admin", role: "admin" },
		});
		const list = Array.isArray(evs) ? evs : [evs];
		expect(list[0]?.eventType).toBe(EventType.AdapterMessage);
		expect((list[0] as any).content).toBe("/kick alice");
	});

	it("maps user:muted to OnUserMuted with userId", () => {
		const ev = adaptSocketEvent("user:muted", {
			user_id: 9,
			duration: 60,
			permanent: false,
			reason: "spam",
		});
		expect(ev?.eventType).toBe(EventType.OnUserMuted);
		expect((ev as any).userId).toBe(9);
	});
});
```

- [ ] **Step 2: 实现 `eventAdapter.ts`**

```ts
import {
	EventType,
	type BotEvent,
	type MemberRef,
	type MessageEvent,
	type PermissionLevel,
	type RoomEvent,
	type RoomRef,
	type UserMuteEvent,
	type MemberStateEvent,
} from "../core/types";

export function adaptSocketEvent(
	event: string,
	raw: any,
): BotEvent | BotEvent[] | null {
	switch (event) {
		case "member:joined":
			return roomMemberEvent(EventType.OnMemberJoined, raw);
		case "member:left":
			return roomMemberEvent(EventType.OnMemberLeft, raw);
		case "room:kicked":
			return {
				eventType: EventType.OnMemberKicked,
				room: roomRef(raw),
				actor: {
					identity: String(raw?.targetIdentity ?? ""),
					name: String(raw?.targetIdentity ?? ""),
					role: "member",
				},
				timestamp: Date.now(),
			} satisfies RoomEvent;
		case "member:updated":
			return {
				eventType: EventType.OnMemberStateChanged,
				room: roomRef(raw),
				member: memberRef(raw),
				muted: Boolean(raw?.isMicMuted),
				timestamp: Date.now(),
			} satisfies MemberStateEvent;
		case "user:muted":
			return userMuteEvent(EventType.OnUserMuted, raw);
		case "user:unmuted":
			return userMuteEvent(EventType.OnUserUnmuted, raw);
		case "bot:command": {
			const msg = commandToMessage(raw, EventType.AdapterMessage);
			const recv = { ...msg, eventType: EventType.OnMessageReceived as const };
			return [msg, recv];
		}
		case "bot:message":
			return commandToMessage(
				{ ...raw, text: raw?.content },
				EventType.OnMessageReceived,
			);
		case "room:created":
			return { eventType: EventType.OnRoomCreated, room: roomRef(raw), timestamp: Date.now() };
		case "room:updated":
			return { eventType: EventType.OnRoomUpdated, room: roomRef(raw), timestamp: Date.now() };
		default:
			return null;
	}
}

function roomRef(raw: any): RoomRef {
	if (typeof raw === "string") return { id: raw, name: raw };
	const name = String(raw?.name ?? raw?.room ?? "");
	const id = String(raw?.uuid ?? raw?.id ?? raw?.room ?? name);
	return { id, name };
}

function memberRef(raw: any): MemberRef {
	const identity = String(raw?.identity ?? raw?.targetIdentity ?? "");
	const name = String(raw?.name ?? raw?.displayName ?? identity);
	const role = (raw?.role ?? "member") as PermissionLevel;
	return { identity, name, role };
}

function roomMemberEvent(type: RoomEvent["eventType"], raw: any): RoomEvent {
	return {
		eventType: type,
		room: roomRef(raw),
		actor: memberRef(raw),
		timestamp: Date.now(),
	};
}

function userMuteEvent(
	type: UserMuteEvent["eventType"],
	raw: any,
): UserMuteEvent {
	const userId = Number(raw?.user_id ?? raw?.userId ?? 0);
	return {
		eventType: type,
		userId,
		member: {
			identity: String(userId),
			name: String(userId),
			role: "member",
		},
		duration: raw?.duration,
		permanent: raw?.permanent,
		reason: raw?.reason,
		expiresAt: raw?.expires_at ?? raw?.expiresAt,
		timestamp: Date.now(),
	};
}

function commandToMessage(raw: any, eventType: MessageEvent["eventType"]): MessageEvent {
	const from = raw?.from ?? {};
	return {
		eventType,
		messageId: String(raw?.messageId ?? `${Date.now()}`),
		room: roomRef(raw),
		sender: {
			identity: String(from.identity ?? ""),
			name: String(from.displayName ?? from.identity ?? ""),
			role: (from.role ?? "member") as PermissionLevel,
		},
		content: String(raw?.text ?? raw?.content ?? ""),
		isCommand: false,
		replyTo: raw?.replyTo ? String(raw.replyTo) : undefined,
		timestamp: Number(raw?.timestamp ?? Date.now()),
	};
}
```

- [ ] **Step 3: `socketClient.setupListeners` 改为调用 adapter**

对每个原始事件：

```ts
const adapted = adaptSocketEvent(eventName, raw);
if (!adapted) return;
const list = Array.isArray(adapted) ? adapted : [adapted];
for (const ev of list) this.emit(ev);
```

仍保留 `connect`/`disconnect` 连接状态逻辑。  
`room:left`（自己离开确认）→ `OnRoomLeft` 可继续在 socketClient 特判，或并入 adapter。

- [ ] **Step 4: 测试**

Run:

```bash
cd packages/bot && pnpm exec vitest run src/runtime/eventAdapter.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/bot/src/runtime/eventAdapter.ts packages/bot/src/runtime/eventAdapter.test.ts packages/bot/src/runtime/socketClient.ts
git commit -m "feat(bot): adapt signal events into typed bot events"
```

---

### Task 6: Bot 白名单与用户 API 归一化

**Files:**
- Modify: `app/server/internal/model/permission.go`
- Modify: `packages/bot/src/runtime/apiClient.ts`（User 字段映射）
- Test: `app/server` 若有 bot permission 测试则更新；否则加 service 单测不必强制

- [ ] **Step 1: Bot 白名单增加 `room:create`**

`BotScopedPermissions`：

```go
var BotScopedPermissions = []string{
	PermRoomRead,
	PermRoomCreate,
	PermUserRead,
	PermSignalKick,
	PermMuteManage,
}
```

- [ ] **Step 2: `getUserByIdentity` 归一化**

```ts
async getUserByIdentity(identity: string): Promise<{
  id: number;
  name: string;
  role: string;
  uuid: string;
}> {
  const raw = await this.request<any>("POST", "/api/v1/user/info", { identity });
  return {
    id: Number(raw.id ?? raw.ID ?? raw.Id),
    name: String(raw.name ?? raw.Name ?? identity),
    role: String(raw.role ?? raw.Role ?? "user"),
    uuid: String(raw.uuid ?? raw.UUID ?? ""),
  };
}
```

- [ ] **Step 3: Go 测试（权限常量无编译问题）**

Run:

```bash
cd app/server && go test ./internal/service -count=1
```

Expected: PASS（或仅与本改动无关的既有失败；本改动不应新增失败）

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/model/permission.go packages/bot/src/runtime/apiClient.ts
git commit -m "feat(bot): allow room:create for bots and normalize user info"
```

---

### Task 7: 更新内置插件监听与动作

**Files:**
- Modify: `packages/bot/src/plugins/builtin/welcome/index.ts`
- Modify: `packages/bot/src/plugins/builtin/moderation/index.ts`
- Modify: `packages/bot/src/plugins/builtin/room-manager/index.ts`
- Modify: `packages/bot/src/plugins/builtin/mute-manager/index.ts`
- Modify: `packages/bot/src/plugins/builtin/builtin.test.ts`

- [ ] **Step 1: welcome 监听 `OnMemberJoined`**

```ts
@On(EventType.OnMemberJoined, { desc: "欢迎新成员" })
async onMemberJoined(event: RoomEvent): Promise<void> {
  if (!this.enabled || !event.actor) return;
  // 忽略自己
  const text = this.template.replaceAll("{name}", event.actor.name);
  await this.ctx.chat.send(event.room.name || event.room.id, text);
}
```

- [ ] **Step 2: moderation 踢人走 `voice.removeMember`（已是 socket）**

保持 `/kick` 调用 `this.ctx.voice.removeMember`。  
`/mute` `/unmute`：改为 identity → 服务端禁言：

```ts
const user = await (this.ctx as any). /* 不要 any */ ;
```

更干净：在 `BotContext` 增加可选 `users` 或让 moderation 用 rooms+既有 api。  
**本计划选择：** 扩展 `BotContext`：

```ts
// context.ts
export interface UserClient {
  getByIdentity(identity: string): Promise<{ id: number; name: string; role: string; uuid: string }>;
}
export interface BotContext {
  // ...
  readonly users: UserClient;
}
```

Capability Router / buildPluginCtx：

```ts
users: {
  getByIdentity: (id) => this.api.getUserByIdentity(id),
},
```

moderation：

```ts
const user = await this.ctx.users.getByIdentity(target);
await this.ctx.voice.muteMember(event.room.id, target, true);
// muteMember 内部已 degrade 到 server mute
```

- [ ] **Step 3: room-manager**

- `OnMemberJoined` / `OnMemberLeft` 替换旧事件
- `create` 走 `ctx.rooms.createRoom`（已指向 `/room/create`）
- 创建失败时把后端 `FORBIDDEN` 原文回复到 `bot:message`

- [ ] **Step 4: mute-manager**

确认全部走 `api.muteUser` 路径（通过 ctx 扩展或继续用 voice + users）。  
命令仍由 `AdapterMessage` 触发。

- [ ] **Step 5: 更新 `builtin.test.ts`**

- boot 后 dispatch `OnMemberJoined` 测 welcome
- dispatch `AdapterMessage` content `/kick bob` 且 mock `voice.removeMember`
- 不再依赖不存在的 HTTP

- [ ] **Step 6: 全量 bot 测试**

Run:

```bash
cd packages/bot && pnpm test
```

Expected: PASS 全部

- [ ] **Step 7: Commit**

```bash
git add packages/bot/src/core/context.ts packages/bot/src/runtime/botRunner.ts packages/bot/src/plugins/builtin packages/bot/src/runtime/capabilityRouter.ts
git commit -m "fix(bot): point builtin plugins at real capabilities and events"
```

---

### Task 8: 端到端验收脚本与文档

**Files:**
- Modify: `packages/bot/README.md`
- Create: `packages/bot/scripts/smoke-capability.mjs`（可选，tsx 也可）

- [ ] **Step 1: README 增加「后端契约」章节**

写明：
1. 创建 bot 时 permissions 建议：

```json
["room:read","room:create","user:read","signal:kick","mute:manage"]
```

2. 启动后 bot 必须 `join` 目标房间，才能收 `bot:command` / 发 `bot:message`
3. 管理端发命令（**仅 Socket**，须先 join 房间）：

```js
socket.emit("bot:command", { room: "lobby", text: "/kick alice" });
```

4. Bot 发消息：

```js
socket.emit("bot:message", { room: "lobby", content: "欢迎加入" });
// 插件侧：await ctx.chat.send("lobby", "欢迎加入")
```

5. 明确 **不是完整 IM**：无历史、无 bot REST 文本桥；前端可不展示 `bot:message`

- [ ] **Step 2: 手动验收清单（写进 README）**

```bash
# terminal A: server
cd app/server && go run . server

# terminal B: bot
cd packages/bot
export GOSPEAK_SERVER_URL=http://localhost:8998
export GOSPEAK_TOKEN=<bot jwt>
export GOSPEAK_PLUGIN_DIR=./plugins
pnpm start
```

验收步骤：
1. Bot 日志显示 connected
2. 代码或插件执行 `rooms.join("lobby")`
3. 管理员入同一房间，Socket emit `bot:command`：`/kick <user>`
4. 目标用户收到 `room:kicked`
5. emit `bot:command`：`/mute <user>` 后 DB mutes 有记录
6. 新成员 join → 房间收到 Socket `bot:message` 欢迎语
7. 往 `pluginDir` 丢新插件文件 → 热加载成功（回归）

- [ ] **Step 3: 跑后端相关测试 + bot 测试**

Run:

```bash
cd app/server && go test ./internal/signal ./internal/service -count=1
cd packages/bot && pnpm test
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add packages/bot/README.md packages/bot/scripts/smoke-capability.mjs
git commit -m "docs(bot): document capability router and command bridge"
```

---

## Test Matrix

| 层级 | 命令 | 覆盖 |
|---|---|---|
| Go signal | `go test ./internal/signal -run Bot -count=1` | command/message 广播与拒收 |
| Go kick 回归 | `go test ./internal/signal -run Kick -count=1` | bot claims 踢人仍可用 |
| Bot unit | `pnpm test` in `packages/bot` | api 契约、router、adapter、builtin |
| Manual | README smoke | join/kick/mute/welcome/hot-load |

---

## Self-Review

### Spec coverage
- Capability Router 纠偏 → Task 3–4
- 事件适配 → Task 1、5
- 最小 command 入口（Socket only）→ Task 2、4、5、7
- 明确不做 bot REST 桥 → Out of Scope + Protocol
- 建房权限 → Task 6
- 内置插件可用 → Task 7
- 文档与验收 → Task 8

### Placeholder scan
- 无 TBD；测试代码与实现代码均给出可粘贴片段
- `newTestHub` 允许复用现有 test helper，实现时按仓库实际符号微调，但不改变行为规格

### Type consistency
- `EventType.OnMemberJoined` / `OnMemberKicked` / `OnUserMuted` 在 Task 1 定义，Task 5/7 使用同名
- `createCapabilityRouter` 在 Task 4 定义，Task 7 经 `ctx.chat/rooms/voice/users` 使用
- `bot:command` / `bot:message` 仅作为 Socket 事件名前后端一致
- 无 `/api/v1/bot/command|message|kick` 路由

---

## Execution notes

- 工作树：执行前如需隔离，用 `superpowers:using-git-worktrees`
- 提交粒度：每 Task 一次 commit（计划内已写）
- 不要顺手做插件市场 / Admin Runtime API / 完整 IM
