# WebSocket Fanout Migration — Phase 2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `github.com/googollee/go-socket.io v1.7.0` with `nhooyr.io/websocket` + self-built room-indexed fanout, eliminating Socket.IO protocol overhead, connection goroutine inflation, and library-induced panic risk. Builds on top of Phase 1 (Guild multi-server architecture).

**Prerequisites:** Phase 1 (Multi-Server Guild Platform) must be fully implemented and merged. Phase 1 introduced `Guild` as the top-level container for rooms, added `GuildUUID` to Room/Message models, and changed the Signal Hub to use composite `guildUUID:roomName` room keys via `roomKey()`. This Phase 2 plan assumes those changes are in place.

**Architecture:** New `internal/ws/` package provides a lightweight nhooyr wrapper (`Client` with goroutine-safe write), a thread-safe room-indexed fanout (`Fanout` for O(1) room broadcast), and an HTTP upgrade handler. The Hub's `socketServer` interface is replaced with the fanout, and all socket.io event handlers move to JSON-message dispatch over raw WS. The fanout uses Guild-aware composite room keys (`roomKey(guildUUID, roomName)`) consistent with Phase 1. The existing NATS event bus is reused with a new `WSDeliverer` adapter.

**Key integration with Phase 1:**
- Fanout room names use the same `roomKey(guildUUID, roomName)` composite key as Phase 1's Hub
- `RoomRequest` already has `GuildUUID` field (from Phase 1)
- `gin.go` preserves Guild DI alongside WS fanout setup
- Guild event handlers maintain the same `clientMessenger` interface

---

## Message Protocol

