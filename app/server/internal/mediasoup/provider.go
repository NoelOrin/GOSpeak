package mediasoup

import (
	"fmt"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type Service struct {
	Bridge *BridgeClient
	host   string
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		Bridge: NewBridgeClient(cfg.MediaSoupBridgeURL),
		host:   cfg.MediaSoupHost,
	}
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	if err := s.Bridge.CreateRouter(room); err != nil {
		return "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return fmt.Sprintf("%s:%s", room, identity), nil
}

func (s *Service) GenerateAdminToken() (string, error) {
	return "mediasoup-admin", nil
}

func (s *Service) ListRooms() (interface{}, error) {
	rooms, err := s.Bridge.ListRouters()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return rooms, nil
}

func (s *Service) ListParticipants(room string) (interface{}, error) {
	return nil, pkg.NewErrSFUNotSupported()
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}

func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}

func (s *Service) RemoveParticipant(room, identity string) error {
	return pkg.NewErrSFUNotSupported()
}

func (s *Service) DeleteRoom(room string) error {
	if err := s.Bridge.DeleteRouter(room); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) GetHost() string {
	return s.host
}

func (s *Service) ProviderName() string {
	return "mediasoup"
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"bridgeUrl": s.host,
	}
}
