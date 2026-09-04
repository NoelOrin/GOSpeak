package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

type fakeGuestChecker struct {
	guests map[string]bool
	bans   map[string]bool // key: domainUUID + "|" + userUUID
}

func (f *fakeGuestChecker) IsGuest(userUUID string) bool { return f.guests[userUUID] }

func (f *fakeGuestChecker) IsGuestBanned(domainUUID, userUUID string) (bool, error) {
	return f.bans[domainUUID+"|"+userUUID], nil
}

func (f *fakeGuestChecker) IsGuestDomainMember(domainUUID, userUUID string) (bool, error) {
	return true, nil
}

// buildGuestGuardCase 构造带 claims 注入与守卫的最小路由。
func buildGuestGuardCase(t *testing.T, checker *fakeGuestChecker, path string, guest bool) (*gin.Engine, *bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	reached := false
	r := gin.New()
	r.Use(func(c *gin.Context) {
		uuid := "user-1"
		if !guest {
			uuid = "admin-1"
		}
		c.Set("claims", &struct{}{})
		c.Set("user_uuid", uuid)
		c.Next()
	})
	r.Use(GuestGuardWith(checker))
	r.POST(path, func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r, &reached
}

func doGuestGuardPost(r *gin.Engine, path, body string, query string) *httptest.ResponseRecorder {
	url := path
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func guardCode(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	code, _ := resp["code"].(float64)
	return int(code)
}

func TestGuestGuard_GuestAllowedPathPasses(t *testing.T) {
	checker := &fakeGuestChecker{guests: map[string]bool{"user-1": true}}
	r, reached := buildGuestGuardCase(t, checker, "/api/v1/room/messages/list", true)
	rec := doGuestGuardPost(r, "/api/v1/room/messages/list", `{}`, "")
	if rec.Code != http.StatusOK || !*reached {
		t.Fatalf("allowed path must reach handler: %d reached=%v %s", rec.Code, *reached, rec.Body.String())
	}
}

func TestGuestGuard_GuestBlockedOnOutsidePath(t *testing.T) {
	checker := &fakeGuestChecker{guests: map[string]bool{"user-1": true}}
	r, reached := buildGuestGuardCase(t, checker, "/api/v1/user/list", true)
	rec := doGuestGuardPost(r, "/api/v1/user/list", `{}`, "")
	if *reached || rec.Code == http.StatusOK {
		t.Fatal("outside path must be blocked before handler")
	}
	if got := guardCode(t, rec); got != int(pkg.FORBIDDEN) {
		t.Fatalf("expect 1013, got %d: %s", got, rec.Body.String())
	}
}

func TestGuestGuard_BannedGuestBlockedWithQueryDomain(t *testing.T) {
	checker := &fakeGuestChecker{
		guests: map[string]bool{"user-1": true},
		bans:   map[string]bool{"dom-1|user-1": true},
	}
	r, reached := buildGuestGuardCase(t, checker, "/api/v1/room/messages/list", true)
	rec := doGuestGuardPost(r, "/api/v1/room/messages/list", `{}`, "domain_uuid=dom-1")
	if *reached || rec.Code == http.StatusOK {
		t.Fatal("banned guest must be blocked")
	}
	if got := guardCode(t, rec); got != int(pkg.FORBIDDEN) {
		t.Fatalf("expect 1013, got %d", got)
	}
}

func TestGuestGuard_BannedGuestBlockedWithBodyDomain(t *testing.T) {
	checker := &fakeGuestChecker{
		guests: map[string]bool{"user-1": true},
		bans:   map[string]bool{"dom-2|user-1": true},
	}
	gin.SetMode(gin.TestMode)
	reached := false
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_uuid", "user-1")
		c.Next()
	})
	r.Use(GuestGuardWith(checker))
	r.POST("/api/v1/signal/token", func(c *gin.Context) {
		var body struct {
			DomainUUID string `json:"domain_uuid"`
		}
		_ = c.ShouldBindJSON(&body) // 守卫必须已回填 body，下游仍可读
		if body.DomainUUID != "dom-2" {
			t.Errorf("downstream lost body domain_uuid: %q", body.DomainUUID)
		}
		reached = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	rec := doGuestGuardPost(r, "/api/v1/signal/token", `{"domain_uuid":"dom-2","room":"r1"}`, "")
	if reached || rec.Code == http.StatusOK {
		t.Fatal("banned guest must be blocked even when domain in body")
	}
}

func TestGuestGuard_NonGuestUnrestricted(t *testing.T) {
	checker := &fakeGuestChecker{guests: map[string]bool{}}
	r, reached := buildGuestGuardCase(t, checker, "/api/v1/user/list", false)
	rec := doGuestGuardPost(r, "/api/v1/user/list", `{}`, "")
	if rec.Code != http.StatusOK || !*reached {
		t.Fatal("non-guest must pass through")
	}
}
