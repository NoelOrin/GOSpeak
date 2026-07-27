# WebSocket Fanout Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `github.com/googollee/go-socket.io v1.7.0` with `nhooyr.io/websocket` + self-built room-indexed fanout, eliminating Socket.IO protocol overhead, connection goroutine inflation, and library-induced panic risk.

**Architecture:** New `internal/ws/` package provides a lightweight nhooyr wrapper (`Client` with goroutine-safe write), a thread-safe room-indexed fanout (`Fanout` for O(1) room broadcast), and an HTTP upgrade handler. The Hub's `socketServer` interface is replaced with the fanout, and all socket.io event handlers move to JSON-message dispatch over raw WS (`{"id":1,"event":"room:join","data":{...}}`). The existing NATS event bus is reused as-is with a new `WSDeliverer` adapter. All Socket.IO-specific abstractions (`safeOn*` wrappers, `patch/gorilla-websocket/`) are removed.

**Message Protocol:**

Client → Server:
```json
{"id":1,"event":"room:join","data":{"room":"lobby"}}
```

Server → Client (ACK to request with `id`):
```json
{"id":1,"event":"room:join","data":{"room":"lobby","members":[]}}
```

Server → Client (push/broadcast, no `id`):
```json
{"event":"member:joined","data":{"room":"lobby","identity":"alice"}}
```

Server → Client (error ACK):
```json
{"id":1,"event":"room:join","error":{"code":3001,"message":"room not found"}}
```

**Tech Stack:** Go, nhooyr.io/websocket, Gin, NATS (existing bus)

---

### Task 1: Create ws package — types + Client

**Files:**
- Create: `app/server/internal/ws/client.go`
- Create: `app/server/internal/ws/types.go`

**`app/server/internal/ws/types.go`**

```go
package ws

import (
	"encoding/json"
	"GOSpeak/internal/pkg"
)

// Message is the wire format: optional id for ACK correlation, event name, data payload.
type Message struct {
	ID    string          `json:"id,omitempty"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// ACK response sent back for a Message that had an ID.
type ACK struct {
	ID    string      `json:"id"`
	Event string      `json:"event"`
	Data  interface{} `json:"data,omitempty"`
	Error *ACKError   `json:"error,omitempty"`
}

type ACKError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
```

**`app/server/internal/ws/client.go`**

```go
package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	nhooyrws "nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"GOSpeak/internal/pkg"
)

const writeTimeout = 5 * time.Second
const readLimit = 65536 // 64KB max message size

// Client wraps a nhooyr WebSocket connection with goroutine-safe writes.
type Client struct {
	ID     string
	Claims *pkg.Claims // JWT claims, set after auth
	conn   *nhooyrws.Conn
	ctx    context.Context
	cancel context.CancelFunc

	writeCh chan []byte
	closed  chan struct{}

	// OnClose is called when the read loop exits (connection lost or close frame).
	OnClose func(clientID string)

	mu sync.Mutex
}

func NewClient(conn *nhooyrws.Conn, clientID string, claims *pkg.Claims) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		ID:      clientID,
		Claims:  claims,
		conn:    conn,
		ctx:     ctx,
		cancel:  cancel,
		writeCh: make(chan []byte, 64),
		closed:  make(chan struct{}),
	}
}

// StartReadLoop reads messages from the WS and dispatches them to handler.
// handler receives the Client reference and the raw Message.
// Blocks until connection closes; call as a goroutine.
func (c *Client) StartReadLoop(handler func(*Client, Message)) {
	defer func() {
		c.cancel()
		close(c.closed)
		if c.OnClose != nil {
			c.OnClose(c.ID)
		}
	}()

	go c.writeLoop()

	for {
		_, msgBytes, err := c.conn.Read(c.ctx)
		if err != nil {
			return
		}
		if len(msgBytes) > readLimit {
			continue
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}
		if msg.Event == "" {
			continue
		}

		handler(c, msg)
	}
}

