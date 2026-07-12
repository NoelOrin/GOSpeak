package service

import (
	"fmt"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

// JoinTokenResult 是 GetJoinToken 的聚合结果，handler 直接映射为 JSON 响应。
type JoinTokenResult struct {
	Token       string
	ServerURL   string
	Provider    string
	ClientInfo  map[string]interface{}
	Stream      string
	StreamToken string
}

// SFUService 是 SFU 调用 + 加入规则校验的 service 中间层。
// handler 只调本 service，不再直持 sfu.Provider 或 signal.Hub，
// 满足 handler→service→repository/SFU 分层契约。
type SFUService struct {
	provider sfu.Provider
	policy   pkg.JoinPolicy
}

func NewSFUService(provider sfu.Provider, policy pkg.JoinPolicy) *SFUService {
	return &SFUService{provider: provider, policy: policy}
}

// GetJoinToken 编排加入规则校验（禁言/限流/密码）+ token 签发 + stream 信息。
// 规则失败返 *pkg.AppError，handler 经 HandleError 映射状态码。
func (s *SFUService) GetJoinToken(room, identity, password string) (*JoinTokenResult, error) {
	if s.policy != nil {
		if muted, _ := s.policy.IsMuted(identity); muted {
			return nil, pkg.NewAppError(pkg.USER_MUTED, "user is muted")
		}
		if full, limit, count, _ := s.policy.CheckRoomLimit(room); full {
			return nil, pkg.NewAppError(pkg.FORBIDDEN, fmt.Sprintf("room is full (%d/%d)", count, limit))
		}
		if ok, err := s.policy.CheckRoomPassword(room, password); !ok {
			if err != nil {
				return nil, pkg.NewAppError(pkg.FORBIDDEN, "room requires password")
			}
			return nil, pkg.NewAppError(pkg.FORBIDDEN, "wrong room password")
		}
	}

	token, err := s.provider.GenerateToken(room, identity)
	if err != nil {
		return nil, err
	}
	res := &JoinTokenResult{
		Token:     token,
		ServerURL: s.provider.GetHost(),
	}
	if p, ok := s.provider.(interface{ ProviderName() string }); ok {
		res.Provider = p.ProviderName()
	}
	if p, ok := s.provider.(sfu.ClientInfoProvider); ok {
		res.ClientInfo = p.ClientInfo()
	}
	if p, ok := s.provider.(sfu.StreamProvider); ok {
		st, stok, err := p.StreamInfo(room, identity)
		if err != nil {
			return nil, err
		}
		res.Stream = st
		res.StreamToken = stok
	}
	return res, nil
}

func (s *SFUService) ListRooms() ([]sfu.RoomSummary, error) {
	return s.provider.ListRooms()
}

func (s *SFUService) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	return s.provider.ListParticipants(room)
}
