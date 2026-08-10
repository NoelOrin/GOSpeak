package service

import (
	"strings"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu/providers/cloudflare"
)

// cloudflareMediaClient is the subset of cloudflare.Client used by the media service.
type cloudflareMediaClient interface {
	AddTracks(sessionID string, req *cloudflare.TrackRequest) (*cloudflare.TracksResponse, error)
	Renegotiate(sessionID string, req *cloudflare.RenegotiateRequest) error
	CloseTracks(sessionID string, req *cloudflare.CloseTrackRequest) (*cloudflare.CloseTrackResponse, error)
	DeleteSession(sessionID string) error
}

// CloudflareMediaService proxies Cloudflare Realtime media APIs so the frontend
// never needs CF_APP_SECRET. Session ownership is checked before any API call.
type CloudflareMediaService struct {
	resolve       func() (*config.Config, error)
	clientFactory func() (cloudflareMediaClient, error)
	sessionOwner  func(sessionID string) (string, bool)
	sessionDomain func(sessionID string) (string, bool)
	domainMember  func(domainUUID, userUUID string) bool
}

func NewCloudflareMediaService(resolve func() (*config.Config, error)) *CloudflareMediaService {
	s := &CloudflareMediaService{resolve: resolve}
	s.clientFactory = s.defaultClient
	return s
}

// SetSessionOwnerLookup injects sessionID → ownerUUID lookup for IDOR protection.
func (s *CloudflareMediaService) SetSessionOwnerLookup(lookup func(sessionID string) (string, bool)) {
	s.sessionOwner = lookup
}

// SetSessionDomainLookup 注入 sessionID → domainUUID 查询。
func (s *CloudflareMediaService) SetSessionDomainLookup(lookup func(sessionID string) (string, bool)) {
	s.sessionDomain = lookup
}

// SetDomainMemberChecker 注入 Domain 成员校验函数。
func (s *CloudflareMediaService) SetDomainMemberChecker(checker func(domainUUID, userUUID string) bool) {
	s.domainMember = checker
}

func (s *CloudflareMediaService) defaultClient() (cloudflareMediaClient, error) {
	if s.resolve == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "cloudflare config resolver missing")
	}
	cfg, err := s.resolve()
	if err != nil {
		return nil, err
	}
	if cfg == nil || strings.TrimSpace(cfg.CFAppID) == "" || strings.TrimSpace(cfg.CFAppSecret) == "" {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "CF_APP_ID and CF_APP_SECRET are required")
	}
	return cloudflare.NewClient(cfg.CFAppID, cfg.CFAppSecret), nil
}

func (s *CloudflareMediaService) client() (cloudflareMediaClient, error) {
	if s.clientFactory == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "cloudflare client factory missing")
	}
	return s.clientFactory()
}

// validateSessionID 校验 Cloudflare session 标识：非空且长度受限。
func validateSessionID(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "sessionId is required")
	}
	if len(sessionID) > 128 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "sessionId is too long")
	}
	return nil
}

func (s *CloudflareMediaService) authorizeSession(sessionID, userUUID string) error {
	if strings.TrimSpace(userUUID) == "" {
		return pkg.NewAppError(pkg.FORBIDDEN, "session owner is required")
	}
	if s.sessionOwner == nil {
		return pkg.NewAppError(pkg.FORBIDDEN, "session ownership is not available")
	}
	owner, ok := s.sessionOwner(sessionID)
	if !ok || owner != userUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "not the session owner")
	}
	if s.sessionDomain != nil && s.domainMember != nil {
		if domainUUID, ok := s.sessionDomain(sessionID); ok && domainUUID != "" {
			if !s.domainMember(domainUUID, userUUID) {
				return pkg.NewAppError(pkg.FORBIDDEN, "not a member of the session domain")
			}
		}
	}
	return nil
}

func (s *CloudflareMediaService) AddTracks(sessionID, userUUID string, req *cloudflare.TrackRequest) (*cloudflare.TracksResponse, error) {
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	if err := s.authorizeSession(sessionID, userUUID); err != nil {
		return nil, err
	}
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	resp, err := client.AddTracks(sessionID, req)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare add tracks: "+err.Error())
	}
	return resp, nil
}

func (s *CloudflareMediaService) Renegotiate(sessionID, userUUID string, req *cloudflare.RenegotiateRequest) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	if req == nil || req.SessionDescription.SDP == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "sessionDescription is required")
	}
	if err := s.authorizeSession(sessionID, userUUID); err != nil {
		return err
	}
	client, err := s.client()
	if err != nil {
		return err
	}
	if err := client.Renegotiate(sessionID, req); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare renegotiate: "+err.Error())
	}
	return nil
}

func (s *CloudflareMediaService) CloseTracks(sessionID, userUUID string, req *cloudflare.CloseTrackRequest) (*cloudflare.CloseTrackResponse, error) {
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	if err := s.authorizeSession(sessionID, userUUID); err != nil {
		return nil, err
	}
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	resp, err := client.CloseTracks(sessionID, req)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare close tracks: "+err.Error())
	}
	return resp, nil
}

func (s *CloudflareMediaService) DeleteSession(sessionID, userUUID string) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	if err := s.authorizeSession(sessionID, userUUID); err != nil {
		return err
	}
	client, err := s.client()
	if err != nil {
		return err
	}
	if err := client.DeleteSession(sessionID); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "cloudflare delete session: "+err.Error())
	}
	return nil
}
