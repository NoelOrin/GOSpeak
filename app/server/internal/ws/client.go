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

const (
	writeTimeout     = 5 * time.Second
	sendQueueTimeout = 500 * time.Millisecond
	readLimit        = 65536 // 64KB max message size
)

// Client 封装 nhooyr WebSocket 连接，提供 goroutine-safe 写能力。
// 实现 ClientMessenger 接口。
type Client struct {
	// id 是客户端唯一标识（通常是 UserUUID），通过 ID() 访问。
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

	// OnClose 是连接关闭时的回调（由 Upgrader 设置，用于从 Fanout 注销）。
	OnClose func(clientID string)

	mu sync.Mutex
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
	}
}

// ID 实现 ClientMessenger.ID。
func (c *Client) ID() string { return c.id }

// Claims 实现 ClientMessenger.Claims。
func (c *Client) Claims() *pkg.Claims { return c.claims }

// StartReadLoop 启动读取循环，阻塞直到连接关闭。
// handler 按收每条消息。应作为 goroutine 调用。
func (c *Client) StartReadLoop(handler func(ClientMessenger, Message)) {
	defer func() {
		c.cancel()
		close(c.closed)
		if c.OnClose != nil {
			c.OnClose(c.ID())
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

// Send 实现 ClientMessenger.Send — goroutine-safe。
// 返回 false 表示客户端已断开或队列满，消息未发送。
func (c *Client) Send(v interface{}) bool {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return false
	}
	t := time.NewTimer(sendQueueTimeout)
	defer t.Stop()
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
	c.cancel()
	if c.conn != nil {
		_ = c.conn.Close(nhooyrws.StatusNormalClosure, "server closing")
	}
}
