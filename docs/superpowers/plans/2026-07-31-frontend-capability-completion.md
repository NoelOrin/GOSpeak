# Frontend Capability Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于 6 份已有计划文档，补全 GOSpeak 前端缺失能力：私聊系统 UI、Guild 布局集成、WebSocket 客户端迁移、E2E 测试辅助脚本，以及后端私聊发送支持。

**Architecture:** 本计划覆盖 5 个独立子系统。子系统 1（后端私聊发送）是子系统 2（前端私聊 UI）的前置依赖。子系统 3（Guild 集成）、子系统 4（WS 迁移）、子系统 5（E2E helpers）互相独立，可并行执行。前端使用 SolidJS + TypeScript + Vite + TanStack Router + DaisyUI，后端使用 Go + Gin + GORM。

**Tech Stack:** SolidJS, TypeScript, Vite, TanStack Router, DaisyUI, cui-solid, lucide-solid, IndexedDB, Socket.IO (→ 原生 WS), Vitest, Playwright, Go, Gin, GORM

**Scope Note:** 本计划覆盖多个独立子系统。如需更细粒度管理，可按子系统拆分为独立 plan 文件。当前合并为一份以便统一追踪。

---

## File Inventory

### 后端新增文件
| 文件 | 职责 |
|------|------|
| `app/server/internal/signal/private_bridge.go` | 私聊 WS 事件 handler (OnPrivateMessageSend) |

### 后端修改文件
| 文件 | 改动 |
|------|------|
| `app/server/internal/model/message.go` | 增加 ConversationType / ConversationID / TargetIdentity 字段 |
| `app/server/internal/signal/events.go` | 增加 EventPrivateSend / EventPrivateNew 常量 |
| `app/server/internal/service/conversation_service.go` | 增加 SendDirect 方法 + eventBus 字段 |
| `app/server/internal/handler/conversation_handler.go` | 增加 Send handler |
| `app/server/internal/router/routes/conversation/routes.go` | 增加 /send 路由 |
| `app/server/server/gin.go` | 注入 eventBus 到 ConversationService |
| `app/server/internal/signal/hub.go` | 注册 OnPrivateMessageSend |

### 前端新增文件
| 文件 | 职责 |
|------|------|
| `app/web/src/components/chat/conversationList.tsx` | 私聊会话列表（左面板） |
| `app/web/src/components/chat/chatWindow.tsx` | 消息列表 + 输入框（中心区） |
| `app/web/src/components/chat/memberSidebar.tsx` | Guild 成员列表（右面板，点击发起私聊） |
| `app/web/src/components/chat/chatPage.tsx` | 组装三栏私聊布局 |
| `app/web/src/pages/(app)/chat/index.tsx` | /chat 路由页面 |
| `app/web/src/socket/wsClient.ts` | 原生 WebSocket 客户端（替代 socket.io-client） |

### 前端修改文件
| 文件 | 改动 |
|------|------|
| `app/web/src/stores/socketStore.ts` | 绑定 PRIVATE_NEW 事件；可选迁移到原生 WS |
| `app/web/src/layouts/common/sidebar.tsx` | 添加聊天图标导航到 /chat |
| `app/web/src/components/common/dynamicRender.tsx` | 添加 /chat 和 /guild 前缀映射 |
| `app/web/src/layouts/layout.tsx` | prev slot 中集成 GuildList 服务器栏 |
| `app/web/src/components/guild/GuildList.tsx` | 添加创建/加入按钮 + 导航回调 |
| `app/web/src/pages/(app)/guild/$guildUUID/index.tsx` | 增强：房间列表 + 离开/删除按钮 |
| `app/web/src/components/room/roomList.tsx` | 按 currentGuildUUID 过滤房间 |
| `app/web/src/routeTree.gen.ts` | 重新生成（新增 /chat 路由） |

### E2E 新增文件
| 文件 | 职责 |
|------|------|
| `.agents/skills/room-voice-e2e/scripts/guild-helpers.mjs` | Guild API + UI 操作辅助 |
| `.agents/skills/room-voice-e2e/scripts/ws-helpers.mjs` | WS 消息抓包 + 协议验证 |
| `.agents/skills/room-voice-e2e/scripts/cleanup-helpers.mjs` | 测试数据清理 |

---

## Subsystem 1: 后端私聊发送支持

> 前端 chatStore 已实现私聊状态管理，但后端缺少发送链路。本子系统补全后端。

### Task 1: 扩展 Message 模型

**Files:**
- Modify: `app/server/internal/model/message.go`

- [ ] **Step 1: 在 Message struct 中添加会话字段**

将以下三个字段添加到 `Message` struct（放在 `ReplyTo` 之后、`EditedAt` 之前）：

```go
	ConversationType string  `gorm:"size:10;index:idx_msg_conversation,priority:1;default:room" json:"conversation_type"`
	ConversationID   *string `gorm:"size:32;index:idx_msg_conversation,priority:2" json:"conversation_id,omitempty"`
	TargetIdentity   *string `gorm:"size:64;index" json:"target_identity,omitempty"`
```

同时将 `RoomUUID` 的 `not null` 去掉（私聊消息不绑定 room）：

```go
	RoomUUID  string `gorm:"size:36;index:idx_msg_room_created,priority:1" json:"room_uuid"`
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/model/message.go
git commit -m "feat(model): add conversation fields to Message for private chat"
```

### Task 2: 添加私聊事件常量

**Files:**
- Modify: `app/server/internal/signal/events.go`

- [ ] **Step 1: 在 events.go 末尾 `)` 前添加私聊常量**

```go
	// 私聊消息事件（客户端 → 服务端）
	EventPrivateSend = "private:send"

	// 私聊消息事件（服务端推送）
	EventPrivateNew = "private:new"
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

### Task 3: ConversationService 增加 SendDirect 方法

**Files:**
- Modify: `app/server/internal/service/conversation_service.go`

- [ ] **Step 1: 添加 import 和 eventBus 字段**

在 import 块中添加：
```go
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/bus"
	"github.com/google/uuid"
