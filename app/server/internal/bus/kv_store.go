package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// MemberRecord is one online member in a room membership snapshot.
type MemberRecord struct {
	Room        string `json:"room"`
	Identity    string `json:"identity"`
	SocketHint  string `json:"socket_hint,omitempty"`
	InstanceID  string `json:"instance_id"`
	Stream      string `json:"stream,omitempty"`
	MicMuted    bool   `json:"mic_muted"`
	Speaking    bool   `json:"speaking"`
	UpdatedAtMS int64  `json:"updated_at_ms"`
	// ExpiresAtMS 是成员 lease 到期时间；0 表示旧格式记录（按未过期处理）。
	// 实例崩溃/分区后，其他实例 merge 时会过滤过期记录，避免幽灵成员无限残留。
	ExpiresAtMS int64 `json:"expires_at_ms,omitempty"`
}

// RoomMembersSnapshot is the KV value for a room's membership view.
type RoomMembersSnapshot struct {
	Room      string         `json:"room"`
	Members   []MemberRecord `json:"members"`
	UpdatedAt int64          `json:"updated_at_ms"`
}

// RoomMeta 是跨实例共享的房间元数据（密码 hash、类型、人数上限等），
// 用于 DB 无记录的临时房间在多实例间保持一致，避免绕过密码或状态分裂。
type RoomMeta struct {
	Name        string `json:"name,omitempty"`
	DomainUUID  string `json:"domain_uuid,omitempty"`
	Password    string `json:"password,omitempty"` // hashed
	Type        string `json:"type,omitempty"`
	Limit       uint   `json:"limit,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAtMS int64  `json:"created_at_ms,omitempty"`
}

// StateStoreConfig opens JetStream KV buckets for membership/stream state.
type StateStoreConfig struct {
	URL    string
	Prefix string
	NC     *nats.Conn // optional: reuse existing connection
}

// StateStore wraps membership + stream JetStream KV buckets.
type StateStore struct {
	nc   *nats.Conn
	js   nats.JetStreamContext
	mem  nats.KeyValue
	strm nats.KeyValue
	own  bool
}

// OpenStateStore creates/opens membership and stream KV buckets.
func OpenStateStore(cfg StateStoreConfig) (*StateStore, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}

	var nc *nats.Conn
	var err error
	own := false
	if cfg.NC != nil {
		nc = cfg.NC
	} else {
		if strings.TrimSpace(cfg.URL) == "" {
			return nil, fmt.Errorf("state store: empty URL and nil NC")
		}
		nc, err = nats.Connect(cfg.URL, nats.Name(cfg.Prefix+"-state"), nats.Timeout(2*time.Second))
		if err != nil {
			return nil, fmt.Errorf("state store connect: %w", err)
		}
		own = true
	}

	js, err := nc.JetStream()
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("state store jetstream: %w", err)
	}

	mem, err := openOrCreateKV(js, cfg.Prefix+"_membership")
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("membership kv: %w", err)
	}
	strm, err := openOrCreateKV(js, cfg.Prefix+"_stream")
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("stream kv: %w", err)
	}

	return &StateStore{nc: nc, js: js, mem: mem, strm: strm, own: own}, nil
}

func openOrCreateKV(js nats.JetStreamContext, bucket string) (nats.KeyValue, error) {
	kv, err := js.KeyValue(bucket)
	if err == nil {
		return kv, nil
	}
	return js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: bucket,
		TTL:    24 * time.Hour,
	})
}

func (s *StateStore) PutRoomMembers(ctx context.Context, snap RoomMembersSnapshot) error {
	_ = ctx
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = s.mem.Put("room."+sanitizeKey(snap.Room), b)
	return err
}

func (s *StateStore) GetRoomMembers(ctx context.Context, room string) (RoomMembersSnapshot, error) {
	_ = ctx
	entry, err := s.mem.Get("room." + sanitizeKey(room))
	if err != nil {
		return RoomMembersSnapshot{}, err
	}
	var snap RoomMembersSnapshot
	if err := json.Unmarshal(entry.Value(), &snap); err != nil {
		return RoomMembersSnapshot{}, err
	}
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	return snap, nil
}

// GetRoomMembersRev 返回 membership 快照及其 NATS KV revision，供乐观锁合并使用。
func (s *StateStore) GetRoomMembersRev(ctx context.Context, room string) (RoomMembersSnapshot, uint64, error) {
	_ = ctx
	entry, err := s.mem.Get("room." + sanitizeKey(room))
	if err != nil {
		return RoomMembersSnapshot{}, 0, err
	}
	var snap RoomMembersSnapshot
	if err := json.Unmarshal(entry.Value(), &snap); err != nil {
		return RoomMembersSnapshot{}, 0, err
	}
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	return snap, entry.Revision(), nil
}

func (s *StateStore) DeleteRoomMembers(ctx context.Context, room string) error {
	_ = ctx
	return s.mem.Delete("room." + sanitizeKey(room))
}

// PutRoomMembersRev 使用 NATS KV revision 乐观写入：rev=0 表示仅创建，非零表示期望的当前 revision。
func (s *StateStore) PutRoomMembersRev(ctx context.Context, snap RoomMembersSnapshot, rev uint64) error {
	_ = ctx
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	key := "room." + sanitizeKey(snap.Room)
	if rev == 0 {
		_, err = s.mem.Create(key, b)
	} else {
		_, err = s.mem.Update(key, b, rev)
	}
	return err
}

// DeleteRoomMembersRev 仅在最新 revision 匹配时删除 membership 记录。
func (s *StateStore) DeleteRoomMembersRev(ctx context.Context, room string, rev uint64) error {
	_ = ctx
	if rev == 0 {
		return nil
	}
	return s.mem.Delete("room."+sanitizeKey(room), nats.LastRevision(rev))
}

func (s *StateStore) PutStream(ctx context.Context, stream, room, identity string) error {
	_ = ctx
	b, err := json.Marshal(map[string]string{
		"room":     room,
		"identity": identity,
	})
	if err != nil {
		return err
	}
	_, err = s.strm.Put("stream."+sanitizeKey(stream), b)
	return err
}

func (s *StateStore) GetStream(ctx context.Context, stream string) (room, identity string, err error) {
	_ = ctx
	entry, err := s.strm.Get("stream." + sanitizeKey(stream))
	if err != nil {
		return "", "", err
	}
	var m map[string]string
	if err := json.Unmarshal(entry.Value(), &m); err != nil {
		return "", "", err
	}
	return m["room"], m["identity"], nil
}

func (s *StateStore) DeleteStream(ctx context.Context, stream string) error {
	_ = ctx
	return s.strm.Delete("stream." + sanitizeKey(stream))
}

func (s *StateStore) PutRoomMeta(ctx context.Context, room string, meta RoomMeta) error {
	_ = ctx
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = s.mem.Put("meta."+sanitizeKey(room), b)
	return err
}

func (s *StateStore) GetRoomMeta(ctx context.Context, room string) (RoomMeta, error) {
	_ = ctx
	entry, err := s.mem.Get("meta." + sanitizeKey(room))
	if err != nil {
		return RoomMeta{}, err
	}
	var meta RoomMeta
	if err := json.Unmarshal(entry.Value(), &meta); err != nil {
		return RoomMeta{}, err
	}
	return meta, nil
}

func (s *StateStore) DeleteRoomMeta(ctx context.Context, room string) error {
	_ = ctx
	return s.mem.Delete("meta." + sanitizeKey(room))
}

// ListRoomNames returns room names currently present in membership KV,
// including rooms that only have shared metadata (remote temporary rooms).
func (s *StateStore) ListRoomNames(ctx context.Context) ([]string, error) {
	_ = ctx
	keys, err := s.mem.Keys()
	if err != nil {
		// empty bucket surfaces as ErrNoKeysFound on some nats versions
		if err == nats.ErrNoKeysFound {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		name := ""
		switch {
		case strings.HasPrefix(k, "room."):
			name = strings.TrimPrefix(k, "room.")
		case strings.HasPrefix(k, "meta."):
			name = strings.TrimPrefix(k, "meta.")
		}
		if name == "" {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if existing == name {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, name)
		}
	}
	return out, nil
}

func (s *StateStore) Backend() string { return "nats" }

// Close closes the underlying connection only when OpenStateStore created it.
func (s *StateStore) Close() error {
	if s == nil {
		return nil
	}
	if s.own && s.nc != nil {
		s.nc.Close()
	}
	return nil
}

func sanitizeKey(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}
