package service

import (
	"fmt"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

// ownerAwareProvider is implemented by SFU providers that bind the joining
// user UUID to the session created by GenerateToken.
type ownerAwareProvider interface {
	GenerateTokenForUser(room, identity, ownerUUID string) (string, error)
}

// JoinTokenResult 是 GetJoinToken 的聚合结果，handler 直接映射为 JSON 响应。
type JoinTokenResult struct {
	Token        string
	ServerURL    string
	Provider     string
	ClientInfo   map[string]interface{}
	Stream       string
	StreamToken  string
	Capabilities sfu.Capabilities
	SFURoom      string
}

// SFUService 是 SFU 调用 + 加入规则校验的 service 中间层。
// handler 只调本 service，不再直持 sfu.Provider 或 signal.Hub，
// 满足 handler→service→repository/SFU 分层契约。
type SFUService struct {
	provider     sfu.Provider
	policy       pkg.JoinPolicy
	domainMember func(domainUUID, userUUID string) bool
}

func NewSFUService(provider sfu.Provider, policy pkg.JoinPolicy) *SFUService {
	return &SFUService{provider: provider, policy: policy}
}

// SetDomainMemberChecker 注入 Domain 成员校验函数。
func (s *SFUService) SetDomainMemberChecker(checker func(domainUUID, userUUID string) bool) {
	s.domainMember = checker
}

// GetJoinToken 编排加入规则校验（禁言/限流/密码）+ token 签发 + stream 信息。
// 规则失败返 *pkg.AppError，handler 经 HandleError 映射状态码。
func (s *SFUService) GetJoinToken(domainUUID, room, identity, userUUID, password string) (*JoinTokenResult, error) {
	if domainUUID != "" && (s.domainMember == nil || !s.domainMember(domainUUID, userUUID)) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a member of this domain")
	}
	if s.policy != nil {
		if muted, err := s.policy.IsMuted(identity); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "mute check failed")
		} else if muted {
			return nil, pkg.NewAppError(pkg.USER_MUTED, "user is muted")
		}
		if full, limit, count, err := s.policy.CheckRoomLimit(domainUUID, room); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "room limit check failed")
		} else if full {
			return nil, pkg.NewAppError(pkg.FORBIDDEN, fmt.Sprintf("room is full (%d/%d)", count, limit))
		}
		if ok, err := s.policy.CheckRoomPassword(domainUUID, room, password); !ok {
			if err != nil {
				return nil, pkg.NewAppError(pkg.FORBIDDEN, "room requires password")
			}
			return nil, pkg.NewAppError(pkg.FORBIDDEN, "wrong room password")
		}
	}

	sfuRoom := pkg.RoomKey(domainUUID, room)
	token, err := s.generateToken(sfuRoom, identity, userUUID)
	if err != nil {
		return nil, err
	}
	res := &JoinTokenResult{
		Token:        token,
		ServerURL:    s.provider.GetHost(),
		Provider:     s.provider.ProviderName(),
		Capabilities: s.provider.Capabilities(),
		SFURoom:      sfuRoom,
	}
	if p, ok := s.provider.(sfu.ClientInfoProvider); ok {
		res.ClientInfo = p.ClientInfo()
	}
	if p, ok := s.provider.(sfu.StreamProvider); ok {
		// 动态门面可能实现 StreamProvider 但当前后端不支持流寻址，须按能力门面跳过。
		if sc, ok2 := s.provider.(interface{ SupportsStream() bool }); ok2 && !sc.SupportsStream() {
			return res, nil
		}
		st, stok, err := p.StreamInfo(sfuRoom, identity)
		if err != nil {
			return nil, err
		}
		res.Stream = st
		res.StreamToken = stok
	}
	return res, nil
}

func (s *SFUService) generateToken(room, identity, userUUID string) (string, error) {
	if op, ok := s.provider.(ownerAwareProvider); ok {
		return op.GenerateTokenForUser(room, identity, userUUID)
	}
	return s.provider.GenerateToken(room, identity)
}

func (s *SFUService) ListRooms() ([]sfu.RoomSummary, error) {
	return s.provider.ListRooms()
}

func (s *SFUService) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	return s.provider.ListParticipants(room)
}

func (s *SFUService) Capabilities() sfu.Capabilities {
	return s.provider.Capabilities()
}