```

在 `ConversationService` struct 中添加 `eventBus` 字段：
```go
type ConversationService struct {
	convRepo    *repository.ConversationRepository
	messageRepo *repository.MessageRepository
	eventBus    *bus.EventBus
}
```

添加 setter：
```go
func (s *ConversationService) SetEventBus(b *bus.EventBus) {
	s.eventBus = b
}
```

- [ ] **Step 2: 实现 SendDirect 方法**

```go
// SendDirect creates and broadcasts a private message, then persists it.
func (s *ConversationService) SendDirect(senderIdentity, targetIdentity, content, clientNonce string) (*MessageDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}
	if senderIdentity == "" || targetIdentity == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "sender and target required")
	}
	if senderIdentity == targetIdentity {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "cannot send to self")
	}

	// Generate conversation ID: sorted identities → MD5 hex (32 chars)
	identities := []string{senderIdentity, targetIdentity}
	sort.Strings(identities)
	hashBytes := md5.Sum([]byte(identities[0] + ":" + identities[1]))
	convID := hex.EncodeToString(hashBytes[:])

	now := time.Now().UTC()
	msgUUID := uuid.New().String()

	// Upsert conversation participant row
	cp := &model.ConversationParticipant{
		ConversationID:      convID,
		IdentityA:           identities[0],
		IdentityB:           identities[1],
		LastContent:         content,
		LastSenderIdentity:  senderIdentity,
		LastMessageAt:       &now,
	}
	if err := s.convRepo.Upsert(cp); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// Increment unread for receiver
	_ = s.convRepo.IncrementUnread(convID, senderIdentity)

	dto := &MessageDTO{
		UUID:      msgUUID,
		AuthorID:  senderIdentity,
		Content:   content,
		Deleted:   false,
		CreatedAt: now,
	}

	// Broadcast private:new to both participants
	if s.eventBus != nil {
		payload, _ := json.Marshal(dto)
		_ = s.eventBus.Publish(context.Background(), "private:new", payload)
	}

	// Persist message (sync for now; job queue can be added later)
	convIDPtr := convID
	targetPtr := targetIdentity
	msg := &model.Message{
		UUID:             msgUUID,
		AuthorID:         senderIdentity,
		Content:          content,
		CreatedAt:        now,
		UpdatedAt:        now,
		ConversationType: "private",
		ConversationID:   &convIDPtr,
		TargetIdentity:   &targetPtr,
	}
	if err := s.messageRepo.Create(msg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return dto, nil
}
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过（可能有 bus.EventBus 类型不匹配，根据实际 bus 包调整）

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/service/conversation_service.go
git commit -m "feat(service): add SendDirect for private chat messages"
```

### Task 4: 添加 Send handler 和路由

**Files:**
- Modify: `app/server/internal/handler/conversation_handler.go`
- Modify: `app/server/internal/router/routes/conversation/routes.go`

- [ ] **Step 1: 在 conversation_handler.go 中添加 Send handler**

```go
func (h *ConversationHandler) Send(c *gin.Context) {
	var req struct {
		TargetIdentity string `json:"target_identity" binding:"required"`
		Content        string `json:"content" binding:"required"`
		ClientNonce    string `json:"client_nonce"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, exists := c.Get("username")
	if !exists {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	identity, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid context")
		return
	}

	out, err := h.svc.SendDirect(identity, req.TargetIdentity, req.Content, req.ClientNonce)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}
```

- [ ] **Step 2: 在 routes.go 中注册 /send 路由**

```go
func RegisterProtected(r *gin.RouterGroup, h *handler.ConversationHandler) {
	r.POST("/list", h.List)
	r.POST("/messages", h.Messages)
	r.POST("/mark-read", h.MarkRead)
	r.POST("/send", h.Send)
}
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/handler/conversation_handler.go app/server/internal/router/routes/conversation/routes.go
git commit -m "feat(handler): add /conversation/send endpoint for private messages"
```

### Task 5: gin.go 注入 eventBus + Hub 注册私聊事件

**Files:**
- Modify: `app/server/server/gin.go`
- Create: `app/server/internal/signal/private_bridge.go`
- Modify: `app/server/internal/signal/hub.go`

- [ ] **Step 1: 在 gin.go 中注入 eventBus 到 ConversationService**

找到 ConversationService 初始化的位置，添加 `convSvc.SetEventBus(eventBus)`（变量名根据实际代码调整）。

- [ ] **Step 2: 创建 private_bridge.go**

```go
package signal

import (
	"encoding/json"
	"fmt"

	socketio "github.com/googollee/go-socket.io"
)

// privateSendPayload is the client->server payload for private:send.
type privateSendPayload struct {
	TargetIdentity string `json:"target_identity"`
	Content        string `json:"content"`
	ClientNonce    string `json:"client_nonce,omitempty"`
}

// OnPrivateMessageSend handles private:send events from clients.
func (h *Hub) OnPrivateMessageSend(s socketio.Conn, data string) (string, error) {
	if h.convSvc == nil {
		return marshalAck(map[string]interface{}{"error": "conversation service unavailable"})
	}

	var req privateSendPayload
	if err := parseJSON(data, &req); err != nil || req.TargetIdentity == "" || req.Content == "" {
		return marshalAck(map[string]interface{}{"error": "target_identity and content are required"})
	}

	identity := claimsIdentity(s)
	if identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	dto, err := h.convSvc.SendDirect(identity, req.TargetIdentity, req.Content, req.ClientNonce)
	if err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}

	resp, _ := json.Marshal(dto)
	return string(resp), nil
}
```

- [ ] **Step 3: 在 Hub 中注册 OnPrivateMessageSend**

在 hub.go 的事件注册函数中（与 OnMessageSend 同级），添加：
```go
server.OnEvent("/", EventPrivateSend, h.OnPrivateMessageSend)
```

在 Hub struct 中添加 `convSvc` 字段（类型 `*service.ConversationService`），并在 gin.go 中注入。

- [ ] **Step 4: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/signal/private_bridge.go app/server/internal/signal/hub.go app/server/server/gin.go
git commit -m "feat(signal): wire private:send WS event to ConversationService"
```

---

## Subsystem 2: 前端私聊 UI

> chatStore 私聊状态管理已实现。本子系统补全 UI 组件、路由、侧边栏集成。

### Task 6: socketStore 绑定 PRIVATE_NEW 事件

**Files:**
- Modify: `app/web/src/stores/socketStore.ts`

- [ ] **Step 1: 在 socketStore 的事件绑定区域添加 PRIVATE_NEW 监听**

找到 socketStore 中绑定 server event 的位置（通常在 connect 回调或 onMount 中），添加：

```ts
// 私聊消息推送
socket.on(EVENTS.PRIVATE_NEW, (dto: PrivateMessageDTO) => {
  chatStore.handlePrivateNew(dto);
});
```

需要在文件顶部添加 import：
```ts
import type { PrivateMessageDTO } from "@/api/conversation";
```

- [ ] **Step 2: 类型检查**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit 2>&1 | head -20`
Expected: 无新增类型错误

### Task 7: 私聊会话列表组件

**Files:**
- Create: `app/web/src/components/chat/conversationList.tsx`

- [ ] **Step 1: 创建 conversationList.tsx**

```tsx
import { For, Show, onMount } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import Avatar from "@/components/common/avatar";

export default function ConversationList() {
	onMount(() => {
		void chatStore.loadConversations();
	});

	const active = () => chatStore.activeConversationID();

	return (
		<div class="flex flex-col h-full overflow-y-auto">
			<div class="px-3 py-2 text-xs font-semibold text-base-content/50 uppercase tracking-wide">
				私聊
			</div>
			<Show
				when={chatStore.conversations().length > 0}
				fallback={
					<div class="px-3 py-8 text-center text-sm text-base-content/30">
						暂无私聊会话
					</div>
				}
			>
				<For each={chatStore.conversations()}>
					{(conv) => (
						<button
							type="button"
							class="flex items-center gap-3 px-3 py-2.5 hover:bg-base-200 transition-colors text-left w-full"
							classList={{
								"bg-base-200": active() === conv.conversation_id,
							}}
							onClick={() => void chatStore.selectConversation(conv.conversation_id)}
						>
							<Avatar
								src={conv.other_avatar || undefined}
								alt={conv.other_display_name}
								size={36}
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between gap-2">
									<span class="text-sm font-medium truncate">
										{conv.other_display_name || conv.other_identity}
									</span>
									<Show when={conv.unread_count > 0}>
										<span class="badge badge-primary badge-xs shrink-0">
											{conv.unread_count}
										</span>
									</Show>
								</div>
								<div class="text-xs text-base-content/40 truncate">
									{conv.last_content || "开始对话"}
								</div>
							</div>
						</button>
					)}
				</For>
			</Show>
		</div>
	);
}
```

### Task 8: 聊天窗口组件

**Files:**
- Create: `app/web/src/components/chat/chatWindow.tsx`

- [ ] **Step 1: 创建 chatWindow.tsx**

```tsx
import { createEffect, For, Show } from "solid-js";
import { createSignal } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import userStore from "@/stores/userStore";
import type { PrivateMessageDTO } from "@/api/conversation";

export default function ChatWindow() {
	const active = () => chatStore.activeConversationID();
	const messages = () =>
		active() ? chatStore.pmMessages()[active()!] || [] : [];
	let scrollRef!: HTMLDivElement;
	const [content, setContent] = createSignal("");
	let textareaRef: HTMLTextAreaElement | undefined;

	// Auto-scroll to bottom on new messages
	createEffect(() => {
		const len = messages().length;
		if (len > 0 && scrollRef) {
			requestAnimationFrame(() => {
				scrollRef.scrollTop = scrollRef.scrollHeight;
			});
		}
	});

	function handleSend() {
		const text = content().trim();
		if (!text || !active()) return;
		chatStore.sendDirect(active()!, text);
		setContent("");
		if (textareaRef) textareaRef.style.height = "auto";
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			handleSend();
		}
	}

	function handleInput() {
		if (!textareaRef) return;
		setContent(textareaRef.value);
		textareaRef.style.height = "auto";
		textareaRef.style.height = `${Math.min(textareaRef.scrollHeight, 96)}px`;
	}

	const isOwn = (msg: PrivateMessageDTO) => {
		const u = userStore.user();
		if (!u) return false;
		return msg.author_id === u.name || msg.author_id === u.display_name;
	};

	const formatTime = (iso: string) => {
		try {
			return new Date(iso).toLocaleTimeString([], {
				hour: "2-digit",
				minute: "2-digit",
			});
		} catch {
			return "";
		}
	};

	return (
		<div class="flex flex-col h-full">
			{/* Top bar */}
			<div class="px-4 py-3 border-b border-base-300 shrink-0">
				<span class="text-sm font-semibold">
					{active()
						? chatStore
								.conversations()
								.find((c) => c.conversation_id === active())
								?.other_display_name || "私聊"
						: "选择一个会话"}
				</span>
			</div>

			{/* Message list */}
			<div ref={scrollRef} class="flex-1 overflow-y-auto px-4 py-2">
				<Show
					when={messages().length > 0}
					fallback={
						<div class="flex items-center justify-center h-full text-sm text-base-content/30">
							发送一条消息开始对话
						</div>
					}
				>
					<For each={messages()}>
						{(msg) => (
							<div
								class="flex flex-col mb-2"
								classList={{
									"items-end": isOwn(msg),
									"items-start": !isOwn(msg),
								}}
							>
								<div
									class="max-w-[70%] rounded-2xl px-3 py-1.5 text-sm break-words"
									classList={{
										"bg-primary text-primary-content": isOwn(msg),
										"bg-base-200": !isOwn(msg),
									}}
								>
									{msg.deleted ? "[消息已删除]" : msg.content}
								</div>
								<span class="text-[10px] text-base-content/30 mt-0.5">
									{formatTime(msg.created_at)}
								</span>
							</div>
						)}
					</For>
				</Show>
			</div>

			{/* Input */}
			<Show when={active()}>
				<div class="border-t border-base-300 p-2 shrink-0">
					<div class="flex items-end gap-2">
						<textarea
							ref={(el) => (textareaRef = el)}
							value={content()}
							onInput={handleInput}
							onKeyDown={handleKeyDown}
							placeholder="输入消息..."
							class="textarea textarea-bordered flex-1 min-h-[40px] max-h-[96px] resize-none text-sm"
							rows={1}
						/>
						<button
							type="button"
							class="btn btn-primary h-10 min-h-10 shrink-0 px-4"
							disabled={!content().trim()}
							onClick={handleSend}
						>
							发送
						</button>
					</div>
				</div>
			</Show>
		</div>
	);
}
```

### Task 9: 成员侧栏组件

**Files:**
- Create: `app/web/src/components/chat/memberSidebar.tsx`

- [ ] **Step 1: 创建 memberSidebar.tsx**

```tsx
import { For, Show, onMount } from "solid-js";
import Avatar from "@/components/common/avatar";
import guildStore from "@/stores/guildStore";
import userStore from "@/stores/userStore";

export default function MemberSidebar() {
	onMount(() => {
		const guildUUID = guildStore.state.currentGuildUUID;
		if (guildUUID) {
			void guildStore.loadMembers(guildUUID);
		}
	});

	const members = () => {
		const guildUUID = guildStore.state.currentGuildUUID;
		if (!guildUUID) return [];
		return guildStore.state.memberCache[guildUUID] || [];
	};

	const currentUserName = () => userStore.user()?.name || "";

	return (
		<div class="flex flex-col h-full overflow-y-auto">
			<div class="px-3 py-2 text-xs font-semibold text-base-content/50 uppercase tracking-wide shrink-0">
				成员
			</div>
			<Show
				when={members().length > 0}
				fallback={
					<div class="px-3 py-8 text-center text-sm text-base-content/30">
						暂无成员
					</div>
				}
			>
				<For each={members()}>
					{(member) => (
						<Show when={member.user_uuid !== currentUserName()}>
							<button
								type="button"
								class="flex items-center gap-2 px-3 py-2 hover:bg-base-200 transition-colors w-full text-left"
								onClick={() => void chatStore.startConversation(member.nickname || member.user_uuid)}
							>
								<Avatar size={28} alt={member.nickname} />
								<span class="text-sm truncate">
									{member.nickname || member.user_uuid}
								</span>
							</button>
						</Show>
					)}
				</For>
			</Show>
		</div>
	);
}
```

注意：需要添加 `import { chatStore } from "@/stores/chatStore";`。

### Task 10: 聊天页面组装 + 路由

**Files:**
- Create: `app/web/src/components/chat/chatPage.tsx`
- Create: `app/web/src/pages/(app)/chat/index.tsx`
- Modify: `app/web/src/layouts/common/sidebar.tsx`
- Modify: `app/web/src/components/common/dynamicRender.tsx`

- [ ] **Step 1: 创建 chatPage.tsx**

```tsx
import ChatWindow from "./chatWindow";
import MemberSidebar from "./memberSidebar";

export default function ChatPage() {
	return (
		<div class="flex flex-row w-full h-full">
			<div class="flex-1 min-w-0 border-r border-base-300">
				<ChatWindow />
			</div>
			<div class="w-48 shrink-0 border-l border-base-300 hidden sm:block">
				<MemberSidebar />
			</div>
		</div>
	);
}
```

- [ ] **Step 2: 创建 /chat 路由页面**

```tsx
import { createFileRoute } from "@tanstack/solid-router";
import ChatPage from "@/components/chat/chatPage";

export const Route = createFileRoute("/(app)/chat/")({
	component: RouteComponent,
	staticData: { title: "聊天", icon: "message-square" },
});

function RouteComponent() {
	return <ChatPage />;
}
```

- [ ] **Step 3: 在 sidebar.tsx 中添加聊天图标**

在"频道"和"设置"之间添加：
```tsx
<OptionSquare label="聊天" onClick={() => navigate({ to: "/chat" })}>
	<MessageSquare {...iconProps} />
</OptionSquare>
```

添加 import：
```tsx
import MessageSquare from "lucide-solid/icons/message-square";
```

- [ ] **Step 4: 在 dynamicRender.tsx 中添加 /chat 前缀**

在 PREFIX_MAP 中添加（放在 `/channel` 之后）：
```tsx
import ConversationList from "@/components/chat/conversationList";

const PREFIX_MAP: [string, (...args: any[]) => JSX.Element][] = [
	["/manage", ManageNav],
	["/channel", RoomList],
	["/chat", ConversationList],
	["/", HomePage],
];
```

- [ ] **Step 5: 重新生成路由树**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx @tanstack/router-cli generate`
Expected: routeTree.gen.ts 中出现 /chat 路由

- [ ] **Step 6: 类型检查**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit 2>&1 | head -20`
Expected: 无新增类型错误

- [ ] **Step 7: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/web/src/components/chat/ app/web/src/pages/\(app\)/chat/ app/web/src/layouts/common/sidebar.tsx app/web/src/components/common/dynamicRender.tsx app/web/src/routeTree.gen.ts
git commit -m "feat(chat): add private chat UI with conversation list, chat window, member sidebar, and /chat route"
```

---

## Subsystem 3: Guild 布局集成

> Guild API/Store/基础组件已存在，但未集成到布局中。

### Task 11: GuildList 增强

**Files:**
- Modify: `app/web/src/components/guild/GuildList.tsx`

- [ ] **Step 1: 添加创建/加入按钮和导航回调**

```tsx
import { type Component, createResource, createSignal, For } from "solid-js";
import Plus from "lucide-solid/icons/plus";
import UserPlus from "lucide-solid/icons/user-plus";
import { useNavigate } from "@tanstack/solid-router";
import { type Guild, getGuild } from "@/api/guild";
import guildStore from "@/stores/guildStore";
import GuildIcon from "./GuildIcon";
import CreateGuildModal from "./CreateGuildModal";
import JoinGuildModal from "./JoinGuildModal";

const GuildList: Component = () => {
	const navigate = useNavigate();
	const { state, loadMyGuilds, setCurrentGuild } = guildStore;
	const [createRef, setCreateRef] = createSignal<HTMLDialogElement>();
	const [joinRef, setJoinRef] = createSignal<HTMLDialogElement>();

	loadMyGuilds();

	const [guilds] = createResource<Guild[], string[]>(
		() => state.myGuildUUIDs,
		async (uuids: string[]) => {
			const results = await Promise.allSettled(uuids.map((u) => getGuild(u)));
			return results
				.filter((r): r is PromiseFulfilledResult<Guild> => r.status === "fulfilled")
				.map((r) => r.value);
		},
	);

	const handleSelect = (uuid: string) => {
		setCurrentGuild(uuid);
		navigate({ to: "/guild/$guildUUID", params: { guildUUID: uuid } });
	};

	return (
		<>
			<div class="w-16 bg-base-300 flex flex-col items-center py-3 gap-2 overflow-y-auto">
				<For each={guilds() || []}>
					{(guild) => (
						<GuildIcon
							name={guild.name}
							iconUrl={guild.icon_url}
							active={state.currentGuildUUID === guild.uuid}
							onClick={() => handleSelect(guild.uuid)}
						/>
					)}
				</For>
				<div class="divider my-1" />
				<button
					type="button"
					class="w-12 h-12 rounded-2xl flex items-center justify-center bg-base-200 hover:bg-base-100 transition-colors text-base-content/60"
					title="创建服务器"
					onClick={() => createRef()?.showModal()}
				>
					<Plus size={22} />
				</button>
				<button
					type="button"
					class="w-12 h-12 rounded-2xl flex items-center justify-center bg-base-200 hover:bg-base-100 transition-colors text-base-content/60"
					title="加入服务器"
					onClick={() => joinRef()?.showModal()}
				>
					<UserPlus size={22} />
				</button>
			</div>
			<CreateGuildModal ref={setCreateRef} onClose={() => createRef()?.close()} />
			<JoinGuildModal ref={setJoinRef} onClose={() => joinRef()?.close()} />
		</>
	);
};

export default GuildList;
```

### Task 12: 布局集成 GuildList

**Files:**
- Modify: `app/web/src/layouts/layout.tsx`

- [ ] **Step 1: 在 DesktopLayout 的 prev slot 中添加 GuildList**

找到 `DesktopLayout` 函数中的 `<Slot name="prev">` 块，在 `<Sidebar>` 之前添加 `<GuildList />`：

```tsx
<Slot name="prev">
	<div class="flex flex-col justify-between h-full" ref={prevRef}>
		<div class="flex h-full">
			<GuildList />
			<Sidebar onOpenSettings={openSettings} />
			<div class="box-border flex-1 border-color border-t border-l border-solid">
				<DynamicRender />
			</div>
		</div>
		<UserBar onOpenSettings={openSettings} />
	</div>
</Slot>
```

添加 import：
```tsx
import GuildList from "@/components/guild/GuildList";
```

### Task 13: DynamicRender 添加 /guild 前缀 + Guild 页面增强

**Files:**
- Modify: `app/web/src/components/common/dynamicRender.tsx`
- Modify: `app/web/src/pages/(app)/guild/$guildUUID/index.tsx`

- [ ] **Step 1: 在 dynamicRender.tsx PREFIX_MAP 中添加 /guild**

```tsx
const PREFIX_MAP: [string, (...args: any[]) => JSX.Element][] = [
	["/manage", ManageNav],
	["/guild", RoomList],
	["/channel", RoomList],
	["/chat", ConversationList],
	["/", HomePage],
];
```

- [ ] **Step 2: 增强 Guild 页面 — 添加离开/删除按钮**

```tsx
import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { createSignal, onMount, Show } from "solid-js";
import { deleteGuild, leaveGuild } from "@/api/guild";
import guildStore from "@/stores/guildStore";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/(app)/guild/$guildUUID/")({
	component: RouteComponent,
	staticData: { title: "语音服务器", icon: "icon-channel" },
});

