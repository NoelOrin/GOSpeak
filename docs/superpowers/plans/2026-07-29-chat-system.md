# Chat System (房间聊天 + 私聊) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add private chat (私聊) support alongside existing room chat, with a new Chat page on the frontend featuring IndexedDB KV cache for all messages, conversation list, member sidebar, and split-panel layout.

**Architecture:**
- **Backend:** Extend `messages` table with `conversation_type`/`conversation_id`/`target_identity` fields; add `conversation_participants` table for session list + unread counts; add `private:send`/`private:new` WS events; add conversation list/messages API.
- **Frontend:** New `/chat` route; IndexedDB KV cache for all messages; chat store with conversation list + message buffers; split-panel layout matching the existing room page pattern.

**Tech Stack:** Go / Gin / GORM / SolidJS / Vite / IndexedDB / Socket.IO / cui-solid Split

---

## File Inventory

### Backend — Files to Create
| File | Purpose |
|------|---------|
| `app/server/internal/model/conversation_participant.go` | ConversationParticipant model |
| `app/server/internal/repository/conversation_repo.go` | ConversationParticipant CRUD |
| `app/server/internal/service/conversation_service.go` | Conversation list + message query |
| `app/server/internal/handler/conversation_handler.go` | Conversation API handlers |
| `app/server/internal/router/routes/conversation/routes.go` | Conversation route registration |

### Backend — Files to Modify
| File | Changes |
|------|---------|
| `app/server/internal/model/message.go` | Add ConversationType, ConversationID, TargetIdentity; make RoomUUID nullable |
| `app/server/internal/repository/message_repo.go` | Add ListByConversation method |
| `app/server/internal/repository/db.go` | Add ConversationParticipant to auto-migrate |
| `app/server/internal/service/message_service.go` | Add SendDirect method; inject ConversationRepository |
| `app/server/internal/signal/events.go` | Add private:send, private:new event constants |
| `app/server/internal/signal/hub.go` | Register OnPrivateMessageSend; wire SendToUser |
| `app/server/internal/signal/message_bridge.go` | Add OnPrivateMessageSend handler |
| `app/server/internal/router/router.go` | Register conversation routes |
| `app/server/server/gin.go` | Wire ConversationRepo, ConversationService, ConversationHandler |

### Frontend — Files to Create
| File | Purpose |
|------|---------|
| `app/web/src/utils/idb-cache.ts` | IndexedDB KV cache wrapper for messages |
| `app/web/src/api/conversation.ts` | Conversation API calls |
| `app/web/src/stores/chatStore.ts` | Chat state: conversations, messages, IDB sync |
| `app/web/src/components/chat/conversationList.tsx` | Left panel: conversation list |
| `app/web/src/components/chat/chatWindow.tsx` | Center: message list + input |
| `app/web/src/components/chat/memberSidebar.tsx` | Right panel: server members |
| `app/web/src/components/chat/chatPage.tsx` | Assembles the full chat layout |
| `app/web/src/pages/(app)/chat/index.tsx` | Route page for /chat |

### Frontend — Files to Modify
| File | Changes |
|------|---------|
| `app/web/src/socket/events.ts` | Add PRIVATE_SEND, PRIVATE_NEW constants |
| `app/web/src/stores/socketStore.ts` | Bind private:new server event |
| `app/web/src/layouts/common/sidebar.tsx` | Add chat icon to sidebar |
| `app/web/src/components/common/dynamicRender.tsx` | Add /chat prefix to PREFIX_MAP |
| `app/web/src/routeTree.gen.ts` | Regenerated |

---

## Task 1: Backend — Message Model Extension

**Files:**
- Modify: `app/server/internal/model/message.go`
- Create: `app/server/internal/model/conversation_participant.go`
- Modify: `app/server/internal/repository/db.go`

**Step 1: Extend Message model**

Add three fields to `Message`, make `RoomUUID` and `GuildUUID` nullable:

```
+ ConversationType string    `gorm:"size:10;index:idx_msg_conversation,priority:1;default:room"`
+ ConversationID   *string   `gorm:"size:32;index:idx_msg_conversation,priority:2"`
+ TargetIdentity   *string   `gorm:"size:64;index"`
  RoomUUID         *string   `gorm:"size:255;index:idx_msg_room_status_id,priority:1"` // changed to *string
  GuildUUID        *string   `gorm:"type:uuid;index"`                                 // changed to *string
  ReplyToID        *string   `gorm:"size:26"`                                         // changed to *string
```

