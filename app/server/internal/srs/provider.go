package srs

import (
	"fmt"
	"strings"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type Service struct {
	client    *Client
	host      string
	apiPort   string
	whipPort  string
	secret    string
	whipURL   string
	serverURL string
	registry  pkg.RoomRegistry
}

func (s *Service) SetRoomRegistry(r pkg.RoomRegistry) {
	s.registry = r
}

func NewService(cfg *config.Config) *Service {
	host := strings.TrimSpace(cfg.SRSHost)
	apiPort := strings.TrimSpace(cfg.SRSApiPort)
	whipPort := strings.TrimSpace(cfg.SRSWHIPPort)
	if host == "" {
		host = "localhost"
	}
	if apiPort == "" {
		apiPort = "1985"
	}
	if whipPort == "" {
		whipPort = "1985"
	}
	baseURL := fmt.Sprintf("http://%s:%s", host, apiPort)
	serverURL := fmt.Sprintf("http://%s:%s", host, whipPort)
	return &Service{
		client:    NewClient(baseURL),
		host:      host,
		apiPort:   apiPort,
		whipPort:  whipPort,
		secret:    cfg.SRSSecret,
		whipURL:   "/rtc/v1/whip/",
		serverURL: serverURL,
	}
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	return GenerateToken(room, identity, s.secret)
}

func (s *Service) GenerateAdminToken() (string, error) {
	return GenerateToken("__admin", "__admin", s.secret)
}

func (s *Service) ListRooms() (interface{}, error) {
	// registry 注入后走 Hub room 聚合视图，避免把 N 个 stream 当 N 个假房间。
	if s.registry != nil {
		rooms := s.registry.Rooms()
		if rooms == nil {
			rooms = []string{}
		}
		return rooms, nil
	}
	// 降级：registry 未注入时回退 SRS /api/v1/streams（语义为 stream 名非 room）。
	rooms, err := s.client.ListRooms()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return rooms, nil
}

func (s *Service) ListParticipants(room string) (interface{}, error) {
	participants, err := s.client.ListParticipants(room)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return participants, nil
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}

func (s *Service) MuteRoomParticipant(room, identity string, muted bool) error {
	return pkg.NewErrSFUNotSupported()
}

func (s *Service) RemoveParticipant(room, identity string) error {
	// SRS client id 由 SRS 内部生成，无法从 identity 推导，必须 list-then-kick。
	// 用 stream 桥接：算 room+identity → stream → KickByStreams。
	stream := GenerateStreamName(room, identity)
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
	// registry 注入后：聚合 kick 该 room 下所有 stream 的 client，再清聚合视图。
	if s.registry != nil {
		streams := s.registry.Streams(room)
		kicked, remaining, err := s.client.KickByStreams(streams)
		s.registry.ClearRoom(room)
		if err != nil {
			return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
		}
		if kicked == 0 && remaining == 0 && len(streams) == 0 {
			return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
		}
		return nil
	}
	// 降级：registry 未注入时走旧 DELETE /api/v1/streams/{name}（SRS5 返 2048，仍保留以兼容）。
	if err := s.client.DeleteRoom(room); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) GetHost() string {
	return s.serverURL
}

func (s *Service) ProviderName() string {
	return "srs"
}

func (s *Service) StreamName(room, identity string) string {
	return GenerateStreamName(room, identity)
}

func (s *Service) StreamInfo(room, identity string) (stream, token string) {
	stream = GenerateStreamName(room, identity)
	t, err := GenerateStreamToken(stream, s.secret)
	if err != nil {
		return stream, ""
	}
	return stream, t
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"whipUrl": s.whipURL,
	}
}