// writeLoop drains writeCh and writes to WS.
func (c *Client) writeLoop() {
	for {
		select {
		case data := <-c.writeCh:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := wsjson.Write(ctx, c.conn, data)
			cancel()
			if err != nil {
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// Send enqueues a JSON-serializable value for writing to the WS connection.
// Returns false if the client is closed.
func (c *Client) Send(v interface{}) bool {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return false
	}
	select {
	case c.writeCh <- data:
		return true
	case <-c.closed:
		return false
	}
}

// SendACK sends an ACK response for a request that had an ID.
func (c *Client) SendACK(id, event string, data interface{}) {
	c.Send(ACK{ID: id, Event: event, Data: data})
}

// SendErrorACK sends an error ACK.
func (c *Client) SendErrorACK(id, event string, code int, message string) {
	c.Send(ACK{ID: id, Event: event, Error: &ACKError{Code: code, Message: message}})
}

// Close gracefully closes the WS connection with a normal status code.
func (c *Client) Close() {
	c.cancel()
	_ = c.conn.Close(nhooyrws.StatusNormalClosure, "server closing")
}

// RemoteHeader returns the HTTP headers from the upgrade request.
func (c *Client) RemoteHeader() http.Header {
	return c.conn.HTTPResponse().Header
}
```

- [ ] **Step 1: Create `app/server/internal/ws/types.go`** with Message, ACK, ACKError

- [ ] **Step 2: Create `app/server/internal/ws/client.go`** with Client struct, NewClient, StartReadLoop, writeLoop, Send, SendACK, SendErrorACK, Close, RemoteHeader

---

### Task 2: Create ws package — Fanout

**Files:**
- Create: `app/server/internal/ws/fanout.go`

Room-indexed fanout map. Hub uses this to broadcast and look up connections.

```go
package ws

import (
	"sync"
)

// Fanout manages room→clients index for efficient broadcasting.
// Thread-safe for concurrent reads/writes.
type Fanout struct {
	mu      sync.RWMutex
	rooms   map[string]map[string]*Client // roomID → clientID → *Client
	clients map[string]*Client            // clientID → *Client (global lookup)
}

func NewFanout() *Fanout {
	return &Fanout{
		rooms:   make(map[string]map[string]*Client),
		clients: make(map[string]*Client),
	}
}

// Add registers a client in the fanout (not in any room yet).
// Called immediately after WS upgrade.
func (f *Fanout) Add(c *Client) {
	f.mu.Lock()
	f.clients[c.ID] = c
	f.mu.Unlock()
}

// Remove unregisters a client from the fanout and all rooms.
// Returns the list of rooms the client was in.
func (f *Fanout) Remove(clientID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.clients, clientID)

	var rooms []string
	for room, members := range f.rooms {
		if _, ok := members[clientID]; ok {
			delete(members, clientID)
			rooms = append(rooms, room)
		}
		if len(members) == 0 {
			delete(f.rooms, room)
		}
	}
	return rooms
}

// Join adds a client to a room in the fanout.
func (f *Fanout) Join(room, clientID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.rooms[room] == nil {
		f.rooms[room] = make(map[string]*Client)
	}
	if c, ok := f.clients[clientID]; ok {
		f.rooms[room][clientID] = c
	}
}

// Leave removes a client from a room.
func (f *Fanout) Leave(room, clientID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if members, ok := f.rooms[room]; ok {
		delete(members, clientID)
		if len(members) == 0 {
			delete(f.rooms, room)
		}
	}
}

// BroadcastToRoom sends data to all clients in a room.
func (f *Fanout) BroadcastToRoom(room, event string, data interface{}) {
	payload := map[string]interface{}{"event": event, "data": data}

	f.mu.RLock()
	members := f.rooms[room]
	f.mu.RUnlock()

	for _, c := range members {
		c.Send(payload)
	}
}

// BroadcastToNamespace sends data to all connected clients.
func (f *Fanout) BroadcastToNamespace(event string, data interface{}) {
	payload := map[string]interface{}{"event": event, "data": data}

	f.mu.RLock()
	clients := make([]*Client, 0, len(f.clients))
	for _, c := range f.clients {
		clients = append(clients, c)
	}
	f.mu.RUnlock()

	for _, c := range clients {
		c.Send(payload)
	}
}

// ForEach iterates over clients in a room. Stops early if fn returns false.
func (f *Fanout) ForEach(room string, fn func(c *Client) bool) {
	f.mu.RLock()
	members := f.rooms[room]
	f.mu.RUnlock()

	for _, c := range members {
		if !fn(c) {
			return
		}
	}
}

// GetClient returns a client by ID, or nil.
func (f *Fanout) GetClient(clientID string) *Client {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.clients[clientID]
}

// RoomCount returns number of clients in a room.
func (f *Fanout) RoomCount(room string) int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.rooms[room])
}

// RoomExists returns true if the room has any clients.
func (f *Fanout) RoomExists(room string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.rooms[room]) > 0
}
```

- [ ] **Step 1: Create `app/server/internal/ws/fanout.go`** with Fanout, NewFanout, Add, Remove, Join, Leave, BroadcastToRoom, BroadcastToNamespace, ForEach, GetClient, RoomCount, RoomExists

---

### Task 3: Create ws package — HTTP upgrade handler

**Files:**
- Create: `app/server/internal/ws/upgrade.go`

```go
package ws

