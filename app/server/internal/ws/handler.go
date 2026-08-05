package ws

import (
	"log"

	"GOSpeak/internal/pkg"
)

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

// NewHandlerRegistry 创建一个空的事件注册表。
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
				code, clientMsg := pkg.ClientError(err)
				c.SendErrorACK(msg.ID, msg.Event, int(code), clientMsg)
			} else {
				c.SendACK(msg.ID, msg.Event, result)
			}
		}
	} else if entry.NoAck != nil {
		entry.NoAck(c, dataStr)
	}
}
