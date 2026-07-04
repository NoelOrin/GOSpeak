package agora

import (
	"errors"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type Service struct {
	appID          string
	appCertificate string
	host           string
	customerID     string
	customerSecret string
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		appID:          cfg.AgoraAppID,
		appCertificate: cfg.AgoraAppCertificate,
		host:           cfg.AgoraHost,
		customerID:     cfg.AgoraCustomerID,
		customerSecret: cfg.AgoraCustomerSecret,
	}
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	if s.appID == "" || s.appCertificate == "" {
		return "", pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "AGORA_APP_ID and AGORA_APP_CERTIFICATE are required")
	}
	token, err := buildRTCToken(s.appID, s.appCertificate, room, identity)
	if err != nil {
		return "", pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return token, nil
}

func (s *Service) GenerateAdminToken() (string, error) {
	return "", nil
}

func (s *Service) ListRooms() (interface{}, error) {
	rooms, err := s.restClient().ListRooms()
	if err != nil {
		return nil, s.mapRESTError(err)
	}
	return rooms, nil
}

func (s *Service) ListParticipants(room string) (interface{}, error) {
	users, err := s.restClient().GetChannelUsers(room)
	if err != nil {
		return nil, s.mapRESTError(err)
	}
	return users, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return nil
}

func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	return nil
}

// RemoveParticipant Agora 服务端无单用户踢出 API：kicking-rule 是 ban 语义
// （阻止再次加入），而非断开当前会话，与"踢出"语义不符。返回 pkg.ErrSFUNotSupported
// 让 Hub 优雅降级，依赖 Agora 频道在用户主动离开后的自动回收。
func (s *Service) RemoveParticipant(room, identity string) error {
	return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, pkg.ErrSFUNotSupported, "agora: server-side participant removal not supported")
}

func (s *Service) DeleteRoom(room string) error {
	if err := s.restClient().DeleteChannel(room); err != nil {
		return s.mapRESTError(err)
	}
	return nil
}

func (s *Service) GetHost() string {
	if s.host != "" {
		return s.host
	}
	return "https://api.agora.io"
}

func (s *Service) ProviderName() string {
	return "agora"
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"appId": s.appID,
	}
}

func (s *Service) restClient() *RESTClient {
	return NewRESTClient(s.appID, s.customerID, s.customerSecret)
}

func (s *Service) mapRESTError(err error) error {
	if errors.Is(err, ErrRESTCredentialsMissing) {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, ErrRESTCredentialsMissing.Error())
	}
	return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
}
