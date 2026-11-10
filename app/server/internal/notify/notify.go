// Package notify 提供服务端到客户端的定向推送原语。
//
// 传输复用现有 WebSocket：NotifyUser 投递到 fanout 伪房间 "__user:{uuid}"，
// 该房间的成员关系由 signal.Hub.OnConnect 建立、断连时清理。
//
// 能力边界：仅对在线连接可达。总线没有 user subject、没有 ack/offset，
// WAL 只覆盖「发布时 NATS 断线」且默认关闭。任何必须送达的通知
// 必须自带持久化 inbox（公告中心即如此），不能只依赖本包。
package notify

import (
	"context"
	"fmt"

	"GOSpeak/internal/pkg"
)

// UserRoomPrefix 是 fanout 中按用户定向投递的伪房间前缀。
const UserRoomPrefix = "__user:"

// EventBus 是本包依赖的最小发布面，由 bus.EventBus 满足。
type EventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
}

// NotifyUser 向单个用户的全部在线连接推送。
// payload 必须传 struct / map / json.RawMessage；禁止传 []byte，
// 否则 Fanout 二次 marshal 后前端会收到 base64 字符串而非对象。
func NotifyUser(ctx context.Context, bus EventBus, userUUID, event string, payload interface{}) error {
	if bus == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: nil event bus")
	}
	if userUUID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "notify: empty user uuid")
	}
	if event == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "notify: empty event")
	}
	if _, isBytes := payload.([]byte); isBytes {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: payload must not be []byte")
	}
	if err := bus.PublishRoom(ctx, UserRoomPrefix+userUUID, event, payload); err != nil {
		return fmt.Errorf("notify user %s: %w", userUUID, err)
	}
	return nil
}

// NotifyAll 向所有在线连接推送。
func NotifyAll(ctx context.Context, bus EventBus, event string, payload interface{}) error {
	if bus == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: nil event bus")
	}
	if event == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "notify: empty event")
	}
	if _, isBytes := payload.([]byte); isBytes {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "notify: payload must not be []byte")
	}
	if err := bus.PublishNamespace(ctx, event, payload); err != nil {
		return fmt.Errorf("notify all: %w", err)
	}
	return nil
}
