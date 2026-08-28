package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"GOSpeak/internal/authstate"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func newTestAuthHandler(t *testing.T) *AuthHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	return NewAuthHandler(service.NewAuthService(repository.NewUserRepository(db), nil, nil), defaultAuthCookieConfig())
}

func TestAuthHandler_Register_RejectsBotPrefixCaseInsensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler(t)
	router := gin.New()
	router.POST("/register", h.Register)

	for _, name := range []string{"Bot_test", "BOT_test", "bOt_test"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodPost,
			"/register",
			strings.NewReader(`{"username":"`+name+`","password":"password123"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if code := intCode(resp["code"]); code != 1012 {
			t.Fatalf("username %q: expected code 1012, got %d", name, code)
		}
	}
}

func TestAuthHandler_Login_SetsHttpOnlyCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	userRepo := repository.NewUserRepository(db)
	hashed, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := userRepo.Create(&model.User{
		Name:     "alice",
		Password: string(hashed),
		Role:     "user",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	h := NewAuthHandler(service.NewAuthService(userRepo, nil, nil), defaultAuthCookieConfig())
	router := gin.New()
	router.POST("/login", h.Login)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		strings.NewReader(`{"username":"alice","password":"password123"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if data, ok := resp["data"].(map[string]interface{}); !ok {
		t.Fatalf("expected data object, got %T", resp["data"])
	} else if expiresIn, ok := data["expires_in"].(float64); !ok || expiresIn != 900 {
		t.Fatalf("expires_in = %v, want 900 (default 15m)", data["expires_in"])
	}

	cookies := rec.Result().Cookies()
	var access, refresh *http.Cookie
	for _, ck := range cookies {
		switch ck.Name {
		case "gospeak_token":
			access = ck
		case "gospeak_refresh_token":
			refresh = ck
		}
	}
	if access == nil || access.Value == "" {
		t.Fatal("expected access token cookie")
	}
	if !access.HttpOnly {
		t.Fatal("access cookie must be HttpOnly")
	}
	if access.Path != "/" || access.MaxAge <= 0 {
		t.Fatalf("unexpected access cookie attributes: path=%q maxAge=%d", access.Path, access.MaxAge)
	}
	if refresh == nil || refresh.Value == "" || !refresh.HttpOnly {
		t.Fatal("expected HttpOnly refresh token cookie")
	}
}

func TestAuthHandler_GetRefreshToken_ReadsRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	userRepo := repository.NewUserRepository(db)
	if err := userRepo.Create(&model.User{
		UUID:         "uuid-alice",
		Name:         "alice",
		DisplayName:  "Alice",
		Role:         "user",
		TokenVersion: 1,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	h := NewAuthHandler(service.NewAuthService(userRepo, nil, nil), defaultAuthCookieConfig())
	router := gin.New()
	router.POST("/refresh_token", h.GetRefreshToken)

	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/refresh_token", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: "gospeak_refresh_token", Value: refresh})
	router.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0 from refresh cookie, got %d: %s", code, rec.Body.String())
	}
	if data, ok := resp["data"].(map[string]interface{}); !ok {
		t.Fatalf("expected data object, got %T", resp["data"])
	} else if expiresIn, ok := data["expires_in"].(float64); !ok || expiresIn != 900 {
		t.Fatalf("expires_in = %v, want 900 (default 15m)", data["expires_in"])
	}

	var gotAccess, gotRefresh bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "gospeak_token" && ck.Value != "" {
			gotAccess = true
		}
		if ck.Name == "gospeak_refresh_token" && ck.Value != "" && ck.Value != refresh {
			gotRefresh = true
		}
	}
	if !gotAccess || !gotRefresh {
		t.Fatal("expected rotated access/refresh cookies after refresh")
	}
}

type fakeAuthBackend struct {
	blacklisted map[string]time.Time
}

func (f *fakeAuthBackend) BlacklistToken(jti string, remaining time.Duration) error {
	f.blacklisted[jti] = time.Now().Add(remaining)
	return nil
}
func (f *fakeAuthBackend) IsBlacklisted(jti string) bool { _, ok := f.blacklisted[jti]; return ok }
func (f *fakeAuthBackend) IsBlacklistedErr(jti string) (bool, error) {
	return f.IsBlacklisted(jti), nil
}
func (f *fakeAuthBackend) GetSigningKey() (string, bool, error) { return "", false, nil }
func (f *fakeAuthBackend) SetSigningKey(string, int64) error    { return nil }
func (f *fakeAuthBackend) UpdateSigningKey(string, int64) error { return nil }
func (f *fakeAuthBackend) GetCreatedAt() (int64, bool, error)   { return 0, false, nil }
func (f *fakeAuthBackend) AddHistoryKey(string) error           { return nil }
func (f *fakeAuthBackend) HistoryKeys() []string                { return nil }
func (f *fakeAuthBackend) MarkRefreshFamilyUsed(string, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeAuthBackend) IsRefreshFamilyUsed(string) (bool, error) { return false, nil }
func (f *fakeAuthBackend) RevokeRefreshFamily(string) error         { return nil }
func (f *fakeAuthBackend) Backend() string                          { return "fake" }

func TestAuthHandler_Logout_RevokesRefreshTokenFromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestAuthHandler(t)
	backend := &fakeAuthBackend{blacklisted: map[string]time.Time{}}
	authstate.SetBackend(backend)
	t.Cleanup(func() { authstate.SetBackend(nil) })

	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	accessClaims, err := pkg.ParseToken(access)
	if err != nil {
		t.Fatalf("ParseToken access: %v", err)
	}
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	refreshClaims, err := pkg.ParseToken(refresh)
	if err != nil {
		t.Fatalf("ParseToken refresh: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", accessClaims)
		c.Next()
	})
	router.POST("/logout", h.Logout)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: h.cookie.RefreshName, Value: refresh})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, resp["msg"])
	}
	if revoked, _ := authstate.IsBlacklistedErr(refreshClaims.ID); !revoked {
		t.Fatal("refresh token from cookie must be blacklisted")
	}
}