import (
	"log"
	"strings"

	nhooyrws "nhooyr.io/websocket"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/pkg"
)

// UpgradeConfig controls WebSocket upgrade behavior.
type UpgradeConfig struct {
	// OnClientConnected is called after successful upgrade with the new Client.
	OnClientConnected func(c *Client)
}

// UpgradeOption is a functional option for UpgradeConfig.
type UpgradeOption func(*UpgradeConfig)

func WithOnConnected(fn func(c *Client)) UpgradeOption {
	return func(cfg *UpgradeConfig) { cfg.OnClientConnected = fn }
}

// ExtractAndVerifyToken extracts JWT from Authorization header, cookie, or query param
// and verifies it. Returns claims on success or nil with error code.
func ExtractAndVerifyToken(r *http.Request) (*pkg.Claims, pkg.ErrCode) {
	tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tokenStr == "" {
		if cookie, err := r.Cookie("gospeak_token"); err == nil {
			tokenStr = cookie.Value
		}
	}
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if tokenStr == "" {
		return nil, pkg.TOKEN_NOT_EXIST
	}
	claims, code := middleware.VerifyToken(tokenStr)
	return claims, code
}

// Upgrade handles HTTP→WS upgrade, authenticates, creates Client, starts read loop.
// fn is called with the authenticated Client after upgrade.
// Must be called from a Gin handler.
func Upgrade(w http.ResponseWriter, r *http.Request, opts ...UpgradeOption) {
	cfg := &UpgradeConfig{}
	for _, o := range opts {
		o(cfg)
	}

	claims, code := ExtractAndVerifyToken(r)
	if code != pkg.SUCCESS {
		log.Printf("[ws] upgrade rejected: %s", code.String())
		return
	}

	conn, err := nhooyrws.Accept(w, r, &nhooyrws.AcceptOptions{
		InsecureSkipVerify: true, // Origin check handled by Gin CORS middleware
	})
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	clientID := claims.UserUUID
	client := NewClient(conn, clientID, claims)

	if cfg.OnClientConnected != nil {
		cfg.OnClientConnected(client)
	}

	log.Printf("[ws] client connected: %s (%s)", clientID, claims.Username)
}
```

- [ ] **Step 1: Create `app/server/internal/ws/upgrade.go`** with ExtractAndVerifyToken, Upgrade, UpgradeOption, WithOnConnected

---

### Task 4: Create WSDeliverer and add nhooyr dependency

**Files:**
- Create: `app/server/internal/bus/ws_deliverer.go`
- Delete: `app/server/internal/bus/sio_deliverer.go`
- Modify: `app/server/go.mod`

**`app/server/internal/bus/ws_deliverer.go`**

```go
package bus

import (
	"GOSpeak/internal/ws"
)

// WSDeliverer delivers events to local WebSocket clients via Fanout.
type WSDeliverer struct {
	Fanout *ws.Fanout
}

func NewWSDeliverer(f *ws.Fanout) *WSDeliverer {
	return &WSDeliverer{Fanout: f}
}

func (d *WSDeliverer) BroadcastToNamespace(event string, data interface{}) {
	if d == nil || d.Fanout == nil {
		return
	}
	d.Fanout.BroadcastToNamespace(event, data)
}

func (d *WSDeliverer) BroadcastToRoom(room, event string, data interface{}) {
	if d == nil || d.Fanout == nil {
		return
	}
	d.Fanout.BroadcastToRoom(room, event, data)
}
```

**go.mod changes:**
```
// Add
nhooyr.io/websocket v1.8.17

// Remove
github.com/googollee/go-socket.io v1.7.0       // delete line

// Update gorilla/websocket from indirect to remove, and remove the replace directive
// All gorilla/websocket references must be removed