Add composite index line: `index:idx_msg_conversation,priority:1` on `ConversationType` and `priority:2` on `ConversationID`.

MessageStatusActive/Deleted constants stay as-is.

**Step 2: Create ConversationParticipant model**

Fields:
- ConversationID string PK `size:32`
- IdentityA string `size:64;not null`
- IdentityB string `size:64;not null`
- LastMessageID *string `size:26`
- LastContent string `size:200`
- LastSenderIdentity string `size:64`
- LastMessageAt *time.Time
- UnreadCountA int `default:0`
- UnreadCountB int `default:0`
- CreatedAt/UpdatedAt time.Time

Table name: `conversation_participants`

IdentityA/IdentityB must be sorted (lexicographic) so lookups work regardless of who sends first.

**Step 3: Add to autoMigrate**

In `db.go`, add `&model.ConversationParticipant{}` to the AutoMigrate list.

---

## Task 2: Backend — Repository Changes

**Files:**
- Modify: `app/server/internal/repository/message_repo.go`
- Create: `app/server/internal/repository/conversation_repo.go`

**Step 1: Add ListByConversation to MessageRepository**

```go
func (r *MessageRepository) ListByConversation(conversationID, beforeULID string, limit int) ([]model.Message, error) {
    q := r.db.Where("conversation_type = ? AND conversation_id = ? AND status = ?",
        "direct", conversationID, model.MessageStatusActive)
    if beforeULID != "" {
        q = q.Where("id < ?", beforeULID)
    }
    var rows []model.Message
    err := q.Order("id DESC").Limit(limit).Find(&rows).Error
    // reverse to ASC
    return rows, err
}
```

**Step 2: Create ConversationRepository**

Methods:
- `Upsert(cp *model.ConversationParticipant) error` — uses Save (insert or update since PK is set)
- `ListByIdentity(identity string, limit int) ([]model.ConversationParticipant, error)` — WHERE identity_a=? OR identity_b=?, ORDER BY COALESCE(last_message_at, created_at) DESC
- `GetByID(conversationID string) (*model.ConversationParticipant, error)` — First by PK
- `IncrementUnread(conversationID, senderIdentity string) error` — update unread_count_a or unread_count_b += 1 depending on who identity_a is
- `ResetUnread(conversationID, identity string) error` — set unread_count_a=0 or unread_count_b=0
- `UpdateLastMessage(convID, msgID, content, senderID string, t time.Time) error` — update summary fields

---

## Task 3: Backend — MessageService Extension

**Files:**
- Modify: `app/server/internal/service/message_service.go`

**Step 1: Inject ConversationRepository**

Add `conversationRepo *repository.ConversationRepository` to `MessageService` struct.
Update `NewMessageService` to accept and store it.

**Step 2: Add helper and SendDirect**

```go
func generateConversationID(a, b string) string {
    pair := []string{a, b}
    sort.Strings(pair)
    h := sha256.Sum256([]byte(pair[0] + ":" + pair[1]))
    return "direct:" + hex.EncodeToString(h[:8])
}

func (s *MessageService) SendDirect(ctx context.Context, in MessageSendInput) (*MessageDTO, error) {
    // Validate input
    // Generate conversationID
    // Create Message with ConversationType="direct", ConversationID, TargetIdentity
    // Persist message (async path, same as room messages)
    // Upsert ConversationParticipant synchronously (lightweight)
    // Return DTO
}
```

Add `TargetIdentity` field to `MessageSendInput`.

**Step 3: Modify Send to route based on TargetIdentity**

```go
func (s *MessageService) Send(ctx context.Context, in MessageSendInput) (*MessageDTO, error) {
    if in.TargetIdentity != "" {
        return s.SendDirect(ctx, in)
    }
    // existing room message logic unchanged
}
```

**Step 4: Add conversation_participants upsert in the send path**

In `SendDirect`, after creating the message row, call `s.conversationRepo.Upsert()` for new conversations or `IncrementUnread() + UpdateLastMessage()` for existing ones.

---

## Task 4: Backend — WS Events + Hub Changes

**Files:**
- Modify: `app/server/internal/signal/events.go`
- Modify: `app/server/internal/signal/hub.go`
- Modify: `app/server/internal/signal/message_bridge.go`

**Step 1: Add event constants**

```go
EventPrivateSend = "private:send"
EventPrivateNew  = "private:new"
```

**Step 2: Add OnPrivateMessageSend handler in message_bridge.go**