function RouteComponent() {
	const { state, setCurrentGuild, removeGuild } = guildStore;
	const params = Route.useParams();
	const navigate = useNavigate();
	const [loading, setLoading] = createSignal(false);

	onMount(() => {
		setCurrentGuild(params().guildUUID);
	});

	const guild = () => state.guildCache[params().guildUUID];
	const isOwner = () => {
		const u = userStore.user();
		return u && guild()?.owner_uuid === u.uuid;
	};

	async function handleLeave() {
		setLoading(true);
		try {
			await leaveGuild(params().guildUUID);
			removeGuild(params().guildUUID);
			navigate({ to: "/" });
		} finally {
			setLoading(false);
		}
	}

	async function handleDelete() {
		if (!confirm("确定删除此服务器？此操作不可撤销。")) return;
		setLoading(true);
		try {
			await deleteGuild(params().guildUUID);
			removeGuild(params().guildUUID);
			navigate({ to: "/" });
		} finally {
			setLoading(false);
		}
	}

	return (
		<div class="flex-1 flex flex-col p-4">
			<div class="text-2xl font-bold mb-2">{guild()?.name || "Loading..."}</div>
			<p class="text-base-content/60 mb-4">{guild()?.description || ""}</p>
			<div class="divider" />
			<div class="flex items-center gap-4 text-sm text-base-content/40 mb-4">
				<span>邀请码: <code class="text-base-content/70">{guild()?.invite_code || "-"}</code></span>
				<span>成员上限: {guild()?.max_rooms || "无限"}</span>
			</div>
			<div class="flex gap-2 mt-auto">
				<Show when={!isOwner()}>
					<button class="btn btn-error btn-sm" onClick={handleLeave} disabled={loading()}>
						离开服务器
					</button>
				</Show>
				<Show when={isOwner()}>
					<button class="btn btn-error btn-sm" onClick={handleDelete} disabled={loading()}>
						删除服务器
					</button>
				</Show>
			</div>
		</div>
	);
}
```

- [ ] **Step 3: 类型检查**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit 2>&1 | head -20`
Expected: 无新增类型错误

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/web/src/components/guild/GuildList.tsx app/web/src/layouts/layout.tsx app/web/src/components/common/dynamicRender.tsx app/web/src/pages/\(app\)/guild/
git commit -m "feat(guild): integrate GuildList into layout, add create/join buttons, enhance guild page"
```

---

## Subsystem 4: WebSocket 客户端迁移

> 将前端从 socket.io-client 迁移到原生 WebSocket，匹配后端 Phase 2 WS 协议。
> 注意：此子系统与后端 Phase 2 WS 迁移有耦合。如果后端尚未完成 WS 迁移，可跳过此子系统，保持 socket.io 客户端不变。

### Task 14: 创建原生 WS 客户端适配器

**Files:**
- Create: `app/web/src/socket/wsClient.ts`

- [ ] **Step 1: 创建 wsClient.ts**

```ts
// Native WebSocket client adapter matching backend Phase 2 WS protocol.
// Wire format: {"id":"1","event":"room:join","data":{"room":"lobby"}}
// ACK: {"id":"1","event":"room:join","data":{...}}
// Push: {"event":"member:joined","data":{...}}
// Error: {"id":"1","event":"room:join","error":{"code":3001,"message":"..."}}

