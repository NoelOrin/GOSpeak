package mediasoup

import (
	"fmt"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type providerBridge interface {
	ListParticipants(roomID string) ([]ParticipantInfo, error)
	CloseParticipant(roomID, identity string) ([]string, error)
	PauseProducer(roomID, producerID string) error
	ResumeProducer(roomID, producerID string) error
	PauseParticipant(roomID, identity string) error
	ResumeParticipant(roomID, identity string) error
}

type Service struct {
	Bridge     *BridgeClient
	partBridge providerBridge
	host       string
}

func NewService(cfg *config.Config) *Service {
	b := NewBridgeClient(cfg.MediaSoupBridgeURL)
	return &Service{
		Bridge:     b,
		partBridge: b,
		host:       cfg.MediaSoupHost,
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
	participants, err := s.partBridge.ListParticipants(room)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return participants, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	var err error
	if trackSid != "" {
		if muted {
			err = s.partBridge.PauseProducer(room, trackSid)
		} else {
			err = s.partBridge.ResumeProducer(room, trackSid)
		}
	} else {
		if muted {
			err = s.partBridge.PauseParticipant(room, identity)
		} else {
			err = s.partBridge.ResumeParticipant(room, identity)
		}
	}
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	var err error
	if muted {
		err = s.partBridge.PauseParticipant(room, identity)
	} else {
		err = s.partBridge.ResumeParticipant(room, identity)
	}
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) RemoveParticipant(room, identity string) error {
	if _, err := s.partBridge.CloseParticipant(room, identity); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
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
