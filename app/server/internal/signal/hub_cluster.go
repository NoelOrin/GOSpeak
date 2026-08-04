package signal

import (
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/ws"
	"context"
	"fmt"
	"log"
	"strings"
)

// OnDomainDelete 清理指定 Domain 下所有信令房间（房间键前缀匹配）。
// - 断开所有在线成员连接
// - 删除本地 rooms map 中的条目
// - 调用 SFU Provider 清理实际房间
func (h *Hub) OnDomainDelete(domainUUID string) {
	if domainUUID == "" {
		return
	}
	prefix := domainUUID + ":"

	var sfuRooms []string
	var roomKeys []string
	var deletedStreams []string
	var closedClients []ws.ClientMessenger
	h.mu.Lock()
	for key, room := range h.rooms {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		// 收集 SFU 房间名（复合 key 与 token/媒体层一致）
		sfuRooms = append(sfuRooms, key)
		// 断开所有成员
		for sid := range room.Members {
			if m := room.Members[sid]; m != nil {
				h.unregisterStreamLocked(key, m.Identity, m.Stream)
				if m.Stream != "" {
					deletedStreams = append(deletedStreams, m.Stream)
				}
			}
			delete(h.connSlots, sid)
			if client := h.fanout.GetClient(sid); client != nil {
				// Fanout.GetClient 对已删除 key 会返回带类型的 nil 指针；该 socket 已断开，跳过避免误 Close。
				if typed, ok := client.(*ws.Client); ok && typed == nil {
					continue
				}
				closedClients = append(closedClients, client)
			}
			h.fanout.Remove(sid)
		}
		delete(h.rooms, key)
		roomKeys = append(roomKeys, key)
	}
	h.mu.Unlock()

	// 在锁外关闭成员连接，避免 Close 阻塞拖住所有信令。
	closed := make(map[string]struct{}, len(closedClients))
	for _, client := range closedClients {
		if _, ok := closed[client.ID()]; ok {
			continue
		}
		closed[client.ID()] = struct{}{}
		client.Close()
	}

	// 清理 SFU 侧
	for _, room := range sfuRooms {
		h.deleteRoomSafe(room)
	}
	// 清理共享状态和 stream 映射
	for _, key := range roomKeys {
		if h.membershipStore != nil {
			_ = h.membershipStore.DeleteRoomMembers(context.Background(), key)
			h.notifyRoomStateChanged(key)
		}
	}
	for _, stream := range deletedStreams {
		if stream != "" {
			h.syncStreamDelete(stream)
		}
	}
}

// HandleRemoteEvent 处理来自其他实例的控制面事件。
// HandleClusterCommand 执行 Agent 下发的本地信令控制命令。

// HandleRemoteEvent 处理来自其他实例的控制面事件。
// HandleClusterCommand 执行 Agent 下发的本地信令控制命令。
func (h *Hub) HandleClusterCommand(cmd cluster.ControlCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}
	switch cmd.Command {
	case cluster.CommandKick:
		h.KickFromRoom(cmd.DomainUUID, cmd.Room, cmd.Identity)
		return nil
	case cluster.CommandDeleteRoom:
		h.DeleteRoomByDomainName(cmd.DomainUUID, cmd.Room)
		return nil
	case cluster.CommandDeleteServer:
		h.OnDomainDelete(cmd.DomainUUID)
		return nil
	case cluster.CommandMute:
		if userID, ok := controlCommandUserID(cmd); ok {
			h.enforceUserMediaMute(userID, true, 0)
		}
		return nil
	case cluster.CommandUnmute:
		if userID, ok := controlCommandUserID(cmd); ok {
			h.enforceUserMediaMute(userID, false, 0)
		}
		return nil
	default:
		return fmt.Errorf("unsupported cluster command %q", cmd.Command)
	}
}

// KickFromRoom 从指定房间移除 identity 对应的本地信令成员，并尽力触发 SFU 媒体移除。