Logic:
1. Parse payload: target_identity, content, reply_to
2. Validate sender is authenticated
3. Rate limit (same 250ms per-user throttle)
4. Call messageSvc.SendDirect with TargetIdentity
5. On success:
   - Broadcast `private:new` to sender
   - Broadcast `private:new` to target via fanout room `__user:{targetIdentity}`
6. Return message ID

**Step 3: Wire user personal rooms in hub.go**

After the WS client is authenticated, the client should join a room named `__user:{identity}`. This allows `BroadcastToRoom` to deliver directly to a specific user.

The `OnPrivateMessageSend` handler uses `h.fanout.BroadcastToRoom("__user:"+targetIdentity, EventPrivateNew, payload)`.

**Step 4: Register handler in hub.go setup**

```go
handler.HandleAck(EventPrivateSend, safeHandlerAck(h.OnPrivateMessageSend))
```

---

## Task 5: Backend — Conversation API + Wiring

**Files:**
- Create: `app/server/internal/handler/conversation_handler.go`
- Create: `app/server/internal/service/conversation_service.go`
- Create: `app/server/internal/router/routes/conversation/routes.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/server/gin.go`

**Step 1: Create ConversationService**

Methods:
- `List(ctx, identity) ([]ConversationDTO, error)` — calls ConversationRepository.ListByIdentity, maps to DTO with per-user unread count
- `GetMessages(ctx, conversationID, identity, before, limit) (*MessageListResult, error)` — validates identity is participant, calls MessageRepository.ListByConversation

**Step 2: Create ConversationHandler**

Handlers:
- `List(c *gin.Context)` — reads username from JWT context
- `Messages(c *gin.Context)` — validates request, delegates to service

**Step 3: Create routes file**

```go
func RegisterProtected(r *gin.RouterGroup, h *handler.ConversationHandler) {
    r.POST("/list", h.List)
    r.POST("/messages", h.Messages)
}
```

**Step 4: Wire in router.go**

Add `Conversation *handler.ConversationHandler` to Handlers struct.
Register: `conversationRoutes.RegisterProtected(protected.Group("/conversation"), h.Conversation)`

**Step 5: Wire in gin.go**

Create instance chain: `conversationRepo -> conversationSvc -> conversationH`.
Pass `conversationRepo` to both `MessageService` and `ConversationService`.

---

## Task 6: Frontend — IndexedDB KV Cache

**Files:**
- Create: `app/web/src/utils/idb-cache.ts`

Create a wrapper around the `idb` library (already exists as `idb` in dependencies or will add):

```ts
// Two object stores:
// 1. conversations: keyPath = "conversationID"  
//    { conversationID, otherIdentity, lastMessage, lastMessageAt, unreadCount }
// 2. messages: autoIncrement, but queried by [conversationID, messageID] via index
//    { conversationID, messageID, content, senderIdentity, senderDisplay, timestamp }
// 3. meta: key-value store for cursor state per conversation

export const chatCache = {
  async getConversations(): Promise<CachedConversation[]>,
  async setConversations(list: CachedConversation[]): void,
  async getMessages(convID: string): Promise<MessageDTO[]>,
  async appendMessages(convID: string, msgs: MessageDTO[]): void,
  async prependMessages(convID: string, msgs: MessageDTO[]): void,
  async getCursor(convID: string): Promise<string | null>,
  async setCursor(convID: string, cursor: string): void,
  async clear(): void,
};
```

The caching strategy is write-through:
- On first load: read cached conversations from IDB for instant display, then refresh from API in background
- When API returns messages: update IDB cache silently
- When receiving WS `private:new`: append to IDB + update store
- Max cache: keep last 200 messages per conversation in IDB

---

## Task 7: Frontend — API + Store + Events

**Files:**
- Create: `app/web/src/api/conversation.ts`
- Create: `app/web/src/stores/chatStore.ts`
- Modify: `app/web/src/socket/events.ts`
- Modify: `app/web/src/stores/socketStore.ts`

**Step 1: Add event constants**

```ts
PRIVATE_SEND = "private:send"
PRIVATE_NEW = "private:new"
```

**Step 2: Create conversation API**

```ts
export async function listConversations(): Promise<ConversationDTO[]> { ... }
export async function getConversationMessages(convID: string, before?: string): Promise<MessageListResult> { ... }
export async function markConversationRead(convID: string): Promise<void> { ... }
```

**Step 3: Create chatStore**

