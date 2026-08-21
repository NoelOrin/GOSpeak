package livekit

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
	"context"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
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

// Close 释放 provider 资源；LiveKit client 当前无显式连接句柄。
func (s *Service) Close() error { return nil }

// ctxWithTimeout returns a context with a bounded timeout for LiveKit API calls.
func ctxWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
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

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.client == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.client.ListRooms(ctx, &livekit.ListRoomsRequest{})
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	out := make([]sfu.RoomSummary, 0, len(resp.Rooms))
	for _, r := range resp.Rooms {
		out = append(out, sfu.RoomSummary{
			Name:        r.Name,
			MemberCount: int(r.NumParticipants),
		})
	}
	return out, nil
}

func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if s.client == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	ctx, cancel := ctxWithTimeout()
	defer cancel()
	resp, err := s.client.ListParticipants(ctx, &livekit.ListParticipantsRequest{
		Room: room,
	})
	if err != nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(resp.Participants))
	for _, p := range resp.Participants {
		out = append(out, sfu.ParticipantSummary{
			Identity: p.Identity,
			JoinedAt: p.JoinedAt,
		})
	}
	return out, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	if s.client == nil {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	if trackSid != "" {
		ctx, cancel := ctxWithTimeout()
		defer cancel()
		_, err := s.client.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
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
	ctx, cancel := ctxWithTimeout()
	defer cancel()
	resp, err := s.client.ListParticipants(ctx, &livekit.ListParticipantsRequest{
		Room: room,
	})
	if err != nil {
		return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	found := false
	for _, p := range resp.Participants {
		if p.Identity != identity {
			continue
		}
		found = true
		for _, track := range p.Tracks {
			if _, err := s.client.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
				Room:     room,
				Identity: identity,
				TrackSid: track.Sid,
				Muted:    muted,
			}); err != nil {
				return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
			}
		}
		break
	}
	if !found {
		return pkg.NewAppError(pkg.SFU_ERROR, "livekit participant not found")
	}
	return nil
}

func (s *Service) RemoveParticipant(room, identity string) error {
	if s.client == nil {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED)
	}
	ctx, cancel := ctxWithTimeout()
	defer cancel()
	_, err := s.client.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{
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
	ctx, cancel := ctxWithTimeout()
	defer cancel()
	_, err := s.client.DeleteRoom(ctx, &livekit.DeleteRoomRequest{
		Room: room,
	})
	if err != nil {
		return pkg.NewAppError(pkg.SFU_ERROR, err.Error())
	}
	return nil
}

func (s *Service) GetHost() string {
	return s.host
}

// ProviderName 返回 SFU provider 标识，用于 hub 的 provider 特性判断（如 kick 踢人）。
func (s *Service) ProviderName() string {
	return "livekit"
}

func (s *Service) Capabilities() sfu.Capabilities {
	return sfu.CapabilitiesFor("livekit")
}
