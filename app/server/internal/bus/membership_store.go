package bus

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/nats-io/nats.go"
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
	Backend() string // "nats" | "none"
	Close() error
}

// Ensure implementations satisfy the interface.
var _ MembershipStore = (*StateStore)(nil)

// ResolveMembershipConfig selects membership backend.
// Mode: auto|nats|none (default auto).
// auto preference: nats → none.
type ResolveMembershipConfig struct {
	Mode   string
	Prefix string

	// NATS connection for JetStream KV; nil skips nats backend.
	NATS *nats.Conn
}

// ResolveMembershipStore opens membership store by mode with degradation.
// Returns (nil, "none", nil) when no backend is available or mode=none.
// Forced mode (nats) returns error if that backend cannot be opened.
func ResolveMembershipStore(cfg ResolveMembershipConfig) (MembershipStore, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "auto"
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "gospeak"
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
	case "nats":
		st, err := tryNATS()
		if err != nil {
			return nil, "none", fmt.Errorf("state store nats: %w", err)
		}
		return st, "nats", nil
	case "auto":
		if st, err := tryNATS(); err == nil {
			return st, "nats", nil
		} else {
			log.Printf("[StateStore] nats unavailable: %v; fallback none", err)
		}
		return nil, "none", nil
	default:
		return nil, "none", fmt.Errorf("state store: unknown mode %q (want auto|nats|none)", mode)
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
	return false
}

// ErrMembershipConflict 表示共享状态 CAS 版本不匹配，调用方应重试合并。
var ErrMembershipConflict = errors.New("membership version mismatch")
