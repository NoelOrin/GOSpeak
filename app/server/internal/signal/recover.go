package signal

import (
	"fmt"
	"log"
	"runtime/debug"

	socketio "github.com/googollee/go-socket.io"
)

// 本文件为 Hub 的 socket.io 事件 handler 提供 panic recovery。
// go-socket.io v1.7.0 的 dispatch 不捕获 handler panic，任一 handler panic 会
// 蔓延到连接 goroutine 顶层导致整个 server 不可用。mediasoup 包的 ack handler
// 自带 safeHandler（签名带 ack 返回值），此处覆盖 Hub 的连接/断开/事件/error handler。

// logPanic 记录 handler panic 及调用栈。
func logPanic(prefix string, r interface{}) {
	log.Printf("[Signal] %s panic: %v\n%s", prefix, r, debug.Stack())
}

// safeOnConnect 包装 OnConnect handler；panic 时返回 error，交由库走 OnError 通道清理连接。
func safeOnConnect(fn func(socketio.Conn) error) func(socketio.Conn) error {
	return func(s socketio.Conn) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("OnConnect", r)
				err = fmt.Errorf("onconnect panic: %v", r)
			}
		}()
		return fn(s)
	}
}

// safeOnDisconnect 包装 OnDisconnect handler。
func safeOnDisconnect(fn func(socketio.Conn, string)) func(socketio.Conn, string) {
	return func(s socketio.Conn, reason string) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("OnDisconnect", r)
			}
		}()
		fn(s, reason)
	}
}

// safeOnEventData 包装带 payload 的 OnEvent handler（room:create/join/leave/kick/mute 等）。
func safeOnEventData(fn func(socketio.Conn, string)) func(socketio.Conn, string) {
	return func(s socketio.Conn, data string) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("OnEvent", r)
			}
		}()
		fn(s, data)
	}
}

func safeOnEventDataAck(fn func(socketio.Conn, string) (string, error)) func(socketio.Conn, string) (ret string, err error) {
	return func(s socketio.Conn, data string) (ret string, err error) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("OnEventAck", r)
				ret = `{"error":"internal server error"}`
				err = nil
			}
		}()
		return fn(s, data)
	}
}

// safeOnEventNoData 包装无 payload 的 OnEvent handler（room:list）。
func safeOnEventNoData(fn func(socketio.Conn)) func(socketio.Conn) {
	return func(s socketio.Conn) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("OnEvent", r)
			}
		}()
		fn(s)
	}
}

// safeOnError 包装 OnError handler；防止 error 处理逻辑自身 panic。
func safeOnError(fn func(socketio.Conn, error)) func(socketio.Conn, error) {
	return func(s socketio.Conn, err error) {
		defer func() {
			if r := recover(); r != nil {
				logPanic("OnError", r)
			}
		}()
		fn(s, err)
	}
}