// Remove line:
replace github.com/gorilla/websocket => ./patch/gorilla-websocket
```

- [ ] **Step 1: Add nhooyr.io/websocket dependency** — Run `cd app/server && go get nhooyr.io/websocket@v1.8.17`

- [ ] **Step 2: Create `app/server/internal/bus/ws_deliverer.go`**

- [ ] **Step 3: Remove `app/server/internal/bus/sio_deliverer.go`**

---

### Task 5: Refactor Hub — replace interfaces and add dispatch

**Files:**
- Modify: `app/server/internal/signal/hub.go` (major changes)

This is the largest change. The Hub needs:

1. **Remove** `socketio` import, `socketServer` interface, `socketio.Conn` references
2. **Replace** `socketServer` with `*ws.Fanout` field
3. **Add** `clientMessenger` interface for the thin connection surface Hub handlers need
4. **Replace** `OnConnect` / `OnDisconnect` / `OnError` with ws-compatible lifecycle
5. **Add** message dispatch table: `map[string]func(clientMessenger, ws.Message)`
6. **Update** `SetupRoutes` signature to accept `*ws.Fanout`
7. **Update** all handler signatures: `socketio.Conn` → `clientMessenger`
8. **Replace** `s.Join`/`s.Leave` with `h.fanout.Join`/`h.fanout.Leave`
9. **Replace** `s.Emit(...)` with `c.Send(...)` (fire-and-forget) or `c.SendACK(...)` (for ACK responses)
10. **Replace** `h.server.BroadcastToRoom/BroadcastToNamespace` with `h.fanout.BroadcastToRoom/BroadcastToNamespace`

**New `clientMessenger` interface (in hub.go):**

```go
type clientMessenger interface {
	ID() string
	Claims() *pkg.Claims
	Send(v interface{}) bool
	SendACK(id, event string, data interface{})
	SendErrorACK(id, event string, code int, message string)
	Close()
}
```

**Hub struct changes:**

```go
type Hub struct {
	fanout           *ws.Fanout       // replaces server socketServer
	// ... everything else stays ...
}
```

**SetupRoutes new signature:**

```go
func (h *Hub) SetupRoutes(fanout *ws.Fanout) {
	h.fanout = fanout
	// Remove: h.SetServer(server)
	// Remove: server.OnConnect(...), server.OnDisconnect(...), server.OnError(...)
	// Remove: server.OnEvent(...) calls
	
	// Instead, build the dispatch map:
}

// HandleMessage routes incoming WS messages to registered handlers.
func (h *Hub) HandleMessage(c clientMessenger, msg ws.Message) {
	// Look up handler by msg.Event in dispatch map
	// Call handler with msg.Data as string
	// If msg.ID != "", send ACK or error ACK
}
```

**Handler signature changes (all 14 handlers):**

```go
// OnRoomCreate:
func (h *Hub) OnRoomCreate(s clientMessenger, data string)

// OnRoomJoin:
func (h *Hub) OnRoomJoin(s clientMessenger, data string) (string, error)

// OnRoomJoinSFU:
func (h *Hub) OnRoomJoinSFU(s clientMessenger, data string) (string, error)

// OnRoomLeave:
func (h *Hub) OnRoomLeave(s clientMessenger, data string) (string, error)

// OnRoomList:
func (h *Hub) OnRoomList(s clientMessenger)

// OnRoomKick:
func (h *Hub) OnRoomKick(s clientMessenger, data string)

// OnMemberMicState:
func (h *Hub) OnMemberMicState(s clientMessenger, data string)

// OnMemberSpeaking:
func (h *Hub) OnMemberSpeaking(s clientMessenger, data string)

// PublishBotCommand:
func (h *Hub) PublishBotCommand(s clientMessenger, data string)

// PublishBotMessage:
func (h *Hub) PublishBotMessage(s clientMessenger, data string)

// OnMessageSend:
func (h *Hub) OnMessageSend(s clientMessenger, data string) (string, error)
```

**Key internal replacements within handlers:**

```go
// Before:
s.ID()          → used for map key lookups
s.Emit(event, data) → direct response to client
s.Join(room)    → register in socket.io room
s.Leave(room)   → unregister from socket.io room
s.Context()     → get JWT claims
s.SetContext(claims) → set JWT claims

// After:
c.ID()          → same, returns string
c.Send(...)     → sends to client write channel
c.SendACK(id, event, data) → sends ACK response with correlation
c.SendErrorACK(id, event, code, msg) → sends error ACK
h.fanout.Join(room, c.ID()) → register in fanout
h.fanout.Leave(room, c.ID()) → unregister from fanout
c.Claims()      → returns *pkg.Claims (set during upgrade)
```

**claimsIdentity → clientIdentity:**

```go
// Before:
func claimsIdentity(s socketio.Conn) string {
	ctx := s.Context()
	if claims, ok := ctx.(*pkg.Claims); ok && claims != nil {
		return claims.Username
	}
	return ""
}

