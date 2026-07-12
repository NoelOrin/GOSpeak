package srs

import (
	"fmt"
	"strings"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type Service struct {
	client     *Client
	secret     string
	host       string
	publicHost string
	whipURL    string
	registry   pkg.RoomRegistry
}

func (s *Service) SetRoomRegistry(r pkg.RoomRegistry) {
	s.registry = r
}

func NewService(cfg *config.Config) *Service {
	host := strings.TrimSpace(cfg.SRSHost)
	apiPort := strings.TrimSpace(cfg.SRSApiPort)
	if host == "" {
		host = "localhost"
	}
	if apiPort == "" {
		apiPort = "1985"
	}
	whipURL := strings.TrimSpace(cfg.SRSWHIPURL)
	if whipURL == "" {
		whipURL = "/rtc/v1/whip/"
	}
	baseURL := fmt.Sprintf("http://%s:%s", host, apiPort)
	return &Service{
		client:     NewClient(baseURL),
		secret:     cfg.SRSSecret,
		host:       baseURL,
		publicHost: strings.TrimSpace(cfg.SRSPublicHost),
		whipURL:    whipURL,
	}
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	return GenerateToken(room, identity, s.secret)
}

func (s *Service) GenerateAdminToken() (string, error) {
	return GenerateToken("__admin", "__admin", s.secret)
}

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.registry == nil {
		return nil, pkg.NewAppError(pkg.SFU_ERROR, "srs room registry not configured")
	}
	rooms := s.registry.Rooms()
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		streams := s.registry.Streams(name)
		out = append(out, sfu.RoomSummary{Name: name, MemberCount: len(streams)})
	}
	return out, nil
}

func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	var streams []string
	if s.registry != nil {
		streams = s.registry.Streams(room)
	}
	if len(streams) == 0 {
		return []sfu.ParticipantSummary{}, nil
	}
	participants, err := s.client.ListParticipantsByStreams(streams)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(participants))
	seen := make(map[string]struct{}, len(participants))
	for _, p := range participants {
		stream, _ := p["stream"].(string)
		identity := ""
		if s.registry != nil && stream != "" {
			if id, ok := s.registry.IdentityForStream(room, stream); ok {
				identity = id
			}
		}
		if identity == "" {
			if id, ok := p["id"].(string); ok {
				identity = id
			}
		}
		if identity == "" {
			continue
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, sfu.ParticipantSummary{Identity: identity})
	}
	return out, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}

func (s *Service) RemoveParticipant(room, identity string) error {
	stream := ""
	if s.registry != nil {
		if st, ok := s.registry.StreamForIdentity(room, identity); ok {
			stream = st
		}
	}
	if stream == "" {
		stream = GenerateStreamName(room, identity)
	}
	kicked, remaining, err := s.client.KickByStreams([]string{stream})
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	if kicked == 0 && remaining == 0 {
		return pkg.NewAppError(pkg.NOT_FOUND, "srs participant not found")
	}
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	if s.registry != nil {
		streams := s.registry.Streams(room)
		kicked, remaining, err := s.client.KickByStreams(streams)
		if err != nil {
			return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
		}
		if kicked == 0 && remaining == 0 && len(streams) == 0 {
			return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
		}
		s.registry.ClearRoom(room)
		return nil
	}
	if err := s.client.DeleteRoom(room); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) GetHost() string {
	if s.publicHost != "" {
		return s.publicHost
	}
	return s.host
}

func (s *Service) ProviderName() string {
	return "srs"
}

func (s *Service) StreamName(room, identity string) string {
	return GenerateStreamName(room, identity)
}

func (s *Service) StreamInfo(room, identity string) (stream, token string, err error) {
	stream = GenerateStreamName(room, identity)
	t, err := GenerateStreamToken(stream, s.secret)
	if err != nil {
		return stream, "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "generate stream token: "+err.Error())
	}
	return stream, t, nil
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"whipUrl": s.whipURL,
	}
}