Client → Server:
```json
{"id":1,"event":"room:join","data":{"room":"lobby","guild_uuid":"..."}}
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

## File Structure

### 后端新增文件

| 文件 | 职责 |
|------|------|
| `app/server/internal/ws/client.go` | WS Client — goroutine-safe write + read loop |
| `app/server/internal/ws/types.go` | Message/ACK wire format types |
| `app/server/internal/ws/fanout.go` | Guild-aware room-indexed fanout |
| `app/server/internal/ws/upgrade.go` | HTTP→WS upgrade with JWT auth |
| `app/server/internal/bus/ws_deliverer.go` | NATS→WS fanout adapter |

### 后端删除文件

| 文件 | 原因 |
|------|------|
| `app/server/internal/bus/sio_deliverer.go` | 替换为 WSDeliverer |
| `app/server/patch/gorilla-websocket/` | 不再需要（socket.io 删掉后 gorilla 间接依赖也移除） |

### 后端修改文件

| 文件 | 改动 |
|------|------|
| `app/server/internal/signal/hub.go` | 替换 `socketServer` → `*ws.Fanout`；所有 handler 签名改为 `clientMessenger`；移除 socket.io 引用 |
| `app/server/internal/signal/types.go` | 保留 GuildUUID（Phase 1 已加）；移除 socket.io 依赖类型 |
| `app/server/internal/signal/recover.go` | 替换 socket.io safe wrapper → 通用 `safeHandler`/`safeHandlerAck` |
| `app/server/internal/signal/bot_bridge.go` | handler 签名改为 `clientMessenger` |
| `app/server/internal/signal/message_bridge.go` | handler 签名改为 `clientMessenger` |
| `app/server/internal/mediasoup/signal.go` | `RegisterRoutes(*socketio.Server)` → `RegisterWS(handlerRegistrationFn)` |
| `app/server/internal/signal/hub.go` (SFUSignalHandler interface) | `RegisterRoutes(*socketio.Server)` → `RegisterWS(...)` |
| `app/server/server/gin.go` | 移除 socket.io 初始化；增加 WS fanout + upgrade route；保留 Guild DI |
| `app/server/internal/router/router.go` | 移除 socketio import；移除 `SetupSocketRoutes` |
| `app/server/go.mod` | 添加 `nhooyr.io/websocket`；移除 `googollee/go-socket.io` |
| `app/server/internal/signal/*_test.go` | 替换 `mockConn`/`mockServer` → `mockClient`/`mockFanout` |

---

---

### Task 1: 创建 ws package — types + Client

**Files:**
- Create: `app/server/internal/ws/types.go`
- Create: `app/server/internal/ws/client.go`

- [ ] **Step 1: 创建消息类型文件**

创建 `app/server/internal/ws/types.go`:

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

- [ ] **Step 2: 创建 WS Client**

创建 `app/server/internal/ws/client.go`:

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
const readLimit = 65536

// Client wraps a nhooyr WebSocket connection with goroutine-safe writes.
type Client struct {
	ID     string
	Claims *pkg.Claims
	conn   *nhooyrws.Conn
	ctx    context.Context
	cancel context.CancelFunc

	writeCh chan []byte
	closed  chan struct{}
	OnClose func(clientID string)
	mu      sync.Mutex
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

func (c *Client) writeLoop() {
	for {
		select {
		case data := <-c.writeCh:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			_ = wsjson.Write(ctx, c.conn, data)
			cancel()
		case <-c.ctx.Done():
			return
		}
	}
}

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

func (c *Client) SendACK(id, event string, data interface{}) {
	c.Send(ACK{ID: id, Event: event, Data: data})
}

func (c *Client) SendErrorACK(id, event string, code int, message string) {
	c.Send(ACK{ID: id, Event: event, Error: &ACKError{Code: code, Message: message}})
}

func (c *Client) Close() {
	c.cancel()
	_ = c.conn.Close(nhooyrws.StatusNormalClosure, "server closing")
}

func (c *Client) RemoteHeader() http.Header {
	return c.conn.HTTPResponse().Header
}
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./internal/ws/...`
Expected: 编译通过（暂时忽略未使用的 import warning）

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/ws/types.go app/server/internal/ws/client.go
git commit -m "feat(ws): add Client and Message types for WebSocket fanout"
```

---

### Task 2: 创建 ws package — Guild-aware Fanout

**Files:**
- Create: `app/server/internal/ws/fanout.go`

与 Phase 1 集成要点: Fanout room 名使用与 Hub 一致的 `roomKey(guildUUID, roomName)` 复合键。
Fanout 本身不感知 GuildUUID 语义，只管理复合键到 Client 集合的映射。

- [ ] **Step 1: 创建 Fanout**

创建 `app/server/internal/ws/fanout.go`:

```go
package ws

import (
	"sync"
)

// Fanout manages room -> clients index for efficient broadcasting.
// Room names use composite keys (see signal/roomKey) for Guild isolation.
// Thread-safe for concurrent reads/writes.
type Fanout struct {
	mu      sync.RWMutex
	rooms   map[string]map[string]*Client // roomKey -> clientID -> *Client
	clients map[string]*Client            // clientID -> *Client
}

func NewFanout() *Fanout {
	return &Fanout{
		rooms:   make(map[string]map[string]*Client),
		clients: make(map[string]*Client),
	}
}

func (f *Fanout) Add(c *Client) {
	f.mu.Lock()
	f.clients[c.ID] = c
	f.mu.Unlock()
}

// Remove unregisters a client and returns rooms it was in.
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

// BroadcastToRoom sends data to all clients in a room (by composite room key).
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

// ForEach iterates over clients in a room. Stops if fn returns false.
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

func (f *Fanout) GetClient(clientID string) *Client {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.clients[clientID]
}

func (f *Fanout) RoomCount(room string) int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.rooms[room])
}

func (f *Fanout) RoomExists(room string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.rooms[room]) > 0
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./internal/ws/...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/ws/fanout.go
git commit -m "feat(ws): add Guild-aware room-indexed Fanout"
```

---

### Task 1（更新）: 创建 ws package — 核心接口 + Client 实现

**设计原则：** ws 包按照"能力面"分层，Hub 和其他组件依赖接口而非具体类型。

```
ws/
  messenger.go   — ClientMessenger 接口（Handler 层依赖的客户端最小面）
  client.go      — Client 结构体（实现 ClientMessenger）
  broadcaster.go — Broadcaster 接口（广播/房间管理的最小面）
  fanout.go      — Fanout 结构体（实现 Broadcaster）
  handler.go     — HandlerRegistry（事件→处理函数映射 + dispatch）
  upgrader.go    — Upgrader（HTTP→WS 升级 + 连接生命周期管理）
  types.go       — Message、ACK 等 wire format 类型
```

**Files:**
- Create: `app/server/internal/ws/types.go`
- Create: `app/server/internal/ws/messenger.go`
- Create: `app/server/internal/ws/client.go`

- [ ] **Step 1: 创建 Message wire format 类型**

创建 `app/server/internal/ws/types.go`:

```go
package ws

// Message 是 websocket 的线路协议格式。
// id 用于客户端-服务端请求-应答关联，推送消息不含 id。
type Message struct {
	ID    string          `json:"id,omitempty"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// ACK 是对客户端请求的应答（request.id 非空时发送）。
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

- [ ] **Step 2: 创建 ClientMessenger 接口**

创建 `app/server/internal/ws/messenger.go`:

```go
package ws

import "GOSpeak/internal/pkg"

// ClientMessenger 是 Handler 层依赖的客户端最小面。
// 所有信号处理器只依赖此接口，不依赖具体 Client 实现。
// Hub 不需要知道底层用的是 nhooyr、gorilla 还是 mock。
type ClientMessenger interface {
	// ID 返回客户端唯一标识（通常是 UserUUID）。
	ID() string
	// Claims 返回 JWT 认证声明。
	Claims() *pkg.Claims
	// Send 发送任意 JSON 可序列化的数据。
	Send(v interface{}) bool
	// SendACK 发送带关联 id 的应答。
	SendACK(id, event string, data interface{})
	// SendErrorACK 发送带关联 id 的错误应答。
	SendErrorACK(id, event string, code int, message string)
	// Close 关闭连接。
	Close()
}
```

- [ ] **Step 3: 创建 Client 实现**

创建 `app/server/internal/ws/client.go`:

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

const (
	writeTimeout = 5 * time.Second
	readLimit    = 65536 // 64KB max message size
)

// Client 封装 nhooyr WebSocket 连接，提供 goroutine-safe 写能力。
// 实现 ClientMessenger 接口。
type Client struct {
	// 公开字段，供 Fanout/Upgrader 读取
	ID     string
	Claims *pkg.Claims

	conn   *nhooyrws.Conn
	ctx    context.Context
	cancel context.CancelFunc

	writeCh chan []byte
	closed  chan struct{}

	// OnClose 是连接关闭时的回调（由 Upgrader 设置，用于从 Fanout 注销）。
	OnClose func(clientID string)
}

// compile-time interface check
var _ ClientMessenger = (*Client)(nil)

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

// StartReadLoop 启动读取循环，阻塞直到连接关闭。
// handler 按收每条消息。应作为 goroutine 调用。
func (c *Client) StartReadLoop(handler func(ClientMessenger, Message)) {
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

func (c *Client) writeLoop() {
	for {
		select {
		case data := <-c.writeCh:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			_ = wsjson.Write(ctx, c.conn, data)
			cancel()
		case <-c.ctx.Done():
			return
		}
	}
}

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

func (c *Client) SendACK(id, event string, data interface{}) {
	c.Send(ACK{ID: id, Event: event, Data: data})
}

func (c *Client) SendErrorACK(id, event string, code int, message string) {
	c.Send(ACK{ID: id, Event: event, Error: &ACKError{Code: code, Message: message}})
}

func (c *Client) Close() {
	c.cancel()
	_ = c.conn.Close(nhooyrws.StatusNormalClosure, "server closing")
}

// RemoteHeader 返回升级请求的 HTTP 头部（用于 JWT 提取）。
func (c *Client) RemoteHeader() http.Header {
	return c.conn.HTTPResponse().Header
}
```

- [ ] **Step 4: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./internal/ws/...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/ws/types.go app/server/internal/ws/messenger.go app/server/internal/ws/client.go
git commit -m "feat(ws): add ClientMessenger interface and Client implementation"
```

---

### Task 2（更新）: 创建 ws package — Broadcaster 接口 + Fanout 实现

**Files:**
- Create: `app/server/internal/ws/broadcaster.go`
- Create: `app/server/internal/ws/fanout.go`

- [ ] **Step 1: 创建 Broadcaster 接口**

创建 `app/server/internal/ws/broadcaster.go`:

```go
package ws

// Broadcaster 是 Hub 等上层组件依赖的广播/房间管理最小面。
// 隔离 Fanout 实现细节，支持 mock 测试。
type Broadcaster interface {
	// Add 注册一个客户端到扇出（不加入任何房间）。
	Add(c *Client)
	// Remove 从扇出和所有房间中移除客户端，返回它所在的房间列表。
	Remove(clientID string) []string
	// Join 将客户端加入指定房间（房间名使用复合键 roomKey）。
	Join(room, clientID string)
	// Leave 将客户端从指定房间移除。
	Leave(room, clientID string)
	// BroadcastToRoom 向房间内所有客户端广播。
	BroadcastToRoom(room, event string, data interface{})
	// BroadcastToNamespace 向所有连接客户端广播。
	BroadcastToNamespace(event string, data interface{})
	// ForEach 遍历房间内客户端，fn 返回 false 时停止。
	ForEach(room string, fn func(ClientMessenger) bool)
	// RoomExists 检查房间是否存在（有客户端连接）。
	RoomExists(room string) bool
	// GetClient 通过 ID 查找客户端。
	GetClient(clientID string) ClientMessenger
}
```

- [ ] **Step 2: 创建 Fanout 实现**

创建 `app/server/internal/ws/fanout.go`:

```go
package ws

import "sync"

// Fanout 实现 Broadcaster 接口。
// 房间名使用复合键（由 signal.roomKey 生成）以支持 Guild 命名空间隔离。
// 线程安全，读写分离（广播时只加 RLock）。
type Fanout struct {
	mu      sync.RWMutex
	rooms   map[string]map[string]*Client // roomKey -> clientID -> *Client
	clients map[string]*Client            // clientID -> *Client
}

// compile-time interface check
var _ Broadcaster = (*Fanout)(nil)

func NewFanout() *Fanout {
	return &Fanout{
		rooms:   make(map[string]map[string]*Client),
		clients: make(map[string]*Client),
	}
}

func (f *Fanout) Add(c *Client) {
	f.mu.Lock()
	f.clients[c.ID] = c
	f.mu.Unlock()
}

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

func (f *Fanout) BroadcastToRoom(room, event string, data interface{}) {
	payload := map[string]interface{}{"event": event, "data": data}
	f.mu.RLock()
	members := f.rooms[room]
	f.mu.RUnlock()
	for _, c := range members {
		c.Send(payload)
	}
}

func (f *Fanout) BroadcastToNamespace(event string, data interface{}) {
	payload := map[string]interface{}{"event": event, "data": data}
	f.mu.RLock()
	all := make([]*Client, 0, len(f.clients))
	for _, c := range f.clients {
		all = append(all, c)
	}
	f.mu.RUnlock()
	for _, c := range all {
		c.Send(payload)
	}
}

func (f *Fanout) ForEach(room string, fn func(ClientMessenger) bool) {
	f.mu.RLock()
	members := f.rooms[room]
	f.mu.RUnlock()
	for _, c := range members {
		if !fn(c) {
			return
		}
	}
}

func (f *Fanout) RoomExists(room string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.rooms[room]) > 0
}

func (f *Fanout) GetClient(clientID string) ClientMessenger {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.clients[clientID]
}
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./internal/ws/...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/ws/broadcaster.go app/server/internal/ws/fanout.go
git commit -m "feat(ws): add Broadcaster interface and Fanout implementation"
```

---

### Task 3（新增）: 创建 ws package — HandlerRegistry 事件分发

**Files:**
- Create: `app/server/internal/ws/handler.go`

- [ ] **Step 1: 创建 HandlerRegistry**

创建 `app/server/internal/ws/handler.go`:

```go
package ws

import "log"

// handlerEntry 注册一个事件的处理函数。
// NoAck 用于推送类事件（无需应答），Ack 用于请求-应答类事件。
type handlerEntry struct {
	NoAck func(ClientMessenger, string)
	Ack   func(ClientMessenger, string) (string, error)
}

// HandlerRegistry 管理事件名到处理函数的映射，提供统一分发入口。
// Hub 通过此注册表注册所有信号事件处理函数。
type HandlerRegistry struct {
	handlers map[string]handlerEntry
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]handlerEntry),
	}
}

// Handle 注册一个无应答处理函数（fire-and-forget）。
func (r *HandlerRegistry) Handle(event string, fn func(ClientMessenger, string)) {
	r.handlers[event] = handlerEntry{NoAck: fn}
}

// HandleAck 注册一个应答处理函数。
func (r *HandlerRegistry) HandleAck(event string, fn func(ClientMessenger, string) (string, error)) {
	r.handlers[event] = handlerEntry{Ack: fn}
}

// Dispatch 分发消息到对应的处理函数，自动处理 panic recover 和 ACK 应答。
// 由 Upgrader 的读取循环在收到每条消息时调用。
func (r *HandlerRegistry) Dispatch(c ClientMessenger, msg Message) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[ws] panic in handler event=%s client=%s: %v", msg.Event, c.ID(), rec)
			if msg.ID != "" {
				c.SendErrorACK(msg.ID, msg.Event, 5001, "internal server error")
			}
		}
	}()

	entry, ok := r.handlers[msg.Event]
	if !ok {
		log.Printf("[ws] unknown event: %s from client=%s", msg.Event, c.ID())
		return
	}

	dataStr := string(msg.Data)
	if dataStr == "null" || dataStr == `""` {
		dataStr = ""
	}

	if entry.Ack != nil {
		result, err := entry.Ack(c, dataStr)
		if msg.ID != "" {
			if err != nil {
				c.SendErrorACK(msg.ID, msg.Event, 5001, err.Error())
			} else {
				c.SendACK(msg.ID, msg.Event, result)
			}
		}
	} else if entry.NoAck != nil {
		entry.NoAck(c, dataStr)
	}
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./internal/ws/...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/ws/handler.go
git commit -m "feat(ws): add HandlerRegistry for typed event dispatch"
```

---

### Task 4（新增）: 创建 ws package — Upgrader 连接生命周期管理

**Files:**
- Create: `app/server/internal/ws/upgrader.go`

Upgrader 封装了完整的 WS 连接生命周期:
1. JWT 鉴权（复用 middleware.VerifyToken）
2. HTTP → WebSocket 升级
3. 注册 Client 到 Fanout
4. 启动读取循环（自动 dispatch 到 HandlerRegistry）
5. 断连时自动从 Fanout 注销

- [ ] **Step 1: 创建 Upgrader**

创建 `app/server/internal/ws/upgrader.go`:

```go
package ws

import (
	"log"
	"net/http"
	"strings"

	nhooyrws "nhooyr.io/websocket"

	"GOSpeak/internal/middleware"
	"GOSpeak/internal/pkg"
)

// UpgraderConfig 控制 WebSocket 升级行为。
type UpgraderConfig struct {
	// Fanout 用于注册/注销客户端连接。
	Fanout Broadcaster
	// Handler 是事件分发注册表。
	Handler *HandlerRegistry
	// OnConnect 在连接建立后、读取循环开始前调用（Hub 用于设置 OnClose）。
	OnConnect func(c *Client)
}

// Upgrader 封装 HTTP→WS 升级、鉴权、生命周期管理。
type Upgrader struct {
	cfg UpgraderConfig
}

func NewUpgrader(cfg UpgraderConfig) *Upgrader {
	return &Upgrader{cfg: cfg}
}

// extractToken 从请求中提取 JWT token：Authorization header > cookie > query。
func extractToken(r *http.Request) string {
	tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tokenStr != "" && tokenStr != r.Header.Get("Authorization") {
		return tokenStr
	}
	if cookie, err := r.Cookie("gospeak_token"); err == nil {
		return cookie.Value
	}
	return r.URL.Query().Get("token")
}

// ServeHTTP 实现 http.Handler，一次完成升级→鉴权→注册→读取循环。
// 应该在 Gin 路由中通过 `r.GET("/ws", gin.WrapH(upgrader))` 挂载。
func (u *Upgrader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractToken(r)
	if tokenStr == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	claims, code := middleware.VerifyToken(tokenStr)
	if code != pkg.SUCCESS {
		log.Printf("[ws] upgrade rejected: token=%s client=%s", code.String(), r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := nhooyrws.Accept(w, r, &nhooyrws.AcceptOptions{
		InsecureSkipVerify: true, // Origin check 由 Gin CORS 中间件处理
	})
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	clientID := claims.UserUUID
	if clientID == "" {
		clientID = claims.Username
	}
	client := NewClient(conn, clientID, claims)

	// 注册到 Fanout
	if u.cfg.Fanout != nil {
		u.cfg.Fanout.Add(client)
	}

	// 生命周期回调（Hub 设置 OnClose 以从 Fanout 注销 + 清理 Hub 状态）
	client.OnClose = func(id string) {
		if u.cfg.Fanout != nil {
			u.cfg.Fanout.Remove(id)
		}
	}

	if u.cfg.OnConnect != nil {
		u.cfg.OnConnect(client)
	}

	log.Printf("[ws] client connected: %s (%s) ip=%s", clientID, claims.Username, r.RemoteAddr)

	// 阻塞读取循环
	if u.cfg.Handler != nil {
		client.StartReadLoop(func(c ClientMessenger, msg Message) {
			u.cfg.Handler.Dispatch(c, msg)
		})
	} else {
		client.StartReadLoop(func(c ClientMessenger, msg Message) {})
	}

	log.Printf("[ws] client disconnected: %s (%s)", clientID, claims.Username)
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./internal/ws/...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/ws/upgrader.go
git commit -m "feat(ws): add Upgrader with JWT auth and lifecycle management"
```

---

### Task 5（原 Task 4）: 创建 WSDeliverer + 添加 nhooyr 依赖

**Files:**
- Create: `app/server/internal/bus/ws_deliverer.go`
- Delete: `app/server/internal/bus/sio_deliverer.go`
- Modify: `app/server/go.mod`

- [ ] **Step 1: 创建 WSDeliverer**

创建 `app/server/internal/bus/ws_deliverer.go`:

```go
package bus

import "GOSpeak/internal/ws"

// WSDeliverer 实现 event bus 的 local deliverer，将事件投递到本地 WS Fanout。
type WSDeliverer struct {
	fanout ws.Broadcaster
}

func NewWSDeliverer(f ws.Broadcaster) *WSDeliverer {
	return &WSDeliverer{fanout: f}
}

func (d *WSDeliverer) BroadcastToNamespace(event string, data interface{}) {
	if d == nil || d.fanout == nil {
		return
	}
	d.fanout.BroadcastToNamespace(event, data)
}

func (d *WSDeliverer) BroadcastToRoom(room, event string, data interface{}) {
	if d == nil || d.fanout == nil {
		return
	}
	d.fanout.BroadcastToRoom(room, event, data)
}
```

- [ ] **Step 2: 添加 nhooyr 依赖并清理旧依赖**

```bash
cd /Users/noelorin/GOSpeak/app/server
go get nhooyr.io/websocket@v1.8.17
```

注：暂不删除 go-socket.io 依赖（Task 9 gin.go 重构时再一并清理），此处先完成新依赖添加。

- [ ] **Step 3: 删除旧的 sio_deliverer.go**

```bash
rm app/server/internal/bus/sio_deliverer.go
```

- [ ] **Step 4: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过（可能因为 sio_deliverer 被删导致某个文件引用报错，视情况补充修改）

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/bus/ws_deliverer.go app/server/go.mod app/server/go.sum
git rm app/server/internal/bus/sio_deliverer.go
git commit -m "feat(ws): add WSDeliverer and nhooyr websocket dependency"
```

---

### Task 6（原 Task 5）: Refactor Hub — 替换 socketServer → Broadcaster + HandlerRegistry

**这是全方案最大改动。** Hub 从 socket.io 驱动迁移到 ws.Broadcaster 驱动。

**Files:**
- Modify: `app/server/internal/signal/hub.go`

**改动清单（影响范围）：**

| 原 | 新 |
|----|----|
| `server socketServer` | `fanout ws.Broadcaster` |
| `import socketio` | 移除 |
| `socketio.Conn` 参数 | `ws.ClientMessenger` 参数 |
| `s.Emit(...)` | `c.Send(...)` 或 `c.SendACK(...)` |
| `s.Join(room)` | `h.fanout.Join(room, c.ID())` |
| `s.Leave(room)` | `h.fanout.Leave(room, c.ID())` |
| `s.ID()` | `c.ID()` |
| `s.Context()` / `s.SetContext()` | `c.Claims()` |
| `h.server.BroadcastToRoom(...)` | `h.fanout.BroadcastToRoom(...)` |
| `h.server.ForEach(...)` | `h.fanout.ForEach(...)` |
| `claimsIdentity(s)` | `clientIdentity(c)` |
| `SetServer(socketio.Server)` | `SetFanout(ws.Broadcaster)` |
| `SetupRoutes(socketio.Server)` | `SetupFanout(ws.Broadcaster, *ws.HandlerRegistry)` |
| `SFUSignalHandler.RegisterRoutes(*socketio.Server)` | `RegisterWS(reg ws.HandlerRegistrar)` |

- [ ] **Step 1: 替换 Hub struct 中的 socketServer → Broadcaster**

```go
// 移除:
type socketServer interface { ... }
server socketServer

// 添加:
fanout ws.Broadcaster

// 移除:
import socketio "github.com/googollee/go-socket.io"
```

- [ ] **Step 2: 添加 clientMessenger 别名（可选，完全兼容 ws.ClientMessenger）**

```go
// 为了减少代码改动，Hub 直接使用 ws.ClientMessenger
// 不需要额外定义。所有 handler 签名改为：
//
// func (h *Hub) OnRoomCreate(c ws.ClientMessenger, data string)
//
// 如果希望保持 Hub 零 ws 依赖，可以定义别名：
// type clientMessenger = ws.ClientMessenger
// 这样 handler 签名中的 ws.ClientMessenger 可写作 clientMessenger。
```

这里选择直接使用 `ws.ClientMessenger`。

- [ ] **Step 3: 替换 SetupRoutes → SetupFanout**

```go
// Before:
func (h *Hub) SetupRoutes(server *socketio.Server) {
	h.SetServer(server)
	server.OnConnect("/", safeOnConnect(h.OnConnect))
	server.OnEvent("/", EventRoomCreate, safeOnEventData(h.OnRoomCreate))
	// ... 14 个事件注册
}

// After:
func (h *Hub) SetupFanout(fanout ws.Broadcaster, reg *ws.HandlerRegistry) {
	h.fanout = fanout

	// 注册所有事件处理器到 HandlerRegistry
	reg.Handle(EventRoomCreate, safeHandler(h.OnRoomCreate))
	reg.HandleAck(EventRoomJoin, safeHandlerAck(h.OnRoomJoin))
	reg.HandleAck(EventRoomJoinSFU, safeHandlerAck(h.OnRoomJoinSFU))
	reg.HandleAck(EventRoomLeave, safeHandlerAck(h.OnRoomLeave))
	reg.Handle(EventRoomList, h.OnRoomList)
	reg.Handle(EventRoomKick, safeHandler(h.OnRoomKick))
	reg.Handle(EventMemberMicState, safeHandler(h.OnMemberMicState))
	reg.Handle(EventMemberSpeaking, safeHandler(h.OnMemberSpeaking))
	reg.Handle(EventBotCommand, safeHandler(h.PublishBotCommand))
	reg.Handle(EventBotMessage, safeHandler(h.PublishBotMessage))
	reg.HandleAck(EventMessageSend, safeHandlerAck(h.OnMessageSend))

	if h.sfuSignalHandler != nil {
		h.sfuSignalHandler.RegisterWS(func(event string, fn func(ws.ClientMessenger, string) (string, error)) {
			reg.HandleAck(event, safeHandlerAck(fn))
		})
	}
}
```

- [ ] **Step 4: 替换 SetServer → SetFanout**

```go
// Before:
func (h *Hub) SetServer(server *socketio.Server) {
	h.server = server
}

// After:
func (h *Hub) SetFanout(fanout ws.Broadcaster) {
	h.fanout = fanout
}
```

- [ ] **Step 5: 移除 OnConnect 方法（JWT 鉴权移至 Upgrader）**

删除整个 `OnConnect` 方法。Upgrader 在 HTTP upgrade 时已完成 JWT 鉴权，
noop 函数由 `Client.StartReadLoop` 的 handler 参数替代。

- [ ] **Step 6: 重写 OnDisconnect → OnClientDisconnect**

输入参数从 `socketio.Conn, string` 变为 `clientID string`（Upgrader 的 OnClose 回调触发）。

```go
// Before:
func (h *Hub) OnDisconnect(s socketio.Conn, reason string) {
	type disconnectCleanup struct { ... }
	type leaveEvent struct { ... }
	h.mu.Lock()
	for roomName, room := range h.rooms {
		if member, ok := room.Members[s.ID()]; ok {
			// cleanup per socket ID
		}
	}
	h.mu.Unlock()
	// publish leave events...
}

// After:
func (h *Hub) OnClientDisconnect(clientID string) {
	type disconnectCleanup struct { ... }
	h.mu.Lock()
	for roomName, room := range h.rooms {
		if member, ok := room.ByIdentity[clientID]; ok {
			// cleanup per identity
			roomDelMember(room, member.Identity) // use identity, not socketID
			delete(room.Speaking, member.Identity)
			delete(room.MicMuted, member.Identity)
			// ... rest same
		}
	}
	h.mu.Unlock()
	// same broadcast/cleanup logic as before
}
```

**关键变更：** Phase 1 后 Hub 的 rooms 用 `roomKey(guildUUID, roomName)` 做 key，
OnClientDisconnect 遍历 `h.rooms` 时对所有 room key 做全量扫描，查找匹配 identity。
由于 `clientID = UserUUID = claims.UserUUID`，匹配逻辑改为按 ByIdentity 查找。

- [ ] **Step 7: 替换所有 handler 签名**

**需要修改签名的方法列表（共 11 个）：**

| 方法 | 旧签名 | 新签名 |
|------|--------|--------|
| OnRoomCreate | `(s socketio.Conn, data string)` | `(c ws.ClientMessenger, data string)` |
| OnRoomJoin | `(s socketio.Conn, data string) (string, error)` | `(c ws.ClientMessenger, data string) (string, error)` |
| OnRoomJoinSFU | `(s socketio.Conn, data string) (string, error)` | `(c ws.ClientMessenger, data string) (string, error)` |
| OnRoomLeave | `(s socketio.Conn, data string) (string, error)` | `(c ws.ClientMessenger, data string) (string, error)` |
| OnRoomList | `(s socketio.Conn)` | `(c ws.ClientMessenger, data string)` |
| OnRoomKick | `(s socketio.Conn, data string)` | `(c ws.ClientMessenger, data string)` |
| OnMemberMicState | `(s socketio.Conn, data string)` | `(c ws.ClientMessenger, data string)` |
| OnMemberSpeaking | `(s socketio.Conn, data string)` | `(c ws.ClientMessenger, data string)` |
| PublishBotCommand | `(s socketio.Conn, data string)` | `(c ws.ClientMessenger, data string)` |
| PublishBotMessage | `(s socketio.Conn, data string)` | `(c ws.ClientMessenger, data string)` |
| OnMessageSend | `(s socketio.Conn, data string) (string, error)` | `(c ws.ClientMessenger, data string) (string, error)` |

- [ ] **Step 8: 替换方法体内 socket.io API 调用**

```go
// s.ID() → c.ID()
// s.Emit(EventFoo, data) → c.Send(...)
// s.Join(req.Room) → h.fanout.Join(roomKey(req.GuildUUID, req.Room), c.ID())
// s.Leave(req.Room) → h.fanout.Leave(roomKey(req.GuildUUID, req.Room), c.ID())
// h.server.BroadcastToRoom("/", room, event, data) → h.fanout.BroadcastToRoom(room, event, data)
// h.server.ForEach("/", room, fn) → h.fanout.ForEach(room, fn)
```

**特别注意 OnRoomJoinSFU 和 OnRoomJoin 中成员写入流程：**
现在 fanout.Join/Leave 替代了 socket.io 的 s.Join/s.Leave，Hub 的 `h.rooms` map
仍然维护房间成员元数据（MemberInfo），而 fanout 只维护 room→clientID 的连接索引。
两者分工：
- `h.fanout.Join(roomKey, clientID)` — 连接层注册（广播可达）
- `h.rooms[roomKey].Members[clientID]` — 元数据层（成员信息、禁言状态等）

- [ ] **Step 9: 替换 claimsIdentity → clientIdentity**

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
func clientIdentity(c ws.ClientMessenger) string {
	if c == nil || c.Claims() == nil {
		return ""
	}
	return c.Claims().Username
}
```

- [ ] **Step 10: 替换 resolveIdentity**

```go
// Before:
func resolveIdentity(s socketio.Conn, requested string) (string, error) {
	identity := claimsIdentity(s)
	if identity == "" {
		return "", fmt.Errorf("unauthorized")
	}
	if requested != "" && requested != identity {
		return "", fmt.Errorf("identity mismatch")
	}
	return identity, nil
}

// After:
func resolveIdentity(c ws.ClientMessenger, requested string) (string, error) {
	identity := clientIdentity(c)
	if identity == "" {
		return "", fmt.Errorf("unauthorized")
	}
	if requested != "" && requested != identity {
		return "", fmt.Errorf("identity mismatch")
	}
	return identity, nil
}
```

- [ ] **Step 11: 更新广播方法（publishRoom / publishNamespace / BroadcastToRoom）**

```go
// publishRoom 使用 h.fanout.BroadcastToRoom 替代 h.server.BroadcastToRoom
func (h *Hub) publishRoom(room, event string, data interface{}) {
	if h.fanout != nil {
		h.fanout.BroadcastToRoom(room, event, data)
	}
}

// publishNamespace 使用 h.fanout.BroadcastToNamespace 替代 h.server.BroadcastToNamespace
func (h *Hub) publishNamespace(event string, data interface{}) {
	if h.fanout != nil {
		h.fanout.BroadcastToNamespace(event, data)
	}
}

// BroadcastToRoom 公开方法供第三方调用
func (h *Hub) BroadcastToRoom(room, event string, data interface{}) {
	h.publishRoom(room, event, data)
}
```

- [ ] **Step 12: 更新 ForEach 用法（kick handler 中查找特定 socket）**

```go
// Before (kick handler):
h.server.ForEach("/", req.Room, func(conn socketio.Conn) {
	if conn != nil && conn.ID() == targetSocketID {
		targetConn = conn
	}
})

// After:
h.fanout.ForEach(req.Room, func(c ws.ClientMessenger) bool {
	if c.ID() == targetClientID {
		c.Send(map[string]interface{}{
			"event": EventRoomKicked,
			"data":  payload,
		})
		return false
	}
	return true
})
```

- [ ] **Step 13: 更新 OnError → 移除（panic recover 由 HandlerRegistry 处理）**

删除 `OnError` 方法。

- [ ] **Step 14: 更新 SFUSignalHandler 接口定义**

```go
// Before:
type SFUSignalHandler interface {
	RegisterRoutes(server *socketio.Server)
}

// After:
type SFUSignalHandler interface {
	RegisterWS(reg func(event string, fn func(ws.ClientMessenger, string) (string, error)))
}
```

- [ ] **Step 15: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过（可能有未使用的 import，`go mod tidy` 后清除）

- [ ] **Step 16: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/signal/hub.go
git commit -m "refactor(signal): replace socket.io with ws.Broadcaster and ClientMessenger"
```

---

### Task 7（原 Task 6-7）: 更新 bot_bridge + message_bridge + recover

**Files:**
- Modify: `app/server/internal/signal/bot_bridge.go`
- Modify: `app/server/internal/signal/message_bridge.go`
- Modify: `app/server/internal/signal/recover.go`

- [ ] **Step 1: 更新 bot_bridge.go**

删除 socket.io import，修改所有 handler 签名:

```go
// Before:
func (h *Hub) PublishBotCommand(s socketio.Conn, data string)
func (h *Hub) PublishBotMessage(s socketio.Conn, data string)

// After:
func (h *Hub) PublishBotCommand(c ws.ClientMessenger, data string)
func (h *Hub) PublishBotMessage(c ws.ClientMessenger, data string)
```

- [ ] **Step 2: 更新 message_bridge.go**

```go
// Before:
func (h *Hub) OnMessageSend(s socketio.Conn, data string) (string, error)
// 方法内: s.Context() → pkg.Claims

// After:
func (h *Hub) OnMessageSend(c ws.ClientMessenger, data string) (string, error)
// 方法内: c.Claims() → *pkg.Claims
```

- [ ] **Step 3: 重写 recover.go**

删掉所有 socket.io 专用 wrapper（`safeOnConnect`、`safeOnDisconnect`、`safeOnError`、`safeOnEventData`、`safeOnEventDataAck`、`safeOnEventNoData`），保留通用函数:

```go
package signal

import "log"

// safeHandler wraps a NoAck handler with panic recovery.
func safeHandler(fn func(ws.ClientMessenger, string)) func(ws.ClientMessenger, string) {
	return func(c ws.ClientMessenger, data string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[signal] handler panic recovered: %v", r)
			}
		}()
		fn(c, data)
	}
}

// safeHandlerAck wraps an Ack handler with panic recovery.
func safeHandlerAck(fn func(ws.ClientMessenger, string) (string, error)) func(ws.ClientMessenger, string) (string, error) {
	return func(c ws.ClientMessenger, data string) (string, error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[signal] ack handler panic: %v", r)
			}
		}()
		return fn(c, data)
	}
}
```

注：实际 panic recover 由 ws.HandlerRegistry.Dispatch 统一兜底，此处为双层防护。

- [ ] **Step 4: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./internal/signal/...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/signal/bot_bridge.go app/server/internal/signal/message_bridge.go app/server/internal/signal/recover.go
git commit -m "refactor(signal): update bot_bridge, message_bridge, recover for ws.ClientMessenger"
```

---

### Task 8（原 Task 8）: 更新 MediasoupSignal — RegisterWS

**Files:**
- Modify: `app/server/internal/mediasoup/signal.go`

- [ ] **Step 1: 替换 RegisterRoutes → RegisterWS**

```go
// Before:
func (m *MediasoupSignal) RegisterRoutes(server *socketio.Server) {
	server.OnEvent("/", "sfu:get-router-capabilities", ...)
	server.OnEvent("/", "sfu:create-transport", ...)
	// ...
}

// After:
func (m *MediasoupSignal) RegisterWS(register func(event string, fn func(ws.ClientMessenger, string) (string, error))) {
	register(EventSFUGetRouterCapabilities, m.handleGetRouterCapabilities)
	register(EventSFUCreateTransport, m.handleCreateTransport)
	register(EventSFUConnectTransport, m.handleConnectTransport)
	register(EventSFUProduce, m.handleProduce)
	register(EventSFUConsume, m.handleConsume)
	// ...
}
```

其中 handler 签名从 `(socketio.Conn, string) (string, error)` 改为 `(ws.ClientMessenger, string) (string, error)`。
MediasoupSignal 内部使用 `c.Send()` 替代 socket.io 的 emit，但 method body 逻辑保持不变。

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/mediasoup/signal.go
git commit -m "refactor(mediasoup): replace RegisterRoutes with RegisterWS"
```

---

### Task 9（原 Task 9）: 更新 gin.go — 移除 socket.io，接入 WS fanout

**Files:**
- Modify: `app/server/server/gin.go`

**核心变更：**
1. 移除所有 socket.io 导入和初始化代码
2. 创建 ws.Fanout + ws.HandlerRegistry + ws.Upgrader
3. Hub.SetupFanout 替代 Hub.SetupRoutes
4. WS upgrade 路由替代 socket.io 路由
5. 保留所有 Phase 1 新增的 Guild DI

- [ ] **Step 1: 删除 socket.io 相关导入**

```go
// 删除以下导入:
socketio "github.com/googollee/go-socket.io"
engineio "github.com/googollee/go-socket.io/engineio"
"github.com/googollee/go-socket.io/engineio/transport"
"github.com/googollee/go-socket.io/engineio/transport/polling"
"github.com/googollee/go-socket.io/engineio/transport/websocket"

// 添加:
"GOSpeak/internal/ws"
```

- [ ] **Step 2: 删除 socket.io 初始化代码**

```go
// 删除:
// wsTransport := websocket.Default
// wsTransport.CheckOrigin = makeCheckOrigin(cfg.CORSOrigin)
// sioServer := socketio.NewServer(&engineio.Options{...})
// sioServer.OnConnect("/", ...)
// 以及 go sioServer.Serve()
// 以及 r.GET("/socket.io/*any", ...) r.POST(...) r.OPTIONS(...)
```

- [ ] **Step 3: 创建 WS 基础设施（放在 guildSvc/guildHandler 之后）**

```go
// 创建 ws.Fanout + HandlerRegistry + Upgrader
wsFanout := ws.NewFanout()
wsHandler := ws.NewHandlerRegistry()

// 初始化 signalHub 并注册到 WS
signalHub.SetFanout(wsFanout)
signalHub.SetupFanout(wsFanout, wsHandler)

// 保留 Guild Handler 需要的 OnClientDisconnect
signalHub.OnClientDisconnect = func(clientID string) {
	wsFanout.Remove(clientID)
	// Hub 内部的清理在 OnClientDisconnect 方法中处理
}

// WSDeliverer 替换 SIODeliverer
deliverer := bus.NewWSDeliverer(wsFanout)
// deliverer 变量替换旧代码中的 sioDeliverer

// 创建 Upgrader
upgrader := ws.NewUpgrader(ws.UpgraderConfig{
	Fanout:    wsFanout,
	Handler:   wsHandler,
	OnConnect: func(c *ws.Client) {
		// 连接建立后的 Hook（可选）
	},
})

// WS 路由
r.GET("/ws", cors, gin.WrapH(upgrader))
```

- [ ] **Step 4: 更新 Deliverer 和 EventBus 创建**

原代码中 `deliverer := bus.NewConcurrentDeliverer(sioServer, 0)` 替换为:

```go
deliverer := bus.NewWSDeliverer(wsFanout)
```

然后 EventBus 创建继续使用此 deliverer。

- [ ] **Step 5: 移除 socket.io 服务 goroutine 和路由**

```go
// 删除:
// go func() {
//    if err := sioServer.Serve(); err != nil {
//        log.Printf("socket.io serve error: %v", err)
//    }
// }()
// defer sioServer.Close()

// 删除:
// r.GET("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))
// r.POST("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))
// r.OPTIONS("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))
```

- [ ] **Step 6: 移除 makeCheckOrigin 函数（不再需要）**

```go
// 删除:
// func makeCheckOrigin(corsOrigin string) func(r *http.Request) bool { ... }
```

- [ ] **Step 7: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过，无 socket.io 引用残留

- [ ] **Step 8: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/server/gin.go
git commit -m "refactor(server): replace socket.io with ws.Fanout and Upgrader"
```

---

### Task 10（原 Task 10）: 更新 router.go — 移除 socket.io 引用

**Files:**
- Modify: `app/server/internal/router/router.go`

- [ ] **Step 1: 删除 socketio import 和 SetupSocketRoutes**

```go
// 删除 import:
socketio "github.com/googollee/go-socket.io"

// 删除:
func SetupSocketRoutes(server *socketio.Server, signalHub interface{...}) {
	signalHub.SetupRoutes(server)
}
```

- [ ] **Step 2: 更新 NoRoute path 前缀检查**

```go
// Before:
if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/socket.io") || ...

// After:
if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") || ...
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/router/router.go
git commit -m "refactor(router): remove socket.io references"
```

---

### Task 11（原 Task 11）: go.mod 清理 + patch 删除

**Files:**
- Modify: `app/server/go.mod`
- Delete: `app/server/patch/gorilla-websocket/`

- [ ] **Step 1: 运行 go mod tidy**

```bash
cd /Users/noelorin/GOSpeak/app/server
go mod tidy
```

Expected: go.mod 中自动移除 `github.com/googollee/go-socket.io` 及其所有传递依赖（gorilla/websocket 等）。

- [ ] **Step 2: 验证 no socket.io references**

```bash
cd /Users/noelorin/GOSpeak/app/server
rg "go-socket.io" go.mod
# Expected: empty (no matches)
rg "gorilla/websocket" go.mod
# Expected: empty
rg "replace github.com/gorilla" go.mod
# Expected: empty
```

- [ ] **Step 3: 删除 patch 目录**

```bash
rm -rf app/server/patch/gorilla-websocket/
```

- [ ] **Step 4: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/go.mod app/server/go.sum
git rm -r app/server/patch/gorilla-websocket/
git commit -m "chore: remove socket.io dependency and gorilla websocket patch"
```

---

### Task 12（原 Task 12）: 更新测试

**Files:**
- Modify: `app/server/internal/signal/hub_test.go`
- Modify: `app/server/internal/signal/hub_kick_test.go`
- Modify: `app/server/internal/signal/hub_integration_test.go`
- Modify: `app/server/internal/signal/hub_event_bus_test.go`
- Modify: `app/server/internal/signal/bot_bridge_test.go`
- Modify: `app/server/internal/signal/message_bridge_test.go`
- + 其他 signal 测试文件

- [ ] **Step 1: 替换测试 Mock**

创建 `mockClientMessenger` 替代 `mockConn`。所有测试文件统一使用此 mock。

```go
// 在 hub_test.go 中添加:
type mockClientMessenger struct {
	id      string
	claims  *pkg.Claims
	emitted []interface{}
	mu      sync.Mutex
}

func newMockClient(id string) *mockClientMessenger {
	return &mockClientMessenger{id: id, claims: &pkg.Claims{Username: "user-" + id, UserUUID: id, Role: "user"}}
}

func (m *mockClientMessenger) ID() string { return m.id }

func (m *mockClientMessenger) Claims() *pkg.Claims { return m.claims }

func (m *mockClientMessenger) Send(v interface{}) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitted = append(m.emitted, v)
	return true
}

func (m *mockClientMessenger) SendACK(id, event string, data interface{}) {
	m.Send(map[string]interface{}{"id": id, "event": event, "data": data})
}

func (m *mockClientMessenger) SendErrorACK(id, event string, code int, message string) {
	m.Send(map[string]interface{}{"id": id, "event": event, "error": map[string]interface{}{"code": code, "message": message}})
}

func (m *mockClientMessenger) Close() {}

func (m *mockClientMessenger) lastEvent(event string) interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.emitted) - 1; i >= 0; i-- {
		if v, ok := m.emitted[i].(map[string]interface{}); ok && v["event"] == event {
			return v["data"]
		}
	}
	return nil
}
```

```go
// mockBroadcaster 替代 mockServer
type mockBroadcaster struct {
	broadcasts map[string][]interface{}
	roomCasts  map[string]map[string][]interface{}
}

func newMockBroadcaster() *mockBroadcaster {
	return &mockBroadcaster{
		broadcasts: make(map[string][]interface{}),
		roomCasts:  make(map[string]map[string][]interface{}),
	}
}

func (m *mockBroadcaster) Add(c *ws.Client) {}
func (m *mockBroadcaster) Remove(clientID string) []string { return nil }
func (m *mockBroadcaster) Join(room, clientID string) {}
func (m *mockBroadcaster) Leave(room, clientID string) {}

func (m *mockBroadcaster) BroadcastToNamespace(event string, data interface{}) {
	m.broadcasts[event] = append(m.broadcasts[event], data)
}

func (m *mockBroadcaster) BroadcastToRoom(room, event string, data interface{}) {
	if m.roomCasts[room] == nil {
		m.roomCasts[room] = make(map[string][]interface{})
	}
	m.roomCasts[room][event] = append(m.roomCasts[room][event], data)
}

func (m *mockBroadcaster) ForEach(room string, fn func(ws.ClientMessenger) bool) {}
func (m *mockBroadcaster) RoomExists(room string) bool { return false }
func (m *mockBroadcaster) GetClient(clientID string) ws.ClientMessenger { return nil }
```

- [ ] **Step 2: 更新测试用例 — 替换 mockConn→mockClientMessenger, mockServer→mockBroadcaster**

```go
// Before:
conn := newMockConn("sock1")
server := newMockServer()
hub.SetServer(server)
// 调用 hub.OnRoomCreate(conn, data)

// After:
client := newMockClient("user-uuid-1")
fanout := newMockBroadcaster()
hub.SetFanout(fanout)
// 调用 hub.OnRoomCreate(client, data)

// 断言:
if client.lastEvent(EventRoomCreated) == nil {
	t.Error("expected room:created event")
}
```

- [ ] **Step 3: 逐个文件更新测试断言**

关键模式：之前测试通过 `conn.emitted[EventFoo]` 直接访问 map，现在需要通过 `client.lastEvent(event)` 方法遍历 `emitted` 切片查找。

- [ ] **Step 4: 运行全部测试**

```bash
cd /Users/noelorin/GOSpeak/app/server
go test ./internal/signal/... -v -count=1 -timeout 120s 2>&1 | tail -30
```

Expected: ALL TESTS PASSED

- [ ] **Step 5: 修复测试失败（如有）**

根据失败信息迭代修复，确保全部通过。

- [ ] **Step 6: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/signal/*_test.go
git commit -m "test(signal): migrate tests to ws.ClientMessenger and mockBroadcaster"
```

---

## Self-Review

### 1. Spec Coverage

| 需求 | Task | 状态 |
|------|------|------|
| ws 包能力分层（Messenger/Broadcaster/Handler/Upgrader） | Task 1-4 | ✅ 接口驱动，依赖倒置 |
| WSDeliverer 替换 SIODeliverer | Task 5 | ✅ |
| Hub 从 socket.io 迁移到 Broadcaster | Task 6 | ✅ 全部 handler 签名更新 |
| bot_bridge + message_bridge 适配 | Task 7 | ✅ |
| recover.go 清理 socket.io wrapper | Task 7 | ✅ 统一 safeHandler/safeHandlerAck |
| MediasoupSignal.RegisterWS | Task 8 | ✅ |
| gin.go 接入 WS + 保留 Guild DI | Task 9 | ✅ |
| router.go 清理 socket.io 路由 | Task 10 | ✅ |
| go.mod + patch 清理 | Task 11 | ✅ |
| 测试迁移 | Task 12 | ✅ mockClientMessenger + mockBroadcaster |

**未覆盖的部分（方案外）：**
- 前端 WS 客户端（前端从 socket.io client 迁移到原生 WebSocket）— 这是单独的前端任务
- socket.io 遗留的 WebTransport/polling 回退处理（不再需要）

### 2. 与 Phase 1 集成验证

| 集成点 | 一致性 |
|--------|--------|
| `roomKey(guildUUID, roomName)` 复合键 | Fanout 接收的 room 参数已是复合键，与 Hub 一致 |
| `RoomRequest.GuildUUID` 字段 | 保留，handler 通过 `c.Claims().UserUUID` 获取身份 |
| gin.go Guild DI | 保留不变，WS 部分在新代码段中独立添加 |
| router.go 路由 | Guild 路由组保留，仅移除 socket.io 路由 |

### 3. Type/Name Consistency

- `ws.ClientMessenger` — Hub 所有 handler 统一使用
- `ws.Broadcaster` — Hub.fanout 字段类型
- `ws.HandlerRegistry` — Hub.SetupFanout 参数
- `ws.Upgrader` — gin.go 中 HTTP handler
- `clientIdentity()` — 替换 `claimsIdentity()`，签名一致
- `resolveIdentity()` — 参数从 `socketio.Conn` 换为 `ws.ClientMessenger`，逻辑一致

### 4. 迁移注意事项

1. **测试必须先行**: Task 12 在 gin.go 变更前完成 mock 替换，确保 Task 9 可以端到端验证
2. **分步提交**: 每个 Task 都可独立编译 + 测试通过，降低回滚风险
3. **前端兼容**: 本方案只改后端 WS 传输层，前端 socket.io client 替换为原生 WebSocket 是单独的前端 task，两阶段之间服务端/前端可以共存（前端用 socket.io client 连接旧端点，后端内部已无 socket.io）

---

## 执行方式

方案已保存至 `docs/superpowers/plans/2026-07-29-websocket-migration-phase2.md`

### 执行建议

**Phase 2 与 Phase 1 的执行顺序：** 建议 Phase 1（Guild）先全部完成后，再启动 Phase 2（WS 迁移）。原因是 Hub 的 `roomKey` 改动在 Phase 1 中做，Phase 2 在此基础上做 handler 签名 + 广播机制替换，变更范围清晰。

**Phase 2 内部执行顺序：**

| 批次 | Task | 说明 |
|------|------|------|
| Batch 1 | 1-5 | ws 包 + WSDeliverer（可并行编写，不依赖现有代码） |
| Batch 2 | 7-8 | bot_bridge + message_bridge + recover + mediasoup（hub 方法签名未变时可独立做） |
| Batch 3 | 6 | Hub 核心重构（依赖 Batch 1-2 完成后，一次性替换所有 handler 签名） |
| Batch 4 | 12 | 测试迁移（与 Batch 3 可并行，但 Batch 3 完成后需跑通全部测试） |
| Batch 5 | 9-11 | gin.go + router.go + go.mod 清理（最后一步，需要 Batch 3 完成后） |

### 两种执行选项:

**1. Subagent-Driven（推荐）** — Batch 1 可并行派发 5 个子 agent（Task 1-5 无依赖）
**2. Inline Execution** — 按 Batch 顺序执行，每个 Batch 编译验证一次