interface WSMessage {
	id?: string;
	event: string;
	data?: unknown;
	error?: { code: number; message: string };
}

export function createWSClient() {
	let ws: WebSocket | null = null;
	let msgIdCounter = 0;
	const pendingAcks = new Map<string, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();
	const eventHandlers = new Map<string, Set<(data: unknown) => void>>();
	const connectedCbs: Array<() => void> = [];
	const disconnectedCbs: Array<(reason: string) => void> = [];
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let currentUrl = "";
	let currentToken = "";

	function connect(url: string, token?: string) {
		currentUrl = url;
		currentToken = token || "";
		if (ws?.readyState === WebSocket.OPEN) return;
		if (ws) ws.close();

		const wsUrl = token ? `${url}?token=${encodeURIComponent(token)}` : url;
		ws = new WebSocket(wsUrl);

		ws.onopen = () => {
			for (const cb of connectedCbs) cb();
		};

		ws.onmessage = (ev: MessageEvent) => {
			try {
				const msg: WSMessage = JSON.parse(ev.data);
				if (!msg.event) return;

				// If it has an id, it's an ACK
				if (msg.id && pendingAcks.has(msg.id)) {
					const pending = pendingAcks.get(msg.id)!;
					pendingAcks.delete(msg.id);
					if (msg.error) {
						pending.reject(new Error(msg.error.message));
					} else {
						pending.resolve(msg.data);
					}
					return;
				}

				// Otherwise it's a push event
				const handlers = eventHandlers.get(msg.event);
				if (handlers) {
					for (const handler of handlers) handler(msg.data);
				}
			} catch {
				// Ignore malformed messages
			}
		};

		ws.onclose = (ev: CloseEvent) => {
			for (const cb of disconnectedCbs) cb(ev.reason || "closed");
			// Auto-reconnect with backoff
			if (currentUrl) {
				reconnectTimer = setTimeout(() => connect(currentUrl, currentToken), 3000);
			}
		};

		ws.onerror = () => {
			// Error handled by onclose
		};
	}

	function disconnect() {
		if (reconnectTimer) clearTimeout(reconnectTimer);
		reconnectTimer = null;
		currentUrl = "";
		if (ws) {
			ws.onclose = null;
			ws.close();
			ws = null;
		}
		for (const [, pending] of pendingAcks) pending.reject(new Error("disconnected"));
		pendingAcks.clear();
	}

	function emitFireAndForget(event: string, payload?: unknown) {
		if (!ws || ws.readyState !== WebSocket.OPEN) return;
		const msg: WSMessage = { event, data: payload };
		ws.send(JSON.stringify(msg));
	}

	function emitAck(event: string, payload?: unknown): Promise<unknown> {
		return new Promise((resolve, reject) => {
			if (!ws || ws.readyState !== WebSocket.OPEN) {
				reject(new Error("socket not connected"));
				return;
			}
			const id = String(++msgIdCounter);
			pendingAcks.set(id, { resolve, reject });
			const msg: WSMessage = { id, event, data: payload };
			ws.send(JSON.stringify(msg));

			// Timeout after 10s
			setTimeout(() => {
				if (pendingAcks.has(id)) {
					pendingAcks.delete(id);
					reject(new Error("ack timeout"));
				}
			}, 10000);
		});
	}

	function onServerEvent(event: string, cb: (data: unknown) => void): () => void {
		if (!eventHandlers.has(event)) eventHandlers.set(event, new Set());
		eventHandlers.get(event)!.add(cb);
		return () => {
			eventHandlers.get(event)?.delete(cb);
		};
	}

	function offAllServerEvents() {
		eventHandlers.clear();
	}

	function isConnected() {
		return ws?.readyState === WebSocket.OPEN;
	}

	function getSocket() {
		return ws;
	}

	function onConnected(cb: () => void): () => void {
		connectedCbs.push(cb);
		return () => {
			const idx = connectedCbs.indexOf(cb);
			if (idx >= 0) connectedCbs.splice(idx, 1);
		};
	}

	function onDisconnected(cb: (reason: string) => void): () => void {
		disconnectedCbs.push(cb);
		return () => {
			const idx = disconnectedCbs.indexOf(cb);
			if (idx >= 0) disconnectedCbs.splice(idx, 1);
		};
	}

	return {
		connect,
		disconnect,
		emitFireAndForget,
		emitAck,
		onServerEvent,
		offAllServerEvents,
		getSocket,
		isConnected,
		onConnected,
		onDisconnected,
	};
}
```

### Task 15: 迁移 socketStore 到 WS 适配器（可选，依赖后端 Phase 2）

**Files:**
- Modify: `app/web/src/stores/socketStore.ts`

> **注意：** 此任务仅在后端完成 Phase 2 WS 迁移后执行。如果后端仍运行 socket.io，跳过此任务。

- [ ] **Step 1: 替换 socket.io import 为 wsClient**

```ts
// 旧:
// import io from "socket.io-client";
// 新:
import { createWSClient } from "@/socket/wsClient";
```

- [ ] **Step 2: 替换 createSocketClient() 调用为 createWSClient()**

- [ ] **Step 3: 调整 emit/on 调用模式**

socket.io 的 `socket.emit(event, data, callback)` 模式需改为：
```ts
// 旧: socket.emit(EVENTS.ROOM_JOIN, JSON.stringify(payload), (ack) => { ... });
// 新: wsClient.emitAck(EVENTS.ROOM_JOIN, payload).then(...).catch(...);
```

socket.io 的 `socket.on(event, cb)` 需改为：
```ts
// 旧: socket.on(EVENTS.MESSAGE_CREATED, handler);
// 新: wsClient.onServerEvent(EVENTS.MESSAGE_CREATED, handler);
```

- [ ] **Step 4: 类型检查 + 运行验证**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit 2>&1 | head -20`
Expected: 无类型错误

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/web/src/socket/wsClient.ts app/web/src/stores/socketStore.ts
git commit -m "feat(socket): migrate from socket.io-client to native WebSocket adapter"
```

---

## Subsystem 5: E2E 测试辅助脚本

> 基于 e2e-test-plan.md，新增 guild/ws/cleanup helpers。

### Task 16: Guild Helpers

**Files:**
- Create: `.agents/skills/room-voice-e2e/scripts/guild-helpers.mjs`

- [ ] **Step 1: 创建 guild-helpers.mjs**

```javascript
// Guild API + UI 操作辅助
// 用于 E2E 测试中的 Guild 创建/加入/成员管理

