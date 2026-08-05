package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const redisStateTTL = 24 * time.Hour

// RedisStateStore stores membership/stream maps in Redis.
type RedisStateStore struct {
	rdb    *goredis.Client
	prefix string
}

// RedisStateStoreConfig opens a Redis-backed membership store.
type RedisStateStoreConfig struct {
	Client *goredis.Client
	Prefix string
}

// OpenRedisStateStore validates redis connectivity and returns a store.
func OpenRedisStateStore(cfg RedisStateStoreConfig) (*RedisStateStore, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("redis state store: nil client")
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cfg.Client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis state store ping: %w", err)
	}
	return &RedisStateStore{rdb: cfg.Client, prefix: cfg.Prefix}, nil
}

func (s *RedisStateStore) Backend() string { return "redis" }

func (s *RedisStateStore) roomKey(room string) string {
	return s.prefix + ":membership:room:" + sanitizeKey(room)
}

func (s *RedisStateStore) roomsSetKey() string {
	return s.prefix + ":membership:rooms"
}

func (s *RedisStateStore) streamKey(stream string) string {
	return s.prefix + ":stream:" + sanitizeKey(stream)
}

func (s *RedisStateStore) PutRoomMembers(ctx context.Context, snap RoomMembersSnapshot) error {
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.roomKey(snap.Room), b, redisStateTTL)
	pipe.SAdd(ctx, s.roomsSetKey(), snap.Room)
	pipe.Expire(ctx, s.roomsSetKey(), redisStateTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// GetRoomMembersBatch 批量读取多个房间的成员快照（Redis MGet）。
func (s *RedisStateStore) GetRoomMembersBatch(ctx context.Context, rooms []string) (map[string]RoomMembersSnapshot, error) {
	out := make(map[string]RoomMembersSnapshot, len(rooms))
	if len(rooms) == 0 {
		return out, nil
	}
	keys := make([]string, len(rooms))
	for i, room := range rooms {
		keys[i] = s.roomKey(room)
	}
	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	for i, val := range vals {
		if val == nil {
			continue
		}
		raw, ok := val.(string)
		if !ok {
			continue
		}
		var snap RoomMembersSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			continue
		}
		if snap.Members == nil {
			snap.Members = []MemberRecord{}
		}
		out[rooms[i]] = snap
	}
	return out, nil
}

func (s *RedisStateStore) GetRoomMembers(ctx context.Context, room string) (RoomMembersSnapshot, error) {
	val, err := s.rdb.Get(ctx, s.roomKey(room)).Bytes()
	if err != nil {
		return RoomMembersSnapshot{}, err
	}
	var snap RoomMembersSnapshot
	if err := json.Unmarshal(val, &snap); err != nil {
		return RoomMembersSnapshot{}, err
	}
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	return snap, nil
}

func (s *RedisStateStore) DeleteRoomMembers(ctx context.Context, room string) error {
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, s.roomKey(room))
	pipe.SRem(ctx, s.roomsSetKey(), room)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStateStore) PutStream(ctx context.Context, stream, room, identity string) error {
	b, err := json.Marshal(map[string]string{
		"room":     room,
		"identity": identity,
	})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.streamKey(stream), b, redisStateTTL).Err()
}

func (s *RedisStateStore) GetStream(ctx context.Context, stream string) (room, identity string, err error) {
	val, err := s.rdb.Get(ctx, s.streamKey(stream)).Bytes()
	if err != nil {
		return "", "", err
	}
	var m map[string]string
	if err := json.Unmarshal(val, &m); err != nil {
		return "", "", err
	}
	return m["room"], m["identity"], nil
}

func (s *RedisStateStore) DeleteStream(ctx context.Context, stream string) error {
	return s.rdb.Del(ctx, s.streamKey(stream)).Err()
}

func (s *RedisStateStore) roomMetaKey(room string) string {
	return s.prefix + ":membership:roommeta:" + sanitizeKey(room)
}

func (s *RedisStateStore) PutRoomMeta(ctx context.Context, room string, meta RoomMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.roomMetaKey(room), b, redisStateTTL)
	pipe.SAdd(ctx, s.roomsSetKey(), room)
	pipe.Expire(ctx, s.roomsSetKey(), redisStateTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStateStore) GetRoomMeta(ctx context.Context, room string) (RoomMeta, error) {
	val, err := s.rdb.Get(ctx, s.roomMetaKey(room)).Bytes()
	if err != nil {
		return RoomMeta{}, err
	}
	var meta RoomMeta
	if err := json.Unmarshal(val, &meta); err != nil {
		return RoomMeta{}, err
	}
	return meta, nil
}

func (s *RedisStateStore) DeleteRoomMeta(ctx context.Context, room string) error {
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, s.roomMetaKey(room))
	pipe.SRem(ctx, s.roomsSetKey(), room)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStateStore) ListRoomNames(ctx context.Context) ([]string, error) {
	names, err := s.rdb.SMembers(ctx, s.roomsSetKey()).Result()
	if err != nil {
		return nil, err
	}
	// filter empty
	out := make([]string, 0, len(names))
	for _, n := range names {
		if strings.TrimSpace(n) != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

// Close does not close the shared redis client.
func (s *RedisStateStore) Close() error { return nil }
