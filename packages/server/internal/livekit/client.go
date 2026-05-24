package livekit

import (
	"go_rtc/internal/pkg"
	"os"
	"time"

	"github.com/livekit/protocol/auth"
)

type Service struct {
	host      string
	apiKey    string
	apiSecret string
}

func NewService() *Service {
	return &Service{
		host:      os.Getenv("LIVEKIT_HOST"),
		apiKey:    os.Getenv("LIVEKIT_KEY"),
		apiSecret: os.Getenv("LIVEKIT_SECRET"),
	}
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	at := auth.NewAccessToken(s.apiKey, s.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	at.AddGrant(grant).
		SetIdentity(identity).
		SetValidFor(time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		return "", pkg.NewAppError(pkg.LIVEKIT_ERROR, err.Error())
	}
	return token, nil
}

func (s *Service) GenerateAdminToken() (string, error) {
	at := auth.NewAccessToken(s.apiKey, s.apiSecret)
	grant := &auth.VideoGrant{
		RoomCreate: true,
		RoomList:   true,
	}
	at.AddGrant(grant).
		SetIdentity("admin").
		SetValidFor(time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		return "", pkg.NewAppError(pkg.LIVEKIT_ERROR, err.Error())
	}
	return token, nil
}

func (s *Service) ListRooms() (interface{}, error) {
	if s.host == "" {
		return nil, pkg.NewAppError(pkg.LIVEKIT_NOT_CONFIGURED)
	}
	return nil, nil
}

func (s *Service) ListParticipants(room string) (interface{}, error) {
	if s.host == "" {
		return nil, pkg.NewAppError(pkg.LIVEKIT_NOT_CONFIGURED)
	}
	_ = room
	return nil, nil
}

func (s *Service) MuteParticipant(room, identity string) error {
	_ = room
	_ = identity
	return nil
}

func (s *Service) RemoveParticipant(room, identity string) error {
	_ = room
	_ = identity
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	_ = room
	return nil
}