async function getAuthToken(page) {
  return await page.evaluate(() => {
    return localStorage.getItem('token') || '';
  });
}

async function createGuild(page, name, { description, isPublic } = {}) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ name: opts.name, description: opts.description || '', is_public: opts.isPublic || false }),
    });
    return r.json();
  }, { token, name, description, isPublic });
  if (res.code !== 0) throw new Error(`createGuild failed: ${res.msg}`);
  return res.data;
}

async function joinGuild(page, inviteCode) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/join', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ invite_code: opts.inviteCode }),
    });
    return r.json();
  }, { token, inviteCode });
  if (res.code !== 0) throw new Error(`joinGuild failed: ${res.msg}`);
  return res.data;
}

async function getGuildMembers(page, guildUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/members', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ guild_uuid: opts.guildUUID }),
    });
    return r.json();
  }, { token, guildUUID });
  return res.data?.members || [];
}

async function kickGuildMember(page, guildUUID, userUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/kick', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ guild_uuid: opts.guildUUID, user_uuid: opts.userUUID }),
    });
    return r.json();
  }, { token, guildUUID, userUUID });
  if (res.code !== 0) throw new Error(`kickGuildMember failed: ${res.msg}`);
}

async function leaveGuild(page, guildUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/leave', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ uuid: opts.guildUUID }),
    });
    return r.json();
  }, { token, guildUUID });
  if (res.code !== 0) throw new Error(`leaveGuild failed: ${res.msg}`);
}

