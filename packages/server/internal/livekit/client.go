package livekit

import (
	"context"
	"go_rtc/internal/config"
	"go_rtc/internal/pkg"
	"time"

	"github.com/livekit/protocol/auth"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/livekit/protocol/livekit"
)

type Service struct {
	host      string
	apiKey    string
	apiSecret string
	client    *lksdk.RoomServiceClient
}

func NewService(cfg *config.Config) *Service {
	s := &Service{
		host:      cfg.LiveKitHost,
		apiKey:    cfg.LiveKitKey,
		apiSecret: cfg.LiveKitSecret,
	}
	if cfg.LiveKitHost != "" {
		s.client = lksdk.NewRoomServiceClient(cfg.LiveKitHost, cfg.LiveKitKey, cfg.LiveKitSecret)
	}
	return s
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
		return "", pkg.NewAppError(pkg.SFU_ERROR, err.Error())
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
		return "", pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return token, nil
}

func (s *Service) ListRooms() (interface{}, error) {
	if s.client == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	resp, err := s.client.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return resp.Rooms, nil
}

func (s *Service) ListParticipants(room string) (interface{}, error) {
	if s.client == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	resp, err := s.client.ListParticipants(context.Background(), &livekit.ListParticipantsRequest{
		Room: room,
	})
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return resp.Participants, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	if s.client == nil {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	_, err := s.client.MutePublishedTrack(context.Background(), &livekit.MuteRoomTrackRequest{
		Room:     room,
		Identity: identity,
		TrackSid: trackSid,
		Muted:    muted,
	})
	if err != nil {
		return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return nil
}

func (s *Service) RemoveParticipant(room, identity string) error {
	if s.client == nil {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	_, err := s.client.RemoveParticipant(context.Background(), &livekit.RoomParticipantIdentity{
		Room:     room,
		Identity: identity,
	})
	if err != nil {
		return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	if s.client == nil {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	_, err := s.client.DeleteRoom(context.Background(), &livekit.DeleteRoomRequest{
		Room: room,
	})
	if err != nil {
		return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return nil
}