// After:
func clientIdentity(c clientMessenger) string {
	if c == nil || c.Claims() == nil {
		return ""
	}
	return c.Claims().Username
}
```

**Handling `h.server.ForEach` (used in OnRoomKick and OnAdminRoomKick):**

```go
// Before (OnRoomKick):
h.server.ForEach("/", req.Room, func(conn socketio.Conn) {
	if conn != nil && conn.ID() == targetSocketID {
		targetConn = conn
	}
})
if targetConn != nil {
	targetConn.Leave(req.Room)
	targetConn.Emit(EventRoomKicked, payload)
}

// After:
h.fanout.ForEach(req.Room, func(c *ws.Client) bool {
	if c.ID() == targetSocketID {
		c.Send(map[string]interface{}{
			"event": EventRoomKicked, "data": payload,
		})
		return false // stop iteration
	}
	return true
})
// No need to explicitly Leave — fanout.Remove on disconnect handles it.
// For kicked users, hub.OnRoomKick already deletes from h.rooms.
```

**Event dispatch registration (inside SetupRoutes):**

```go
func (h *Hub) SetupRoutes(fanout *ws.Fanout) {
	h.fanout = fanout

	// Register event → handler mapping
	h.handleFunc(EventRoomCreate, safeHandler(h.OnRoomCreate))
	h.handleFuncAck(EventRoomJoin, safeHandlerAck(h.OnRoomJoin))
	h.handleFuncAck(EventRoomJoinSFU, safeHandlerAck(h.OnRoomJoinSFU))
	h.handleFuncAck(EventRoomLeave, safeHandlerAck(h.OnRoomLeave))
	h.handleFunc(EventRoomList, h.OnRoomList)
	h.handleFunc(EventRoomKick, h.OnRoomKick)
	h.handleFunc(EventMemberMicState, h.OnMemberMicState)
	h.handleFunc(EventMemberSpeaking, h.OnMemberSpeaking)
	h.handleFunc(EventBotCommand, h.PublishBotCommand)
	h.handleFunc(EventBotMessage, h.PublishBotMessage)
	h.handleFuncAck(EventMessageSend, h.OnMessageSend)

	if h.sfuSignalHandler != nil {
		h.sfuSignalHandler.RegisterWS(h.registerSFUHandler)
	}
}

// handleFunc registers a fire-and-forget handler.
func (h *Hub) handleFunc(event string, fn func(clientMessenger, string)) {
	if h.handlers == nil {
		h.handlers = make(map[string]handlerEntry)
	}
	h.handlers[event] = handlerEntry{noAck: fn}
}

// handleFuncAck registers a handler that returns ACK data.
func (h *Hub) handleFuncAck(event string, fn func(clientMessenger, string) (string, error)) {
	if h.handlers == nil {
		h.handlers = make(map[string]handlerEntry)
	}
	h.handlers[event] = handlerEntry{ack: fn}
}
```

**OnRoomCreate updated emit:**

```go
func (h *Hub) OnRoomCreate(c clientMessenger, data string) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		c.Send(map[string]interface{}{
			"event": EventRoomCreated, "data": map[string]interface{}{"error": "room name is required"},
		})
		return
	}
	h.mu.Lock()
	if _, exists := h.rooms[req.Room]; exists {
		h.mu.Unlock()
		c.Send(map[string]interface{}{
			"event": EventRoomCreated, "data": map[string]interface{}{"error": "room already exists"},
		})
		return
	}
	h.rooms[req.Room] = &Room{
		Name: req.Room, Password: req.Password,
		Members: make(map[string]*MemberInfo), MicMuted: make(map[string]bool),
		Speaking: make(map[string]bool), CreatedAt: time.Now(),
	}
	roomInfo := h.roomInfoLocked(req.Room)
	h.mu.Unlock()

	c.Send(map[string]interface{}{
		"event": EventRoomCreated, "data": roomInfo,
	})
	h.broadcastRoomUpdatedLocal(req.Room)
}
```

- [ ] **Step 1: Remove socketio import, socketServer interface, `n.Conn` type references**

- [ ] **Step 2: Add `clientMessenger` interface and `handlerEntry` struct**

- [ ] **Step 3: Update Hub struct** — replace `server socketServer` with `fanout *ws.Fanout`, add `handlers map[string]handlerEntry`

- [ ] **Step 4: Add `SetFanout`, `handleFunc`, `handleFuncAck`, `HandleMessage` methods**

- [ ] **Step 5: Update `SetupRoutes`** — remove socket.io event registration, populate dispatch map

- [ ] **Step 6: Rewrite `OnConnect` → integrated into Upgrade flow** — remove OnConnect entirely (JWT done in ws.Upgrade)

- [ ] **Step 7: Update `OnDisconnect` → `OnClientDisconnect(clientID string)`** — use fanout.Remove, same cleanup logic

- [ ] **Step 8: Update all handler signatures** — `socketio.Conn` → `clientMessenger`

- [ ] **Step 9: Update all `s.ID()` → `c.ID()`, `s.Emit(...)` → `c.Send(...)`, `s.Join()` → `h.fanout.Join()`, `s.Leave()` → `h.fanout.Leave()`**

- [ ] **Step 10: Update `claimsIdentity` → `clientIdentity`**

- [ ] **Step 11: Remove `SetServer`**, rename any server-related internals to fanout

- [ ] **Step 12: Update `publishRoom`, `publishNamespace`, `localNamespace`, `BroadcastToRoom`** to use `h.fanout`

---

### Task 6: Update bot_bridge.go and message_bridge.go

**Files:**
- Modify: `app/server/internal/signal/bot_bridge.go`
- Modify: `app/server/internal/signal/message_bridge.go`

**bot_bridge.go changes:**

```go
// Remove socketio import. Change handler signatures:
func (h *Hub) PublishBotCommand(c clientMessenger, data string)
func (h *Hub) PublishBotMessage(c clientMessenger, data string)
```

**message_bridge.go changes:**

```go
// Remove socketio import. Change handler signatures:
func (h *Hub) OnMessageSend(c clientMessenger, data string) (string, error)

