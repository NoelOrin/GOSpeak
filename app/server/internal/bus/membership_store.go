package bus

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
)

// MembershipStore is the shared room membership / stream map backend.
// Used by signal.Hub for cross-instance room views.
type MembershipStore interface {
	PutRoomMembers(ctx context.Context, snap RoomMembersSnapshot) error
	GetRoomMembers(ctx context.Context, room string) (RoomMembersSnapshot, error)
	DeleteRoomMembers(ctx context.Context, room string) error
	PutStream(ctx context.Context, stream, room, identity string) error
	GetStream(ctx context.Context, stream string) (room, identity string, err error)
	DeleteStream(ctx context.Context, stream string) error
	ListRoomNames(ctx context.Context) ([]string, error)
	Backend() string // "redis" | "nats"
	Close() error
}

// Ensure implementations satisfy the interface.
var (
	_ MembershipStore = (*StateStore)(nil)
	_ MembershipStore = (*RedisStateStore)(nil)
)

// ResolveMembershipConfig selects membership backend.
// Mode: auto|redis|nats|none (default auto).
// auto preference: redis → nats → none.
type ResolveMembershipConfig struct {
	Mode   string
	Prefix string

	// Redis client; nil/disabled skips redis backend.
	Redis *goredis.Client

	// NATS connection for JetStream KV; nil skips nats backend.
	NATS *nats.Conn
}

// ResolveMembershipStore opens membership store by mode with degradation.
// Returns (nil, "none", nil) when no backend is available or mode=none.
// Forced mode (redis/nats) returns error if that backend cannot be opened.
func ResolveMembershipStore(cfg ResolveMembershipConfig) (MembershipStore, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "auto"
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "gospeak"
	}

	tryRedis := func() (MembershipStore, error) {
		if cfg.Redis == nil {
			return nil, fmt.Errorf("redis client not connected")
		}
		return OpenRedisStateStore(RedisStateStoreConfig{
			Client: cfg.Redis,
			Prefix: prefix,
		})
	}
	tryNATS := func() (MembershipStore, error) {
		if cfg.NATS == nil {
			return nil, fmt.Errorf("nats connection not available")
		}
		return OpenStateStore(StateStoreConfig{
			Prefix: prefix,
			NC:     cfg.NATS,
		})
	}

	switch mode {
	case "none":
		return nil, "none", nil
	case "redis":
		st, err := tryRedis()
		if err != nil {
			return nil, "none", fmt.Errorf("state store redis: %w", err)
		}
		return st, "redis", nil
	case "nats":
		st, err := tryNATS()
		if err != nil {
			return nil, "none", fmt.Errorf("state store nats: %w", err)
		}
		return st, "nats", nil
	case "auto":
		if st, err := tryRedis(); err == nil {
			return st, "redis", nil
		} else {
			log.Printf("[StateStore] redis unavailable: %v; try nats", err)
		}
		if st, err := tryNATS(); err == nil {
			return st, "nats", nil
		} else {
			log.Printf("[StateStore] nats unavailable: %v; fallback none", err)
		}
		return nil, "none", nil
	default:
		return nil, "none", fmt.Errorf("state store: unknown mode %q (want auto|redis|nats|none)", mode)
	}
}

// IsMembershipNotFound 判断共享状态读取错误是否为“记录不存在”。
// 调用方只有在记录不存在时才允许继续合并/创建；其他错误必须中止，避免覆盖远程状态。
func IsMembershipNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, nats.ErrKeyNotFound) || errors.Is(err, nats.ErrKeyDeleted) {
		return true
	}
	return errors.Is(err, goredis.Nil)
}
