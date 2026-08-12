package mediasoup

import (
	"errors"
	"fmt"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

var _ sfu.Provider = (*Service)(nil)
var _ sfu.ClientInfoProvider = (*Service)(nil)

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

// Close 释放 bridge HTTP 空闲连接。
func (s *Service) Close() error {
	if s.Bridge != nil && s.Bridge.client != nil {
		s.Bridge.client.CloseIdleConnections()
	}
	return nil
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	return fmt.Sprintf("%s:%s", room, identity), nil
}

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	rooms, err := s.Bridge.ListRouters()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		out = append(out, sfu.RoomSummary{Name: name})
	}
	return out, nil
}

func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	participants, err := s.partBridge.ListParticipants(room)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(participants))
	for _, p := range participants {
		out = append(out, sfu.ParticipantSummary{Identity: p.Identity})
	}
	return out, nil
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

func (s *Service) RemoveParticipant(room, identity string) error {
	if _, err := s.partBridge.CloseParticipant(room, identity); err != nil {
		if errors.Is(err, ErrParticipantNotFound) {
			return nil
		}
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

func (s *Service) Capabilities() sfu.Capabilities {
	return sfu.CapabilitiesFor("mediasoup")
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"bridgeUrl": s.host,
	}
}
