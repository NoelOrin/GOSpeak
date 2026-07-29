package signal

import (
	"log"
	"runtime/debug"

	"GOSpeak/internal/ws"
)

// 本文件为 Hub 的信号事件 handler 提供 panic recovery 双层防护。
// 第一层由 ws.HandlerRegistry.Dispatch 统一兜底；此处 safeHandler/safeHandlerAck
// 为第二层，防止单个 handler panic 影响其他 handler。

// logPanic 记录 handler panic 及调用栈。
func logPanic(prefix string, r interface{}) {
	log.Printf("[Signal] %s panic: %v\n%s", prefix, r, debug.Stack())
}

// safeHandler 包装无应答 handler（fire-and-forget）。
func safeHandler(fn func(ws.ClientMessenger, string)) func(ws.ClientMessenger, string) {
	return func(c ws.ClientMessenger, data string) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("handler", r)
			}
		}()
		fn(c, data)
	}
}

// safeHandlerAck 包装应答 handler。
func safeHandlerAck(fn func(ws.ClientMessenger, string) (string, error)) func(ws.ClientMessenger, string) (string, error) {
	return func(c ws.ClientMessenger, data string) (ret string, err error) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("ack handler", r)
				ret = `{"error":"internal server error"}`
				err = nil
			}
		}()
		return fn(c, data)
	}
}

// safeHandlerNoData 包装无 payload handler（如 room:list）。
func safeHandlerNoData(fn func(ws.ClientMessenger)) func(ws.ClientMessenger, string) {
	return func(c ws.ClientMessenger, _ string) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("handler", r)
			}
		}()
		fn(c)
	}
}
