package bus

import (
	"encoding/json"
	"sync"
)

// InternalDispatcher 按事件名分发内部事件到注册的 handler，取代组合根里的
// if-else 闭包分派。线程安全；未注册的事件返回 false 交回调用方兜底。
type InternalDispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]func(payload interface{})
}

func NewInternalDispatcher() *InternalDispatcher {
	return &InternalDispatcher{handlers: make(map[string][]func(payload interface{}))}
}

// Register 登记事件 handler；同一事件可多订阅，按注册顺序执行。
func (d *InternalDispatcher) Register(event string, handler func(payload interface{})) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[event] = append(d.handlers[event], handler)
}

// Dispatch 依次执行注册 handler；返回是否有订阅者。
func (d *InternalDispatcher) Dispatch(event string, payload interface{}) bool {
	d.mu.RLock()
	handlers, ok := d.handlers[event]
	d.mu.RUnlock()
	if !ok {
		return false
	}
	for _, h := range handlers {
		h(payload)
	}
	return true
}

// AsRemoteHook 适配为 NATSBus.RemoteHook。
func (d *InternalDispatcher) AsRemoteHook() func(event string, payload interface{}) {
	return func(event string, payload interface{}) { d.Dispatch(event, payload) }
}

// DecodePayload 把内部事件 payload 还原为类型化 struct。payload 经 NATS JSON
// 往返后是 map[string]interface{}；订阅方一律经本 helper 还原，禁止手工拆箱。
func DecodePayload[T any](payload interface{}) (*T, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	out := new(T)
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, err
	}
	return out, nil
}
