package sfu

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

// DefaultProviderName 是唯一内置 provider，也是 env 未指定时的回退值。
// 字面量单点在 config.DefaultSFUProvider；新增 provider 时在此包登记能力表
// 并在 factory 注册构造器。
const DefaultProviderName = config.DefaultSFUProvider

// Provider abstracts an SFU backend (currently LiveKit).
//
// Capability contract:
//   - Supported operations return concrete results / nil error.
//   - Unsupported operations MUST return pkg.NewErrSFUNotSupported()
//     so callers can degrade with errors.Is(err, pkg.ErrSFUNotSupported).
//   - Missing configuration should return SFU_NOT_CONFIGURED, not "not supported".
//   - Capabilities() is declarative and must match the real method behavior.
//   - 能力方法（WithPublish/Timed/Stream/ClientInfo）统一收敛在主接口，
//     调用方直接调用并按 ErrSFUNotSupported 降级，禁止散落类型断言探测。
type Provider interface {
	// ProviderName returns the stable provider id (e.g. livekit).
	ProviderName() string
	// Capabilities returns media-layer hard-enforcement support for this provider.
	Capabilities() Capabilities
	GenerateToken(room, identity string) (string, error)
	// GenerateTokenWithPublish 显式声明发布权限签发 token（访客说权限等场景）。
	// 不支持发布控制的实现返回 ErrSFUNotSupported，调用方降级 GenerateToken。
	GenerateTokenWithPublish(room, identity string, canPublish bool) (string, error)
	ListRooms() ([]RoomSummary, error)
	ListParticipants(room string) ([]ParticipantSummary, error)
	MuteParticipant(room, identity, trackSid string, muted bool) error
	// MuteParticipantTimed 带 TTL 的定时禁言；不支持定时语义的实现返回
	// ErrSFUNotSupported，调用方降级 MuteParticipant。
	MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error
	RemoveParticipant(room, identity string) error
	DeleteRoom(room string) error
	GetHost() string
	// SupportsStream 是否支持流寻址（WHIP/WHEP）。
	SupportsStream() bool
	// StreamName/StreamInfo 仅在 SupportsStream 时有意义；未支持实现返回空值。
	StreamName(room, identity string) string
	StreamInfo(room, identity string) (stream, token string, err error)
	// ClientInfo 返回 provider 特定连接元数据；未支持实现返回空 map。
	ClientInfo() map[string]interface{}
}

// BaseProvider 提供全部能力方法的"不支持"零实现，供测试桩与最小实现嵌入，
// 只需覆写自己真正支持的方法。
type BaseProvider struct{}

func (BaseProvider) GenerateTokenWithPublish(room, identity string, canPublish bool) (string, error) {
	return "", pkg.NewErrSFUNotSupported()
}

func (BaseProvider) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	return pkg.NewErrSFUNotSupported()
}

func (BaseProvider) SupportsStream() bool { return false }

func (BaseProvider) StreamName(room, identity string) string { return "" }

func (BaseProvider) StreamInfo(room, identity string) (string, string, error) {
	return "", "", pkg.NewErrSFUNotSupported()
}

func (BaseProvider) ClientInfo() map[string]interface{} { return map[string]interface{}{} }