State shape:
```ts
interface ChatState {
  conversations: ConversationDTO[];
  activeConversationID: string | null;
  messages: Record<string, MessageDTO[]>; // keyed by conversationID
  hasMore: Record<string, boolean>;       // per-conversation pagination flag
  loadingList: boolean;
  loadingMessages: Record<string, boolean>;
}
```

Store methods:
- `loadConversations()` — API + IDB cache cycle
- `selectConversation(id)` — load messages, reset unread
- `loadMoreMessages(convID)` — paginate older messages
- `sendMessage(convID, content)` — emit WS private:send
- `handleNewMessage(dto)` — called on private:new WS event
- `handleNewRoomMessage(room, dto)` — called on message:new WS event
- `markRead(convID)` — API call + store update

**Step 4: Wire WS binding in socketStore.ts**

```ts
adapter.onServerEvent(EVENTS.PRIVATE_NEW, (dto: MessageDTO) => {
  chatStore.handleNewMessage(dto);
});
```

---

## Task 8: Frontend — Chat Page Components

**Files:**
- Create: `app/web/src/components/chat/conversationList.tsx`
- Create: `app/web/src/components/chat/chatWindow.tsx`
- Create: `app/web/src/components/chat/memberSidebar.tsx`
- Create: `app/web/src/components/chat/chatPage.tsx`

**Step 1: ConversationList**

Left panel inside DynamicRender's prev slot. Shows:
- Section: "房间消息" — lists guild's text-enabled rooms
- Section: "私聊" — lists direct conversations, each with: other user's display name, last message preview, timestamp, unread badge
- Active conversation highlighted
- Click to switch activeConversation

**Step 2: ChatWindow**

Main center area in the right split slot:
- Top bar: conversation name (room name or user display name)
- Middle: scrollable message list
  - Messages rendered as bubbles (own messages right-aligned, others left-aligned)
  - Auto-scroll to bottom on new messages
  - Load-more on scroll-to-top (infinite scroll)
  - Timestamp separators for different dates
- Bottom: message input
  - Text area + send button
  - Enter to send, Shift+Enter for newline
  - Sending disabled when empty

**Step 3: MemberSidebar**

Right panel in the right split slot. Reuses the same pattern as `components/room/components/memberSidebar.tsx`:
- Shows all members of the current guild
- Each member: avatar + display name
- Click to start a private conversation with that member

**Step 4: ChatPage Assembly**

```tsx
const ChatPage = () => {
  const { conversations, activeConversationID, messages } = chatStore;
  
  return (
    <div class="flex flex-row w-full h-full">
      <ChatWindow
        conversationID={activeConversationID()}
        messages={messages()[activeConversationID()] || []}
        onSend={(content) => chatStore.sendMessage(activeConversationID(), content)}
        onLoadMore={() => chatStore.loadMoreMessages(activeConversationID())}
      />
      <MemberSidebar onStartChat={(identity) => chatStore.startConversation(identity)} />
    </div>
  );
};
```

---

## Task 9: Frontend — Routing

**Files:**
- Create: `app/web/src/pages/(app)/chat/index.tsx`
- Modify: `app/web/src/layouts/common/sidebar.tsx`
- Modify: `app/web/src/components/common/dynamicRender.tsx`
- Modify: `app/web/src/routeTree.gen.ts` (regenerate)

**Step 1: Create chat route page**

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

**Step 2: Update sidebar**

Add a MessageSquare icon between channel and settings. This navigates to `/chat`.

**Step 3: Update DynamicRender**

Add `["/chat", ChatConversationList]` to PREFIX_MAP. Create a thin `ChatConversationList` wrapper that renders the `ConversationList` component as the left panel.

**Step 4: Regenerate route tree**

```bash
cd app/web && npx @tanstack/router-cli generate
```

---

## Spec Coverage

| Requirement | Implementation |
|-------------|---------------|
| IndexedDB KV cache for all messages | Task 6 (idb-cache.ts) + Task 7 (chatStore write-through) |
| New Chat page with split layout | Task 8 (chatPage) + Task 9 (routing) |
| Left split: conversations | Task 8 (conversationList in DynamicRender prev slot) |
| Right split right side: server members | Task 8 (memberSidebar) |
| Rest: chat window | Task 8 (chatWindow) |
| Room messages also cached | Task 7 (handleNewRoomMessage) |
| 一切信息以远程为准 | Task 7 (API-first, IDB as write-through cache) |
| Private chat model | Task 1 (Message fields + ConversationParticipant) |
| Unread counts | Task 1 (conversation_participants) |
| WS events for private chat | Task 4 (private:send / private:new) |
| Conversation list API | Task 5 (conversation API) |
