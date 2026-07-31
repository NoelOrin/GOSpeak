package mediasoup

import (
	"encoding/json"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"GOSpeak/internal/ws"
)

const recentCloseTTL = 5 * time.Minute

type BroadcastFn func(room, event string, data interface{})

type participantBridge interface {
	CreateRouter(roomID string) error
	GetRouterCapabilities(roomID string) (json.RawMessage, error)
	CreateTransport(roomID, identity, direction string) (*TransportParams, error)
	ConnectTransport(roomID, transportID string, dtlsParameters json.RawMessage) error
	Produce(roomID, transportID, kind string, rtpParameters, appData json.RawMessage) (*ProduceResult, error)
	Consume(roomID, transportID, producerID string, rtpCapabilities json.RawMessage) (*ConsumeResult, error)
	CloseParticipant(roomID, identity string) ([]string, error)
}

type MediasoupSignal struct {
	bridge    participantBridge
	broadcast BroadcastFn
	// recentClose dedups CloseParticipant + producer-closed broadcast per (room,identity):
	// OnDisconnect fires removeParticipantSafe (CloseParticipant) and OnParticipantLeft (CloseParticipant);
	// close-transport event may also call OnParticipantLeft before disconnect. Without a guard this is
	// up to 3 redundant HTTP round-trips + 2 broadcasts per leave. LoadOrStore marks first call wins.
	// Cleared on sfu:produce so a rejoining identity (same room+identity, new socket) cleans up again.
	// TTL-cleared by AfterFunc so an identity that leaves without rejoining doesn't leak its marker
	// forever — pointer equality guards against deleting a newer marker set by a rejoin.
	recentClose sync.Map
}

type dedupMarker struct{}

func NewMediasoupSignal(bridge *BridgeClient, broadcast BroadcastFn) *MediasoupSignal {
	return &MediasoupSignal{bridge: bridge, broadcast: broadcast}
}

// safeHandler wraps a WS ack handler with panic recovery.
// 使用 named return (ret/err)：recover 在 defer 中需给命名返回值赋值才能覆盖 panic 路径的结果。
// panic 时 err 设为 nil：错误已写入 ret 的 JSON body，让 WS 走成功 ack 通道把 body 回传客户端，
// 避免库层 error（NACK/断连）干扰应用层错误语义。debug.Stack 记录调用栈便于定位。
func safeHandler(fn func(s ws.ClientMessenger, payload string) (string, error)) func(s ws.ClientMessenger, payload string) (ret string, err error) {
	return func(s ws.ClientMessenger, payload string) (ret string, err error) {
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

func (m *MediasoupSignal) RegisterWS(register func(event string, fn func(ws.ClientMessenger, string) (string, error))) {
	register("sfu:get-router-capabilities", safeHandler(func(s ws.ClientMessenger, payload string) (string, error) {
		var req struct {
			Room string `json:"room"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" {
			return `{"error":"room required"}`, nil
		}
		// 确保 router 存在（worker.createRouter 幂等）：get-router-capabilities 是 mediasoup 客户端首调，
		// router 此前仅由 GenerateToken 副作用创建，移除副作用后在此懒建，避免 404。
		if err := m.bridge.CreateRouter(req.Room); err != nil {
			return errorJSON(err), nil
		}
		rtpCaps, err := m.bridge.GetRouterCapabilities(req.Room)
		if err != nil {
			return errorJSON(err), nil
		}
		return marshalJSON(map[string]interface{}{"rtpCapabilities": rtpCaps}), nil
	}))

	register("sfu:create-transport", safeHandler(func(s ws.ClientMessenger, payload string) (string, error) {
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

	register("sfu:connect-transport", safeHandler(func(s ws.ClientMessenger, payload string) (string, error) {
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

	register("sfu:produce", safeHandler(func(s ws.ClientMessenger, payload string) (string, error) {
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
		var appDataMap map[string]interface{}
		_ = json.Unmarshal(req.AppData, &appDataMap)
		if id, ok := appDataMap["identity"].(string); ok && id != "" {
			m.recentClose.Delete(req.Room + "\x00" + id)
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

	register("sfu:consume", safeHandler(func(s ws.ClientMessenger, payload string) (string, error) {
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

	register("sfu:close-transport", safeHandler(func(s ws.ClientMessenger, payload string) (string, error) {
		var req struct {
			Room     string `json:"room"`
			Identity string `json:"identity"`
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil || req.Room == "" || req.Identity == "" {
			return `{"error":"room and identity required"}`, nil
		}
		m.OnParticipantLeft(req.Room, req.Identity)
		return `{"ok":true}`, nil
	}))
}

func (m *MediasoupSignal) OnParticipantLeft(room, identity string) {
	key := room + "\x00" + identity
	marker := &dedupMarker{}
	if _, loaded := m.recentClose.LoadOrStore(key, marker); loaded {
		return
	}
	time.AfterFunc(recentCloseTTL, func() {
		if actual, ok := m.recentClose.Load(key); ok && actual == marker {
			m.recentClose.Delete(key)
		}
	})
	if m.broadcast != nil {
		m.broadcast(room, "sfu:producer-closed", map[string]interface{}{
			"room":     room,
			"identity": identity,
		})
	}
	go func() {
		if _, err := m.bridge.CloseParticipant(room, identity); err != nil {
			log.Printf("[mediasoup] CloseParticipant error: %v", err)
		}
	}()
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
