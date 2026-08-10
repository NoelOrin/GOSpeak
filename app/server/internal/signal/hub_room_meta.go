package signal

import (
	"context"
	"errors"
	"log"
	"time"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
)

// roomMetaStore 是 membershipStore 可选实现的扩展接口：
// Redis/NATS 后端提供跨实例房间元数据，测试 double 不实现时静默降级。
type roomMetaStore interface {
	PutRoomMeta(ctx context.Context, room string, meta bus.RoomMeta) error
	GetRoomMeta(ctx context.Context, room string) (bus.RoomMeta, error)
	DeleteRoomMeta(ctx context.Context, room string) error
}

var errRoomMetaNotFound = errors.New("room meta not found")

// syncRoomMeta 写入共享房间元数据，使远端实例能正确校验临时房间的
// 密码/类型/人数上限，避免跨实例绕过密码或产生分裂状态。
func (h *Hub) syncRoomMeta(key string, meta bus.RoomMeta) {
	if h.membershipStore == nil {
		return
	}
	s, ok := h.membershipStore.(roomMetaStore)
	if !ok {
		return
	}
	ctx, cancel := kvTimeoutCtx()
	err := s.PutRoomMeta(ctx, key, meta)
	cancel()
	if err != nil {
		log.Printf("[Signal] state store put room meta %s: %v", key, err)
	}
}

func (h *Hub) getRoomMeta(key string) (bus.RoomMeta, error) {
	if h.membershipStore == nil {
		return bus.RoomMeta{}, errRoomMetaNotFound
	}
	s, ok := h.membershipStore.(roomMetaStore)
	if !ok {
		return bus.RoomMeta{}, errRoomMetaNotFound
	}
	ctx, cancel := kvTimeoutCtx()
	meta, err := s.GetRoomMeta(ctx, key)
	cancel()
	if err != nil {
		if bus.IsMembershipNotFound(err) {
			return bus.RoomMeta{}, errRoomMetaNotFound
		}
		log.Printf("[Signal] state store get room meta %s: %v", key, err)
		return bus.RoomMeta{}, err
	}
	return meta, nil
}

func (h *Hub) deleteRoomMeta(key string) {
	if h.membershipStore == nil {
		return
	}
	s, ok := h.membershipStore.(roomMetaStore)
	if !ok {
		return
	}
	ctx, cancel := kvTimeoutCtx()
	err := s.DeleteRoomMeta(ctx, key)
	cancel()
	if err != nil && !bus.IsMembershipNotFound(err) {
		log.Printf("[Signal] state store delete room meta %s: %v", key, err)
	}
}

// ensureRoomOwnership 在媒体面加入成功后把当前实例登记为房间持有者，
// Agent 进房 token 才能按 (domain_uuid, room) 解析到同一个 Worker。
func (h *Hub) ensureRoomOwnership(key string, req RoomRequest, room *Room) {
	meta, err := h.getRoomMeta(key)
	if err != nil {
		meta = bus.RoomMeta{
			Name:        req.Room,
			DomainUUID:  req.DomainUUID,
			Type:        model.RoomTypeVoice,
			CreatedAtMS: time.Now().UnixMilli(),
		}
	}
	meta.OwnerNodeID = h.instanceID
	if room != nil {
		if meta.Password == "" {
			meta.Password = room.Password
		}
		if meta.Type == "" {
			meta.Type = model.RoomTypeVoice
		}
	}
	h.syncRoomMeta(key, meta)
}
