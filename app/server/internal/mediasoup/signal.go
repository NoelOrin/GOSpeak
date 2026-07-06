package mediasoup

import (
	"encoding/json"
	"log"
	"runtime/debug"

	socketio "github.com/googollee/go-socket.io"
)

type BroadcastFn func(room, event string, data interface{})

type MediasoupSignal struct {
	bridge    *BridgeClient
	broadcast BroadcastFn
}

func NewMediasoupSignal(bridge *BridgeClient, broadcast BroadcastFn) *MediasoupSignal {
	return &MediasoupSignal{bridge: bridge, broadcast: broadcast}
}

// safeHandler wraps a socket.io ack handler with panic recovery.
// 使用 named return (ret/err)：recover 在 defer 中需给命名返回值赋值才能覆盖 panic 路径的结果。
// panic 时 err 设为 nil：错误已写入 ret 的 JSON body，让 socket.io 走成功 ack 通道把 body 回传客户端，
// 避免库层 error（NACK/断连）干扰应用层错误语义。debug.Stack 记录调用栈便于定位。
func safeHandler(fn func(s socketio.Conn, payload string) (string, error)) func(s socketio.Conn, payload string) (ret string, err error) {
	return func(s socketio.Conn, payload string) (ret string, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[mediasoup] handler panic: %v\n%s", r, debug.Stack())
				ret = `{"error":"internal server error"}`
				err = nil
			}
		}()
		return fn(s, payload)
	}
}

func (m *MediasoupSignal) RegisterRoutes(server *socketio.Server) {
	server.OnEvent("/", "sfu:get-router-capabilities", safeHandler(func(s socketio.Conn, payload string) (string, error) {
		var req struct {
			Room string `json:"room"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" {
			return `{"error":"room required"}`, nil
		}
		rtpCaps, err := m.bridge.GetRouterCapabilities(req.Room)
		if err != nil {
			return errorJSON(err), nil
		}
		return marshalJSON(map[string]interface{}{"rtpCapabilities": rtpCaps}), nil
	}))

	server.OnEvent("/", "sfu:create-transport", safeHandler(func(s socketio.Conn, payload string) (string, error) {
		var req struct {
			Room      string `json:"room"`
			Direction string `json:"direction,omitempty"`
			Identity  string `json:"identity,omitempty"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" {
			return `{"error":"room required"}`, nil
		}
		params, err := m.bridge.CreateTransport(req.Room, req.Identity, req.Direction)
		if err != nil {
			return errorJSON(err), nil
		}
		return marshalJSON(params), nil
	}))

	server.OnEvent("/", "sfu:connect-transport", safeHandler(func(s socketio.Conn, payload string) (string, error) {
		var req struct {
			Room           string          `json:"room"`
			TransportID    string          `json:"transportId"`
			DtlsParameters json.RawMessage `json:"dtlsParameters"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" || req.TransportID == "" {
			return `{"error":"room and transportId required"}`, nil
		}
		if err := m.bridge.ConnectTransport(req.Room, req.TransportID, req.DtlsParameters); err != nil {
			return errorJSON(err), nil
		}
		return `{"ok":true}`, nil
	}))

	server.OnEvent("/", "sfu:produce", safeHandler(func(s socketio.Conn, payload string) (string, error) {
		var req struct {
			Room          string          `json:"room"`
			TransportID   string          `json:"transportId"`
			Kind          string          `json:"kind"`
			RTPParameters json.RawMessage `json:"rtpParameters"`
			AppData       json.RawMessage `json:"appData"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" || req.TransportID == "" {
			return `{"error":"room and transportId required"}`, nil
		}
		result, err := m.bridge.Produce(req.Room, req.TransportID, req.Kind, req.RTPParameters, req.AppData)
		if err != nil {
			return errorJSON(err), nil
		}
		if m.broadcast != nil {
			m.broadcast(req.Room, "sfu:producer-ready", map[string]interface{}{
				"room":       req.Room,
				"producerId": result.ID,
				"kind":       result.Kind,
				"appData":    req.AppData,
			})
		}
		return marshalJSON(result), nil
	}))

	server.OnEvent("/", "sfu:consume", safeHandler(func(s socketio.Conn, payload string) (string, error) {
		var req struct {
			Room            string          `json:"room"`
			TransportID     string          `json:"transportId"`
			ProducerID      string          `json:"producerId"`
			RTPCapabilities json.RawMessage `json:"rtpCapabilities"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" || req.TransportID == "" || req.ProducerID == "" {
			return `{"error":"room, transportId, producerId required"}`, nil
		}
		result, err := m.bridge.Consume(req.Room, req.TransportID, req.ProducerID, req.RTPCapabilities)
		if err != nil {
			return errorJSON(err), nil
		}
		return marshalJSON(result), nil
	}))
}

// marshalJSON 序列化 v；失败时记录错误并返回占位 JSON，避免向客户端发送空串或无效 JSON。
func marshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[mediasoup] json marshal error: %v", err)
		return `{"error":"serialization failed"}`
	}
	return string(data)
}

func errorJSON(err error) string {
	return marshalJSON(map[string]string{"error": err.Error()})
}
