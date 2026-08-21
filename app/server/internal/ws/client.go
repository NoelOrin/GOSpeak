package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	nhooyrws "nhooyr.io/websocket"

	"GOSpeak/internal/pkg"
)

// ConnState 表示服务端 WS 客户端的连接生命周期状态。
type ConnState string

const (
	ConnStateNew        ConnState = "new"
	ConnStateConnecting ConnState = "connecting"
	ConnStateOpen       ConnState = "open"
	ConnStateClosing    ConnState = "closing"
	ConnStateClosed     ConnState = "closed"
)

func (s ConnState) String() string { return string(s) }

const (
	writeTimeout     = 5 * time.Second
	sendQueueTimeout = 500 * time.Millisecond
	readLimit        = 65536 // 64KB max message size

	heartbeatInterval     = 30 * time.Second
	heartbeatWriteTimeout = 5 * time.Second
)

// Client 封装 nhooyr WebSocket 连接，提供 goroutine-safe 写能力。
// 实现 ClientMessenger 接口。
type Client struct {
	// id 是连接级唯一标识（由 Upgrader 生成），不直接复用用户 ID，
	// 避免同一用户多个连接互相覆盖 Fanout/Hub 中的成员状态。
	id string
	// claims 是 JWT 认证声明，通过 Claims() 访问。
	claims *pkg.Claims

	conn   *nhooyrws.Conn
	ctx    context.Context
	cancel context.CancelFunc

	writeCh chan []byte
	closed  chan struct{}
	// dropped 统计因队列满被显式降级丢弃的消息数（原子计数）。
	dropped uint64
	// closedCount 仅用于测试断言（Close 幂等执行次数）。
	closedCount uint64

	// OnClose 是连接关闭时的回调（由 Upgrader 设置，用于从 Fanout 注销）。
	OnClose func(clientID string)

	// OnStateChange 在连接状态发生合法迁移时触发，便于外部观测连接生命周期。
	OnStateChange func(oldState, newState ConnState)

	mu        sync.Mutex
	state     ConnState
	closeOnce sync.Once
}

// compile-time interface check
var _ ClientMessenger = (*Client)(nil)

// NewClient 创建一个 Client，但不启动读取循环（调用方需自行 StartReadLoop）。
func NewClient(conn *nhooyrws.Conn, clientID string, claims *pkg.Claims) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	if conn != nil {
		// nhooyr 默认 32KB，超限会直接断连；必须与 StartReadLoop 的 readLimit 检查一致。
		conn.SetReadLimit(readLimit)
	}
	return &Client{
		id:      clientID,
		claims:  claims,
		conn:    conn,
		ctx:     ctx,
		cancel:  cancel,
		writeCh: make(chan []byte, 64),
		closed:  make(chan struct{}),
		state:   ConnStateNew,
	}
}

// ID 实现 ClientMessenger.ID。
func (c *Client) ID() string { return c.id }

// Claims 实现 ClientMessenger.Claims。
func (c *Client) Claims() *pkg.Claims { return c.claims }

// State 返回当前连接状态。
func (c *Client) State() ConnState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// setState 只允许合法的单向状态迁移，避免关闭后状态回退。
func (c *Client) setState(state ConnState) {
	c.mu.Lock()
	old := c.state
	if old == state || !validConnStateTransition(old, state) {
		c.mu.Unlock()
		return
	}
	c.state = state
	c.mu.Unlock()
	log.Printf("[ws] client=%s state=%s->%s", c.id, old, state)
	if c.OnStateChange != nil {
		c.OnStateChange(old, state)
	}
}

func validConnStateTransition(from, to ConnState) bool {
	switch from {
	case ConnStateNew:
		return to == ConnStateConnecting || to == ConnStateClosing || to == ConnStateClosed
	case ConnStateConnecting:
		return to == ConnStateOpen || to == ConnStateClosing || to == ConnStateClosed
	case ConnStateOpen:
		return to == ConnStateClosing || to == ConnStateClosed
	case ConnStateClosing:
		return to == ConnStateClosed
	default:
		return to == ConnStateClosed
	}
}

// StartReadLoop 启动读取循环，阻塞直到连接关闭。
// handler 按收每条消息。应作为 goroutine 调用。
func (c *Client) StartReadLoop(handler func(ClientMessenger, Message)) {
	c.setState(ConnStateConnecting)
	defer func() {
		c.setState(ConnStateClosing)
		c.cancel()
		c.closeClosed()
		c.setState(ConnStateClosed)
		if c.OnClose != nil {
			c.OnClose(c.ID())
		}
	}()

	go c.writeLoop()
	go c.heartbeatLoop()
	c.setState(ConnStateOpen)

	for {
		_, msgBytes, err := c.conn.Read(c.ctx)
		if err != nil {
			return
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
	defer c.cancel()
	for {
		select {
		case data := <-c.writeCh:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := c.conn.Write(ctx, nhooyrws.MessageText, data)
			cancel()
			if err != nil {
				log.Printf("[ws] write error client=%s: %v", c.id, err)
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// heartbeatLoop 周期性发送 ping 探测对端可达性；写超时视为连接故障并主动关闭。
func (c *Client) heartbeatLoop() {
	if c.conn == nil {
		return
	}
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, heartbeatWriteTimeout)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				log.Printf("[ws] heartbeat failed client=%s: %v", c.id, err)
				c.Close()
				return
			}
		}
	}
}

// Send 实现 ClientMessenger.Send — goroutine-safe。
// 返回 false 表示客户端已断开或队列满，消息未发送。
func (c *Client) Send(v interface{}) bool {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return false
	}
	return c.sendRaw(data)
}

// sendRaw 将已序列化数据写入发送队列；返回 false 表示客户端已断开或队列满。
func (c *Client) sendRaw(data []byte) bool {
	t := time.NewTimer(sendQueueTimeout)
	defer t.Stop()
	// 先做一次非阻塞检查：Close 后 writeCh 可能仍为空，select 会随机选中写入分支。
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

// SendACK 实现 ClientMessenger.SendACK。
func (c *Client) SendACK(id, event string, data interface{}) {
	c.Send(ACK{ID: id, Event: event, Data: data})
}

// SendErrorACK 实现 ClientMessenger.SendErrorACK。
func (c *Client) SendErrorACK(id, event string, code int, message string) {
	c.Send(ACK{ID: id, Event: event, Error: &ACKError{Code: code, Message: message}})
}

// DroppedCount 返回因发送队列满被显式降级丢弃的消息总数（统计入口）。
func (c *Client) DroppedCount() uint64 {
	return atomic.LoadUint64(&c.dropped)
}

// Close 实现 ClientMessenger.Close。
func (c *Client) Close() {
	c.setState(ConnStateClosing)
	c.cancel()
	c.closeClosed()
	if c.conn != nil {
		_ = c.conn.Close(nhooyrws.StatusNormalClosure, "server closing")
	}
	c.setState(ConnStateClosed)
}

// closeClosed 以幂等方式关闭 closed channel，保证 Close 后 Send 立即返回 false。
func (c *Client) closeClosed() {
	c.closeOnce.Do(func() {
		atomic.AddUint64(&c.closedCount, 1)
		close(c.closed)
	})
}