// KickFromRoom 从指定房间移除 identity 对应的本地信令成员，并尽力触发 SFU 媒体移除。
func (h *Hub) KickFromRoom(domainUUID, room, targetIdentity string) {
	if targetIdentity == "" {
		return
	}
	key := roomKey(domainUUID, room)
	h.mu.Lock()
	roomState, ok := h.rooms[key]
	if !ok {
		h.mu.Unlock()
		return
	}
	var targetSocketID string
	var targetStream string
	roomDeleted := false
	for sid, member := range roomState.Members {
		if member == nil || member.Identity != targetIdentity {
			continue
		}
		h.unregisterStreamLocked(key, member.Identity, member.Stream)
		roomDelMember(roomState, sid)
		delete(roomState.Speaking, targetIdentity)
		delete(roomState.MicMuted, targetIdentity)
		delete(h.connSlots, sid)
		if h.fanout != nil {
			h.fanout.Leave(key, sid)
		}
		targetSocketID = sid
		targetStream = member.Stream
		roomDeleted = h.deleteRoomIfEmptyLocked(key)
		break
	}
	h.mu.Unlock()
	if targetSocketID == "" {
		return
	}

	h.clearConnRoomSlot(targetSocketID, key)
	if targetStream != "" {
		h.syncStreamDelete(targetStream)
	}

	enforcement := h.removeParticipantSafe(key, targetIdentity)

	// Fanout.GetClient 对已删除 key 会返回带类型的 nil 指针；该 socket 已断开，跳过避免误 Close。
	var targetConn ws.ClientMessenger
	if h.fanout != nil {
		if client := h.fanout.GetClient(targetSocketID); client != nil {
			if typed, ok := client.(*ws.Client); !ok || typed != nil {
				targetConn = client
			}
		}
	}

	h.publishRoom(key, EventRoomKicked, map[string]interface{}{
		"room":           room,
		"domain_uuid":    domainUUID,
		"targetIdentity": targetIdentity,
		"enforcement":    enforcement,
	})
	if targetConn != nil {
		targetConn.Send(map[string]interface{}{"event": EventRoomKicked, "data": map[string]interface{}{
			"room":           room,
			"domain_uuid":    domainUUID,
			"targetIdentity": targetIdentity,
			"enforcement":    enforcement,
		}})
	}

	h.publishRoom(key, EventMemberLeft, map[string]interface{}{
		"room":        room,
		"domain_uuid": domainUUID,
		"identity":    targetIdentity,
		"id":          targetSocketID,
		"enforcement": enforcement,
	})

	if targetConn != nil {
		targetConn.Close()
	}

	h.syncRoomToStore(key)
	if h.participantCleanup != nil {
		h.participantCleanup.OnParticipantLeft(key, targetIdentity)
	}

	if roomDeleted {
		h.deleteRoomSafe(key)
		h.broadcastRoomList(domainUUID)
		return
	}
	h.broadcastRoomUpdatedLocal(key)
}

// DeleteRoomByDomainName 删除指定 Domain 下的单个信令房间，并清理 SFU 与共享状态。

// DeleteRoomByDomainName 删除指定 Domain 下的单个信令房间，并清理 SFU 与共享状态。
func (h *Hub) DeleteRoomByDomainName(domainUUID, room string) {
	key := roomKey(domainUUID, room)
	var deletedStreams []string
	var closedClients []ws.ClientMessenger
	h.mu.Lock()
	roomState, ok := h.rooms[key]
	if !ok {
		h.mu.Unlock()
		return
	}
	for sid, member := range roomState.Members {
		if member == nil {
			continue
		}
		h.unregisterStreamLocked(key, member.Identity, member.Stream)
		if member.Stream != "" {
			deletedStreams = append(deletedStreams, member.Stream)
		}
		roomDelMember(roomState, sid)
		delete(h.connSlots, sid)
		if h.fanout != nil {
			h.fanout.Leave(key, sid)
			// Fanout.GetClient 对已删除 key 会返回带类型的 nil 指针；该 socket 已断开，跳过避免误 Close。
			if client := h.fanout.GetClient(sid); client != nil {
				if typed, ok := client.(*ws.Client); !ok || typed != nil {
					closedClients = append(closedClients, client)
				}
			}
		}
	}
	delete(h.rooms, key)
	h.mu.Unlock()

	// 在锁外关闭成员连接，避免 Close 阻塞拖住所有信令。
	closed := make(map[string]struct{}, len(closedClients))
	for _, client := range closedClients {
		if _, ok := closed[client.ID()]; ok {
			continue
		}
		closed[client.ID()] = struct{}{}
		client.Close()
	}
	for _, stream := range deletedStreams {
		h.syncStreamDelete(stream)
	}
	h.deleteRoomSafe(key)
	h.syncRoomToStore(key)
	h.broadcastRoomList(domainUUID)
}

func controlCommandUserID(cmd cluster.ControlCommand) (uint, bool) {
	if cmd.Payload == nil {
		return 0, false
	}
	switch v := cmd.Payload["user_id"].(type) {
	case float64:
		return uint(v), true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case uint:
		return v, true
	}
	return 0, false
}

// 当前仅对 sfu:provider-changed 做本机房间清理；其余事件已由 EventBus 投递到本地 WebSocket。

// 当前仅对 sfu:provider-changed 做本机房间清理；其余事件已由 EventBus 投递到本地 WebSocket。
func (h *Hub) HandleRemoteEvent(event string, payload interface{}) {
	switch event {
	case EventSFUProviderChanged:
		provider := ""
		switch p := payload.(type) {
		case map[string]interface{}:
			if v, ok := p["provider"].(string); ok {
				provider = v
			}
		}
		log.Printf("[Signal] remote SFU provider switch received: %s", provider)
		h.clearLocalRoomsForSFUSwitch()
	case EventStateRoomChanged:
		room := ""
		switch p := payload.(type) {
		case map[string]interface{}:
			if v, ok := p["room"].(string); ok {
				room = v
			}
		}
		h.ApplyRemoteRoomState(room)
	default:
		// other events already delivered to local WebSocket by EventBus
	}
}

// clearLocalRoomsForSFUSwitch 清理本机信令房间与 stream 视图（不重复广播 provider-changed）。
