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

// PolicyDecision 是策略层对一条 WS 消息的判决。
// Code 为 pkg.SUCCESS 表示放行；Terminate 表示命中封禁，需同时关闭连接。
type PolicyDecision struct {
	Code      pkg.ErrCode
	Message   string
	Terminate bool
}

// PolicyGuard 在分发每条上行消息前执行策略判决。
// 由组合根注入，ws 包不依赖具体业务策略；判决必须每条消息实时执行，
// 因为域访客封禁不会失效已签发的 JWT，也不会主动断开已有连接。
type PolicyGuard func(c ClientMessenger, msg Message) PolicyDecision

// HandlerRegistry 管理事件名到处理函数的映射，提供统一分发入口。
// Hub 通过此注册表注册所有信号事件处理函数。
type HandlerRegistry struct {
	handlers map[string]handlerEntry
	guard    PolicyGuard
}

// NewHandlerRegistry 创建一个空的事件注册表。
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]handlerEntry),
	}
}

// SetPolicyGuard 注入传输无关策略的 WS 适配器。
// 未注入时保持原有行为（仅测试/嵌入场景使用），生产组合根必须注入。
func (r *HandlerRegistry) SetPolicyGuard(g PolicyGuard) {
	r.guard = g
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

	// 策略判决先于事件分发，且每条消息都重新执行：封禁不失效 JWT，
	// 只在建连时查一次会让被封访客凭已建立的长连接继续发消息/私信。
	if r.guard != nil {
		d := r.guard(c, msg)
		if d.Code != pkg.SUCCESS {
			clientMsg := d.Message
			if clientMsg == "" {
				clientMsg = pkg.GetErrMsg(d.Code)
			}
			log.Printf("[ws] policy rejected event=%s client=%s code=%d", msg.Event, c.ID(), d.Code)
			if msg.ID != "" {
				c.SendErrorACK(msg.ID, msg.Event, int(d.Code), clientMsg)
			}
			// 命中封禁时主动断连，同时停止后续上行与房间下行广播。
			if d.Terminate {
				c.Close()
			}
			return
		}
	}

	entry, ok := r.handlers[msg.Event]
	if !ok {
		log.Printf("[ws] unknown event: %s from client=%s", msg.Event, c.ID())
		if msg.ID != "" {
			c.SendErrorACK(msg.ID, msg.Event, int(pkg.INVALID_PARAMS), "unknown event")
		}
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