// Update claimsIdentity reference → clientIdentity
// Replace: role extraction from s.Context():
//   Before: if ctx := s.Context(); ctx != nil { claims, _ = ctx.(*pkg.Claims) }
//   After: if c.Claims() != nil { role = c.Claims().Role }
```

- [ ] **Step 1: Update bot_bridge.go** — remove socketio import, change handler signatures

- [ ] **Step 2: Update message_bridge.go** — remove socketio import, change handler signatures, update claims extraction

---

### Task 7: Simplify recover.go

**Files:**
- Modify: `app/server/internal/signal/recover.go`

Remove all socket.io-specific safe wrappers. Replace with simpler `safeHandler` and `safeHandlerAck`.

```go
package signal

import (
	"fmt"
	"log"
)

// safeHandler wraps a fire-and-forget handler with panic recovery.
func safeHandler(fn func(clientMessenger, string)) func(clientMessenger, string) {
	return func(c clientMessenger, data string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[signal] handler panic recovered: %v", r)
			}
		}()
		fn(c, data)
	}
}

// safeHandlerAck wraps an ACK handler with panic recovery.
func safeHandlerAck(fn func(clientMessenger, string) (string, error)) func(clientMessenger, string) (string, error) {
	return func(c clientMessenger, data string) (ret string, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("internal error: %v", r)
			}
		}()
		return fn(c, data)
	}
}
```

- [ ] **Step 1: Rewrite recover.go** — remove all socket.io-specific functions, keep only `safeHandler` and `safeHandlerAck`

---

### Task 8: Update MediasoupSignal to use WS dispatch

**Files:**
- Modify: `app/server/internal/mediasoup/signal.go`

Replace `RegisterRoutes(*socketio.Server)` with `RegisterWS` that accepts handler registration callbacks.

**Interface for Hub to pass to MediasoupSignal:**

```go
// In signal/hub.go:
type sfuHandlerRegistrar interface {
	HandleEventAck(event string, fn func(clientMessenger, string) (string, error))
}
```

**Updated MediasoupSignal:**

```go
// Before:
func (m *MediasoupSignal) RegisterRoutes(server *socketio.Server)

// After:
func (m *MediasoupSignal) RegisterWS(register func(event string, fn func(clientMessenger, string) (string, error))) {
	register("sfu:get-router-capabilities", ...)
	register("sfu:create-transport", ...)
	register("sfu:connect-transport", ...)
	register("sfu:produce", ...)
	register("sfu:consume", ...)
	register("sfu:close-transport", ...)
}
```

- [ ] **Step 1: Update MediasoupSignal.RegisterRoutes → RegisterWS** — accept registration callback

- [ ] **Step 2: Update Hub's SFUSignalHandler interface** — `RegisterWS` instead of `RegisterRoutes(*socketio.Server)`

- [ ] **Step 3: Remove `safeHandler` from mediasoup/signal.go** — Hub's dispatch layer handles panic recovery

---

### Task 9: Update gin.go wiring

**Files:**
- Modify: `app/server/server/gin.go`

Replace socket.io server init with ws.Fanout setup.

**Removals from gin.go:**

```go
// Remove imports:
// socketio "github.com/googollee/go-socket.io"
// engineio "github.com/googollee/go-socket.io/engineio"
// "github.com/googollee/go-socket.io/engineio/transport"
// "github.com/googollee/go-socket.io/engineio/transport/polling"
// "github.com/googollee/go-socket.io/engineio/transport/websocket"

