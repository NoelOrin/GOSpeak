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
		whipURL:   serverURL + "/rtc/v1/whip/",
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
	rooms, err := s.client.ListRooms()
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
	if err := s.client.RemoveParticipant(room, identity); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) DeleteRoom(room string) error {
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

func (s *Service) StreamInfo(room, identity string) (stream, token string) {
	stream = GenerateStreamName(room, identity)
	token = GenerateStreamToken(stream, s.secret)
	return
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"whipUrl": s.whipURL,
	}
}
