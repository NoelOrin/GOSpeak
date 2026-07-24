package service

import (
	"strings"

	"GOSpeak/internal/sfu/providers/cloudflare"
	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
)

// CloudflareMediaService proxies Cloudflare Realtime media APIs so the frontend
// never needs CF_APP_SECRET.
type CloudflareMediaService struct {
	resolve func() (*config.Config, error)
}

func NewCloudflareMediaService(resolve func() (*config.Config, error)) *CloudflareMediaService {
	return &CloudflareMediaService{resolve: resolve}
}

func (s *CloudflareMediaService) client() (*cloudflare.Client, error) {
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

func (s *CloudflareMediaService) AddTracks(sessionID string, req *cloudflare.TrackRequest) (*cloudflare.TracksResponse, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "sessionId is required")
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

func (s *CloudflareMediaService) Renegotiate(sessionID string, req *cloudflare.RenegotiateRequest) error {
	if strings.TrimSpace(sessionID) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "sessionId is required")
	}
	if req == nil || req.SessionDescription.SDP == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "sessionDescription is required")
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

func (s *CloudflareMediaService) CloseTracks(sessionID string, req *cloudflare.CloseTrackRequest) (*cloudflare.CloseTrackResponse, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "sessionId is required")
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

func (s *CloudflareMediaService) DeleteSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "sessionId is required")
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