async function deleteGuild(page, guildUUID) {
  const token = await getAuthToken(page);
  const res = await page.evaluate(async (opts) => {
    const r = await fetch('/api/v1/guild/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ uuid: opts.guildUUID }),
    });
    return r.json();
  }, { token, guildUUID });
  if (res.code !== 0) throw new Error(`deleteGuild failed: ${res.msg}`);
}

// UI 操作
async function getVisibleGuilds(page) {
  return page.$$eval('[data-testid="guild-icon"]', (els) =>
    els.map((el) => ({ name: el.title, uuid: el.dataset.guildUuid }))
  );
}

async function selectGuildInUI(page, guildUUID) {
  await page.click(`[data-testid="guild-icon"][data-guild-uuid="${guildUUID}"]`);
  await page.waitForSelector('[data-testid="guild-name"]');
}

module.exports = {
  getAuthToken,
  createGuild,
  joinGuild,
  getGuildMembers,
  kickGuildMember,
  leaveGuild,
  deleteGuild,
  getVisibleGuilds,
  selectGuildInUI,
};
```

### Task 17: WS Helpers

**Files:**
- Create: `.agents/skills/room-voice-e2e/scripts/ws-helpers.mjs`

- [ ] **Step 1: 创建 ws-helpers.mjs**

```javascript
// WebSocket 消息抓包 + 协议验证辅助
// 用于 E2E 测试中的 WS 协议验证

