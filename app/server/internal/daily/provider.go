package daily

import (
	"strings"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

type Service struct {
	client  *Client
	apiKey  string
	domain  string
	hostURL string
}

func NewService(cfg *config.Config) *Service {
	domain := strings.TrimSpace(cfg.DailyDomain)
	if domain != "" && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}
	return &Service{
		client:  NewClient(cfg.DailyAPIKey),
		apiKey:  strings.TrimSpace(cfg.DailyAPIKey),
		domain:  strings.TrimSpace(cfg.DailyDomain),
		hostURL: domain,
	}
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	if s.apiKey == "" || s.domain == "" {
		return "", pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "DAILY_API_KEY and DAILY_DOMAIN are required")
	}
	token, err := s.client.CreateMeetingToken(room, identity)
	if err != nil {
		return "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return token, nil
}

func (s *Service) GenerateAdminToken() (string, error) {
	return s.GenerateToken("admin", "admin")
}

func (s *Service) ListRooms() (interface{}, error) {
	if s.apiKey == "" {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "DAILY_API_KEY is required")
	}
	rooms, err := s.client.ListRooms()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return rooms, nil
}

func (s *Service) ListParticipants(room string) (interface{}, error) {
	if s.apiKey == "" {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "DAILY_API_KEY is required")
	}
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

// RemoveParticipant 通过 Daily REST API 踢出指定 identity。
// Daily 的 remove 端点需要 participant session id（非 user_name），
// 故先 ListParticipants 查 user_name==identity 的 id，再调用 remove。
// participant 不在房间时静默返回 nil（与 LiveKit/SRS 行为对齐）。
func (s *Service) RemoveParticipant(room, identity string) error {
	if s.apiKey == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "DAILY_API_KEY is required")
	}
	participants, err := s.client.ListParticipants(room)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	var targetID string
	for _, p := range participants {
		if name, ok := p["user_name"].(string); ok && name == identity {
			if id, ok := p["id"].(string); ok && id != "" {
				targetID = id
				break
			}
		}
	}
	if targetID == "" {
		return nil
	}
	if err := s.client.RemoveParticipant(room, targetID); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	if s.apiKey == "" {
		return pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "DAILY_API_KEY is required")
	}
	if err := s.client.DeleteRoom(room); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

func (s *Service) GetHost() string {
	return s.hostURL
}

func (s *Service) ProviderName() string {
	return "daily"
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"dailyDomain": s.domain,
	}
}
