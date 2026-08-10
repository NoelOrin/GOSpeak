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
	ttl    time.Duration
}

// RedisStateStoreConfig opens a Redis-backed membership store.
type RedisStateStoreConfig struct {
	Client *goredis.Client
	Prefix string
	// TTL 覆盖成员/流/房间元数据键的默认过期时间；零值使用 redisStateTTL（24h）。
	TTL time.Duration
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
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = redisStateTTL
	}
	return &RedisStateStore{rdb: cfg.Client, prefix: cfg.Prefix, ttl: ttl}, nil
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

// redisMembershipEnvelope 是 Redis 端带版本号的成员快照包装，
// 供多实例并发合并时做 CAS 校验；旧格式纯快照仍可读取。
type redisMembershipEnvelope struct {
	Version uint64                `json:"version"`
	Snap    RoomMembersSnapshot `json:"snap"`
}

const redisMembershipCASScript = `
local raw = redis.call('GET', KEYS[1])
local current = 0
if raw then
  local ok, parsed = pcall(cjson.decode, raw)
  if ok and parsed and parsed['version'] then
    current = tonumber(parsed['version'])
  end
end
if current ~= tonumber(ARGV[1]) then
  return redis.error_reply('ERR membership version mismatch')
end
redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
redis.call('SADD', KEYS[2], ARGV[4])
redis.call('EXPIRE', KEYS[2], ARGV[3])
return current + 1
`

const redisMembershipDeleteCASScript = `
local raw = redis.call('GET', KEYS[1])
local current = 0
if raw then
  local ok, parsed = pcall(cjson.decode, raw)
  if ok and parsed and parsed['version'] then
    current = tonumber(parsed['version'])
  end
end
if current ~= tonumber(ARGV[1]) then
  return redis.error_reply('ERR membership version mismatch')
end
redis.call('DEL', KEYS[1])
redis.call('SREM', KEYS[2], ARGV[2])
return 1
`

// parseRedisMembership 兼容旧版纯快照格式与新版本包装格式。
func parseRedisMembership(raw []byte) (RoomMembersSnapshot, uint64, error) {
	var env redisMembershipEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && (env.Version != 0 || env.Snap.Room != "" || len(env.Snap.Members) > 0) {
		if env.Snap.Members == nil {
			env.Snap.Members = []MemberRecord{}
		}
		return env.Snap, env.Version, nil
	}
	var snap RoomMembersSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return RoomMembersSnapshot{}, 0, err
	}
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	return snap, 0, nil
}

func (s *RedisStateStore) PutRoomMembers(ctx context.Context, snap RoomMembersSnapshot) error {
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	b, err := json.Marshal(redisMembershipEnvelope{Version: 1, Snap: snap})
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.roomKey(snap.Room), b, s.ttl)
	pipe.SAdd(ctx, s.roomsSetKey(), snap.Room)
	pipe.Expire(ctx, s.roomsSetKey(), s.ttl)
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
		snap, _, err := parseRedisMembership([]byte(raw))
		if err != nil {
			continue
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
	snap, _, err := parseRedisMembership(val)
	if err != nil {
		return RoomMembersSnapshot{}, err
	}
	return snap, nil
}

// GetRoomMembersRev 返回成员快照与当前版本号，供 Hub 做乐观锁合并。
func (s *RedisStateStore) GetRoomMembersRev(ctx context.Context, room string) (RoomMembersSnapshot, uint64, error) {
	val, err := s.rdb.Get(ctx, s.roomKey(room)).Bytes()
	if err != nil {
		return RoomMembersSnapshot{}, 0, err
	}
	snap, rev, err := parseRedisMembership(val)
	if err != nil {
		return RoomMembersSnapshot{}, 0, err
	}
	return snap, rev, nil
}

// PutRoomMembersRev 用 Lua 原子 CAS 写入：只有当前版本等于期望版本时才落盘。
func (s *RedisStateStore) PutRoomMembersRev(ctx context.Context, snap RoomMembersSnapshot, rev uint64) error {
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	next := rev + 1
	b, err := json.Marshal(redisMembershipEnvelope{Version: next, Snap: snap})
	if err != nil {
		return err
	}
	_, err = s.rdb.Eval(ctx, redisMembershipCASScript,
		[]string{s.roomKey(snap.Room), s.roomsSetKey()},
		rev, string(b), int64(s.ttl.Seconds()), snap.Room).Result()
	if err != nil {
		if strings.Contains(err.Error(), "membership version mismatch") {
			return ErrMembershipConflict
		}
		return err
	}
	return nil
}

// DeleteRoomMembersRev 仅在版本匹配时删除成员快照。
func (s *RedisStateStore) DeleteRoomMembersRev(ctx context.Context, room string, rev uint64) error {
	if rev == 0 {
		return nil
	}
	err := s.rdb.Eval(ctx, redisMembershipDeleteCASScript,
		[]string{s.roomKey(room), s.roomsSetKey()}, rev, room).Err()
	if err != nil {
		if strings.Contains(err.Error(), "membership version mismatch") {
			return ErrMembershipConflict
		}
		return err
	}
	return nil
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
	return s.rdb.Set(ctx, s.streamKey(stream), b, s.ttl).Err()
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
	pipe.Set(ctx, s.roomMetaKey(room), b, s.ttl)
	pipe.SAdd(ctx, s.roomsSetKey(), room)
	pipe.Expire(ctx, s.roomsSetKey(), s.ttl)
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