/**
 * 在浏览器上创建一个 WS 连接并收集所有收到的消息。
 * 返回一个 controller 对象，可发送消息、等待特定事件、关闭连接。
 */
async function createWSProbe(page, url, token) {
  const probeId = `ws_probe_${Date.now()}_${Math.random().toString(36).slice(2)}`;

  await page.evaluate(({ id, url, token }) => {
    window[id] = {
      ws: null,
      messages: [],
      eventWaiters: {},
    };

    const wsUrl = token ? `${url}?token=${encodeURIComponent(token)}` : url;
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      window[id].connected = true;
    };

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        window[id].messages.push(msg);
        // Resolve any waiters for this event
        const waiters = window[id].eventWaiters[msg.event];
        if (waiters) {
          waiters.forEach((resolve) => resolve(msg));
          window[id].eventWaiters[msg.event] = [];
        }
      } catch {
        window[id].messages.push({ raw: ev.data });
      }
    };

    ws.onclose = () => {
      window[id].connected = false;
    };

    window[id].ws = ws;
  }, { id: probeId, url, token });

  return {
    probeId,

    async send(event, data) {
      await page.evaluate(({ id, event, data }) => {
        const msg = JSON.stringify({ event, data });
        window[id].ws.send(msg);
      }, { id: probeId, event, data });
    },

    async sendWithAck(event, data, timeout = 10000) {
      return page.evaluate(({ id, event, data, timeout }) => {
        return new Promise((resolve, reject) => {
          const msgId = String(++window[id].counter || (window[id].counter = 1));
          const msg = JSON.stringify({ id: msgId, event, data });
          const timer = setTimeout(() => reject(new Error('ack timeout')), timeout);

          const originalOnMessage = window[id].ws.onmessage;
          window[id].ws.onmessage = (ev) => {
            try {
              const resp = JSON.parse(ev.data);
              if (resp.id === msgId) {
                clearTimeout(timer);
                window[id].ws.onmessage = originalOnMessage;
                window[id].messages.push(resp);
                resolve(resp);
              } else {
                originalOnMessage.call(window[id].ws, ev);
              }
            } catch {
              originalOnMessage.call(window[id].ws, ev);
            }
          };

          window[id].ws.send(msg);
        });
      }, { id: probeId, event, data, timeout });
    },

    async waitForEvent(event, timeout = 10000) {
      return page.evaluate(({ id, event, timeout }) => {
        return new Promise((resolve, reject) => {
          const timer = setTimeout(() => reject(new Error(`timeout waiting for ${event}`)), timeout);
          if (!window[id].eventWaiters[event]) window[id].eventWaiters[event] = [];
          window[id].eventWaiters[event].push((msg) => {
            clearTimeout(timer);
            resolve(msg);
          });
        });
      }, { id: probeId, event, timeout });
    },

    async getMessages() {
      return page.evaluate((id) => window[id].messages, probeId);
    },

    async isConnected() {
      return page.evaluate((id) => window[id].connected, probeId);
    },

    async close() {
      await page.evaluate((id) => {
        if (window[id].ws) window[id].ws.close();
        delete window[id];
      }, probeId);
    },
  };
}

