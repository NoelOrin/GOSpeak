package bus

import (
	"context"
	"errors"
	"fmt"
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
// Mode: auto|nats (default auto). A missing NATS connection is a fatal error:
// the process always runs with a NATS connection (embedded or external,
// guaranteed by bus.Init), so there is no none / in-memory degradation.
type ResolveMembershipConfig struct {
	Mode   string
	Prefix string

	// NATS connection for JetStream KV; must be non-nil (provided by bus.Init).
	NATS *nats.Conn
}

// ResolveMembershipStore opens the membership/stream store backed by NATS KV.
// It fails fast when NATS is unavailable instead of degrading to a local-only
// "none" backend, so cross-instance room views always have shared state.
//
// 注意（风险 7）：自 v5138 起强制依赖 NATS（embedded 或 external），已移除
// 旧的 STATE_STORE=none 内存降级路径。单测/离线单进程若不提供 NATS 连接会直接 fail fast；
// 嵌入式 NATS 端口被占用也会导致整个服务启动失败，需在部署文档与单测 bootstrap 中
// 显式标注并预留端口或改用外部 NATS。
func ResolveMembershipStore(cfg ResolveMembershipConfig) (MembershipStore, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "auto"
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "gospeak"
	}
	if mode != "auto" && mode != "nats" {
		return nil, "", fmt.Errorf("state store: unsupported mode %q (want auto|nats)", mode)
	}
	if cfg.NATS == nil {
		return nil, "", fmt.Errorf("state store: nats connection required (embedded or external URL)")
	}
	st, err := OpenStateStore(StateStoreConfig{
		Prefix: prefix,
		NC:     cfg.NATS,
	})
	if err != nil {
		return nil, "", fmt.Errorf("state store nats: %w", err)
	}
	return st, "nats", nil
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
