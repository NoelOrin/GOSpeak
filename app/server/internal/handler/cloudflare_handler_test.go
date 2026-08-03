package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu/providers/cloudflare"

	"github.com/gin-gonic/gin"
)

type fakeCloudflareMediaService struct {
	sessionID         string
	addTracksUser     string
	renegotiateUser   string
	closeTracksUser   string
	deleteSessionUser string
	err               error
}

func (f *fakeCloudflareMediaService) AddTracks(sessionID, userUUID string, req *cloudflare.TrackRequest) (*cloudflare.TracksResponse, error) {
	f.sessionID = sessionID
	f.addTracksUser = userUUID
	if f.err != nil {
		return nil, f.err
	}
	return &cloudflare.TracksResponse{}, nil
}

func (f *fakeCloudflareMediaService) Renegotiate(sessionID, userUUID string, req *cloudflare.RenegotiateRequest) error {
	f.sessionID = sessionID
	f.renegotiateUser = userUUID
	return f.err
}

func (f *fakeCloudflareMediaService) CloseTracks(sessionID, userUUID string, req *cloudflare.CloseTrackRequest) (*cloudflare.CloseTrackResponse, error) {
	f.sessionID = sessionID
	f.closeTracksUser = userUUID
	if f.err != nil {
		return nil, f.err
	}
	return &cloudflare.CloseTrackResponse{}, nil
}

func (f *fakeCloudflareMediaService) DeleteSession(sessionID, userUUID string) error {
	f.sessionID = sessionID
	f.deleteSessionUser = userUUID
	return f.err
}

func setupCloudflareRouter(svc cloudflareMediaService, userUUID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "alice")
		if userUUID != "" {
			c.Set("user_uuid", userUUID)
		}
		c.Next()
	})
	h := NewCloudflareHandler(svc)
	r.POST("/sessions/:sessionId/tracks/new", h.AddTracks)
	r.PUT("/sessions/:sessionId/renegotiate", h.Renegotiate)
	r.PUT("/sessions/:sessionId/tracks/close", h.CloseTracks)
	r.DELETE("/sessions/:sessionId", h.DeleteSession)
	return r
}

type cloudflareRouteCase struct {
	name   string
	method string
	path   string
	body   string
	user   func(*fakeCloudflareMediaService) string
}

func cloudflareRouteCases() []cloudflareRouteCase {
	return []cloudflareRouteCase{
		{
			name:   "add tracks",
			method: http.MethodPost,
			path:   "/sessions/sess-1/tracks/new",
			body:   `{}`,
			user:   func(f *fakeCloudflareMediaService) string { return f.addTracksUser },
		},
		{
			name:   "renegotiate",
			method: http.MethodPut,
			path:   "/sessions/sess-1/renegotiate",
			body:   `{"sessionDescription":{"type":"answer","sdp":"sdp"}}`,
			user:   func(f *fakeCloudflareMediaService) string { return f.renegotiateUser },
		},
		{
			name:   "close tracks",
			method: http.MethodPut,
			path:   "/sessions/sess-1/tracks/close",
			body:   `{}`,
			user:   func(f *fakeCloudflareMediaService) string { return f.closeTracksUser },
		},
		{
			name:   "delete session",
			method: http.MethodDelete,
			path:   "/sessions/sess-1",
			body:   ``,
			user:   func(f *fakeCloudflareMediaService) string { return f.deleteSessionUser },
		},
	}
}

func TestCloudflareHandler_OwnerOperationsSucceed(t *testing.T) {
	for _, tc := range cloudflareRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeCloudflareMediaService{}
			r := setupCloudflareRouter(fake, "uuid-owner")

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected HTTP 200, got %d: %s", w.Code, w.Body.String())
			}
			resp := parseResp(t, w.Body.String())
			if resp.Code != pkg.SUCCESS {
				t.Fatalf("expected business code %d, got %d", pkg.SUCCESS, resp.Code)
			}
			if fake.sessionID != "sess-1" {
				t.Fatalf("expected sessionId sess-1, got %q", fake.sessionID)
			}
			if got := tc.user(fake); got != "uuid-owner" {
				t.Fatalf("expected user uuid-owner, got %q", got)
			}
		})
	}
}

func TestCloudflareHandler_ForbiddenPropagatesToClient(t *testing.T) {
	for _, tc := range cloudflareRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeCloudflareMediaService{err: pkg.NewAppError(pkg.FORBIDDEN, "not the session owner")}
			r := setupCloudflareRouter(fake, "uuid-other")

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected HTTP 403, got %d", w.Code)
			}
			resp := parseResp(t, w.Body.String())
			if resp.Code != pkg.FORBIDDEN {
				t.Fatalf("expected business code %d, got %d", pkg.FORBIDDEN, resp.Code)
			}
			if fake.sessionID != "sess-1" {
				t.Fatalf("expected sessionId sess-1, got %q", fake.sessionID)
			}
		})
	}
}