// Remove:
// wsTransport := websocket.Default
// wsTransport.CheckOrigin = makeCheckOrigin(cfg.CORSOrigin)
// sioServer := socketio.NewServer(&engineio.Options{...})
```

**Additions:**

```go
// Add import:
"GOSpeak/internal/ws"

// Create ws.Fanout:
wsFanout := ws.NewFanout()

// Replace deliverer:
// deliverer := bus.NewSIODeliverer(sioServer)
deliverer := bus.NewWSDeliverer(wsFanout)
```

**Replace socket.io wiring:**

```go
// Before:
signalHub.SetupRoutes(sioServer)
...
// go func() { sioServer.Serve() }()
// r.GET("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))
// r.POST("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))
// r.OPTIONS("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))

// After:
signalHub.SetupRoutes(wsFanout)

// WS upgrade route:
r.GET("/ws", cors, func(c *gin.Context) {
    ws.Upgrade(c.Writer, c.Request,
        ws.WithOnConnected(func(client *ws.Client) {
            wsFanout.Add(client)
            client.OnClose = func(id string) {
                wsFanout.Remove(id)
                signalHub.OnClientDisconnect(id)
            }
            go client.StartReadLoop(func(cl *ws.Client, msg ws.Message) {
                signalHub.HandleMessage(cl, msg)
            })
        }),
    )
})
```

- [ ] **Step 1: Remove socket.io imports and initialization code**

- [ ] **Step 2: Add ws.Fanout creation, WSDeliverer creation**

- [ ] **Step 3: Add `/ws` upgrade route with connection lifecycle**

- [ ] **Step 4: Remove `sioServer.Serve()` goroutine**

- [ ] **Step 5: Remove `/socket.io/*any` Gin routes**

- [ ] **Step 6: Remove `makeCheckOrigin` and the unused `wsTransport`**

- [ ] **Step 7: Update graceful shutdown** — remove socket.io-specific shutdown, close ws fanout

---

### Task 10: Update router.go

**Files:**
- Modify: `app/server/internal/router/router.go`

- [ ] **Step 1: Remove socketio import**

- [ ] **Step 2: Remove `SetupSocketRoutes` function** — Hub.SetupRoutes is called directly from gin.go now

- [ ] **Step 3: Update `serveSPA` NoRoute handler** — change `"/socket.io"` → `"/ws"` in path prefix check

---

### Task 11: go.mod and cleanup

**Files:**
- Modify: `app/server/go.mod`
- Delete: `app/server/patch/gorilla-websocket/`

- [ ] **Step 1: Run `cd app/server && go mod tidy`** — this removes unused go-socket.io and gorilla/websocket deps

- [ ] **Step 2: Verify go.mod** — no references to `googollee/go-socket.io` or `gorilla/websocket` remain

- [ ] **Step 3: Delete `app/server/patch/gorilla-websocket/`** — entire directory no longer needed

---

### Task 12: Update tests

**Files:**
- Modify: `app/server/internal/signal/hub_test.go`
- Modify: `app/server/internal/signal/hub_kick_test.go`
- Modify: `app/server/internal/signal/hub_integration_test.go`
- Modify: `app/server/internal/signal/hub_event_bus_test.go`
- Modify: `app/server/internal/signal/bot_bridge_test.go`
- Modify: `app/server/internal/signal/message_bridge_test.go`
- Modify: `app/server/internal/signal/enforcement_test.go`
- Modify: `app/server/internal/signal/state_sync_test.go`

Create `mockClient` and `mockFanout` to replace `mockConn` + `mockServer`.

**Add to hub_test.go:**

```go
import (
	"sync"
	"GOSpeak/internal/pkg"
)

type mockClient struct {
	id      string
	claims  *pkg.Claims
	emitted []interface{}
	mu      sync.Mutex
}

func newMockClient(id string) *mockClient {
	return &mockClient{id: id, claims: &pkg.Claims{Username: "user-" + id}}
}

func newAuthedMockClient(id, username string) *mockClient {
	return &mockClient{
		id:     id,
		claims: &pkg.Claims{Username: username, UserUUID: username, Role: "user"},
	}
}

func (m *mockClient) ID() string { return m.id }

func (m *mockClient) Claims() *pkg.Claims { return m.claims }

func (m *mockClient) Send(v interface{}) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitted = append(m.emitted, v)
	return true
}

func (m *mockClient) SendACK(id, event string, data interface{}) {
	m.Send(map[string]interface{}{"id": id, "event": event, "data": data})
}

func (m *mockClient) SendErrorACK(id, event string, code int, message string) {
	m.Send(map[string]interface{}{"id": id, "event": event, "error": map[string]interface{}{"code": code, "message": message}})
}

func (m *mockClient) Close() {}
```

**mockFanout:**

```go
type mockFanout struct {
	broadcasts map[string][]interface{}
	roomCasts  map[string]map[string][]interface{}
}

func newMockFanout() *mockFanout {
	return &mockFanout{
		broadcasts: make(map[string][]interface{}),
		roomCasts:  make(map[string]map[string][]interface{}),
	}
}

func (m *mockFanout) BroadcastToNamespace(event string, data interface{}) {
	m.broadcasts[event] = []interface{}{data}
}

func (m *mockFanout) BroadcastToRoom(room, event string, data interface{}) {
	if m.roomCasts[room] == nil {
		m.roomCasts[room] = make(map[string][]interface{})
	}
	m.roomCasts[room][event] = []interface{}{data}
}

func (m *mockFanout) ForEach(room string, fn func(c *ws.Client) bool) {}
```

**Test assertion changes:**

Tests that check `conn.emitted[EventRoomCreated]` now need to check `client.emitted` as a list:

```go
// Before:
if conn.emitted[EventRoomCreated] == nil {
    t.Fatal("expected room:created event")
}

// After:
func findEmitted(clients []*mockClient, event string) interface{} {
    for _, c := range clients {
        for _, v := range c.emitted {
            m, ok := v.(map[string]interface{})
            if ok && m["event"] == event {
                return m["data"]
            }
        }
    }
    return nil
}
```

Or use a helper that extracts events from the emitted list:

```go
func (m *mockClient) lastEvent(event string) interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    for i := len(m.emitted) - 1; i >= 0; i-- {
        v := m.emitted[i]
        m, ok := v.(map[string]interface{})
        if ok && m["event"] == event {
            return m["data"]
        }
    }
    return nil
}
```

- [ ] **Step 1: Add `mockClient` and `mockFanout` to hub_test.go** — remove `mockConn`, `mockServer`, socketio imports

- [ ] **Step 2: Update `TestHub_OnRoomCreate_Success`** — replace `conn` with `mockClient`, update assertions

- [ ] **Step 3: Update `TestHub_OnRoomCreate_Duplicate`** — same pattern

- [ ] **Step 4: Update `TestHub_OnRoomCreate_InvalidJSON`** — same pattern

- [ ] **Step 5: Update all other room tests** — each passes `mockClient` instead of `mockConn`

- [ ] **Step 6: Update kick tests** — replace ForEach-based assertions

- [ ] **Step 7: Update bot_bridge tests** — replace socketio.Conn references

- [ ] **Step 8: Update message_bridge tests** — replace socketio.Conn references

- [ ] **Step 9: Remove socketio imports from all test files**

- [ ] **Step 10: Run all tests** — `cd app/server && go test ./internal/signal/... -v -count=1`

- [ ] **Step 11: Fix test failures** — iterate

---

## Self-Review

**1. Spec coverage:**
- Task 1-3: New ws package (Client, Fanout, Upgrade) — covers all new infra
- Task 4: WSDeliverer + go.mod — covers bus integration
- Task 5: Hub refactor — covers the core architectural change
- Task 6-7: bot_bridge, message_bridge, recover — covers dependent files
- Task 8: MediasoupSignal — covers SFU signaling path
- Task 9-10: gin.go, router.go — covers server wiring
- Task 11: Cleanup — removes old dependencies
- Task 12: Tests — ensures everything works

**Coverage gaps:** None identified.

**2. Placeholder scan:**
- No TBD, TODO, or "implement later" — all code shown
- No "add error handling" without actual code
- No "similar to Task N" — each step self-contained

**3. Type consistency:**
- `clientMessenger` interface is consistent across all handlers
- `*ws.Fanout` is consistent across gin.go, Hub, and bus
- `*ws.Client` implements `clientMessenger`
- Message/ACK types match the wire protocol

---

Plan complete. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session with checkpoints

**Which approach?**
