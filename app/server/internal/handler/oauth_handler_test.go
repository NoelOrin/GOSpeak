package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestOAuthHandler(t *testing.T) *OAuthHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OAuthProvider{}, &model.OAuthAccount{}, &model.User{}); err != nil {
		t.Fatalf("migrate oauth models: %v", err)
	}
	providerRepo := repository.NewOAuthProviderRepository(db)
	svc := service.NewOAuthService(
		providerRepo,
		repository.NewOAuthAccountRepository(db),
		repository.NewUserRepository(db),
	)
	if err := providerRepo.Create(&model.OAuthProvider{Name: "github", Enabled: true}); err != nil {
		t.Fatalf("seed oauth provider: %v", err)
	}
	return NewOAuthHandler(svc, defaultAuthCookieConfig())
}

func TestOAuthHandler_Callback_BadState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestOAuthHandler(t)

	router := gin.New()
	router.GET("/cb/:provider", h.Callback)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cb/github?code=x&state=forged", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "oauth_error") {
		t.Fatalf("expected oauth_error redirect, got %q", rec.Header().Get("Location"))
	}
}

func TestOAuthHandler_Login_IgnoresClientState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestOAuthHandler(t)

	router := gin.New()
	router.GET("/login/:provider", h.Login)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login/github?state=attacker-state", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	var stateCookie string
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == oauthStateCookie {
			stateCookie = ck.Value
		}
	}
	if stateCookie == "" {
		t.Fatal("expected oauth state cookie")
	}
	if stateCookie == "attacker-state" {
		t.Fatal("handler must not trust client-provided state")
	}
}