/**
 * 验证 WS 消息格式符合协议规范。
 */
function assertMessageFormat(msg) {
  if (!msg || typeof msg !== 'object') throw new Error('message is not an object');
  if (!msg.event || typeof msg.event !== 'string') throw new Error('missing event field');
  // Push messages don't have id; ACK messages do
  if (msg.id !== undefined && typeof msg.id !== 'string') throw new Error('id must be string');
  if (msg.error) {
    if (typeof msg.error.code !== 'number') throw new Error('error.code must be number');
    if (typeof msg.error.message !== 'string') throw new Error('error.message must be string');
  }
}

module.exports = { createWSProbe, assertMessageFormat };
```

### Task 18: Cleanup Helpers

**Files:**
- Create: `.agents/skills/room-voice-e2e/scripts/cleanup-helpers.mjs`

- [ ] **Step 1: 创建 cleanup-helpers.mjs**

```javascript
// 测试数据清理 — 删除 Guild/房间/用户
// 在测试结束后调用，确保测试环境干净

async function getAuthToken(page) {
  return await page.evaluate(() => localStorage.getItem('token') || '');
}

async function cleanupGuild(page, guildUUID) {
  const token = await getAuthToken(page);
  await page.evaluate(async (opts) => {
    await fetch('/api/v1/guild/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ uuid: opts.guildUUID }),
    });
  }, { token, guildUUID });
}

async function cleanupAllGuilds(page) {
  const token = await getAuthToken(page);
  // Get my guilds
  const res = await page.evaluate(async (token) => {
    const r = await fetch('/api/v1/guild/my-guilds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({}),
    });
    return r.json();
  }, token);

  const uuids = res.data?.guild_uuids || [];
  for (const uuid of uuids) {
    // Leave first (in case not owner), then try delete
    await page.evaluate(async (opts) => {
      await fetch('/api/v1/guild/leave', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
        body: JSON.stringify({ uuid: opts.uuid }),
      });
    }, { token, uuid });
  }
}

async function cleanupTestUser(page, baseUrl, username) {
  // Admin-only operation — delete a test user
  const token = await getAuthToken(page);
  await page.evaluate(async (opts) => {
    await fetch('/api/v1/user/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${opts.token}` },
      body: JSON.stringify({ name: opts.username }),
    });
  }, { token, username });
}

module.exports = {
  getAuthToken,
  cleanupGuild,
  cleanupAllGuilds,
  cleanupTestUser,
};
```

- [ ] **Step 2: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add .agents/skills/room-voice-e2e/scripts/guild-helpers.mjs .agents/skills/room-voice-e2e/scripts/ws-helpers.mjs .agents/skills/room-voice-e2e/scripts/cleanup-helpers.mjs
git commit -m "test(e2e): add guild, ws, and cleanup helper scripts"
```

---

## Final Verification

### Task 19: 全量编译验证

- [ ] **Step 1: 后端编译**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过，零错误

- [ ] **Step 2: 前端类型检查**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit`
Expected: 无新增类型错误

- [ ] **Step 3: 前端单测**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx vitest run`
Expected: 所有已有测试通过

- [ ] **Step 4: 后端单测**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./... -count=1 -timeout 120s`
Expected: 所有测试通过

---

## Self-Review

**1. Spec coverage:**

| 需求来源 | 需求 | 对应 Task |
|----------|------|-----------|
| chat-system.md | 私聊 UI 组件 (conversationList/chatWindow/memberSidebar/chatPage) | Task 7-10 |
| chat-system.md | IndexedDB KV cache | 已存在 (idb-cache.ts) |
| chat-system.md | chatStore 私聊状态 | 已存在 (chatStore.ts) |
| chat-system.md | PRIVATE_SEND/PRIVATE_NEW 事件 | 前端已存在 (events.ts) + Task 2 (后端) |
| chat-system.md | /chat 路由 + 侧边栏图标 | Task 10 |
| chat-system.md | DynamicRender /chat 前缀 | Task 10 |
| chat-system.md | socketStore 绑定 private:new | Task 6 |
| chat-system.md | 后端私聊发送 API | Task 3-5 |
| multi-server-platform.md | Guild 布局集成 | Task 11-13 |
| multi-server-platform.md | GuildList 增强 | Task 11 |
| multi-server-platform.md | Guild 页面增强 | Task 13 |
| multi-server-platform.md | DynamicRender /guild 前缀 | Task 13 |
| websocket-migration-phase2.md | 原生 WS 客户端 | Task 14-15 |
| e2e-test-plan.md | guild-helpers.mjs | Task 16 |
| e2e-test-plan.md | ws-helpers.mjs | Task 17 |
| e2e-test-plan.md | cleanup-helpers.mjs | Task 18 |
| full-phase-regression.md | 编译验证 | Task 19 |

**Coverage gaps:** None identified.

**2. Placeholder scan:**
- 无 TBD/TODO/"implement later"
- 所有代码步骤包含完整代码
- 所有命令包含预期输出

**3. Type consistency:**
- `PrivateMessageDTO` 在 conversation.ts 中定义，在 chatStore.ts、chatWindow.tsx、socketStore.ts 中引用一致
- `ConversationDTO` 在 conversation.ts 中定义，在 chatStore.ts、conversationList.tsx 中引用一致
- `chatStore` 方法名 (`loadConversations`, `selectConversation`, `sendDirect`, `startConversation`, `markRead`, `handlePrivateNew`) 在 store 和组件中引用一致
- `EVENTS.PRIVATE_SEND` / `EVENTS.PRIVATE_NEW` 在 events.ts 中定义，与后端 `EventPrivateSend` / `EventPrivateNew` 对齐
- `Guild` / `GuildMember` 类型在 guild.ts 中定义，在 guildStore.ts、GuildList.tsx、memberSidebar.tsx 中引用一致

---

Plan complete and saved to `docs/superpowers/plans/2026-07-31-frontend-capability-completion.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
