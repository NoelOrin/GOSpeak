package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestOAuthService(t *testing.T) *OAuthService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OAuthProvider{}, &model.OAuthAccount{}, &model.User{}); err != nil {
		t.Fatalf("migrate oauth models: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	svc := NewOAuthService(
		repository.NewOAuthProviderRepository(db),
		repository.NewOAuthAccountRepository(db),
		repository.NewUserRepository(db),
	)
	if err := svc.providerRepo.Create(&model.OAuthProvider{Name: "github", Enabled: true}); err != nil {
		t.Fatalf("seed oauth provider: %v", err)
	}
	return svc
}

func (s *OAuthService) disableProvider(name string) error {
	provider, err := s.providerRepo.GetByName(name)
	if err != nil {
		return err
	}
	provider.Enabled = false
	return s.providerRepo.Update(provider)
}

func TestOAuthService_HandleCallback_UnknownProvider(t *testing.T) {
	svc := newTestOAuthService(t)

	_, err := svc.HandleCallback("nope", "code")
	checkAppErrCode(t, err, pkg.OAUTH_PROVIDER_NOT_FOUND)
}

func TestOAuthService_HandleCallback_DisabledProvider(t *testing.T) {
	svc := newTestOAuthService(t)
	if err := svc.disableProvider("github"); err != nil {
		t.Fatalf("disable provider: %v", err)
	}

	_, err := svc.HandleCallback("github", "code")
	checkAppErrCode(t, err, pkg.OAUTH_PROVIDER_DISABLED)
}

func oauthGithubTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, _ = w.Write([]byte("access_token=tok"))
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id": 7, "login": "alice"}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func configureGithubProvider(t *testing.T, svc *OAuthService, ts *httptest.Server) {
	t.Helper()
	provider, err := svc.providerRepo.GetByName("github")
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	provider.ClientID = "cid"
	provider.ClientSecret = "secret"
	provider.AuthURL = ts.URL + "/auth"
	provider.TokenURL = ts.URL + "/token"
	provider.UserInfoURL = ts.URL + "/user"
	if err := svc.providerRepo.Update(provider); err != nil {
		t.Fatalf("update provider: %v", err)
	}
}

func TestOAuthService_HandleCallback_AutoCreateDisabled(t *testing.T) {
	ts := oauthGithubTestServer(t)
	defer ts.Close()
	svc := newTestOAuthService(t)
	configureGithubProvider(t, svc, ts)

	svc.SetAutoCreateUser(false)
	_, err := svc.HandleCallback("github", "code")
	checkAppErrCode(t, err, pkg.FORBIDDEN)

	if _, err := svc.userRepo.GetByName("alice"); err == nil {
		t.Fatal("expected no user to be auto-created")
	}
}

func TestOAuthService_HandleCallback_AutoCreateEnabled(t *testing.T) {
	ts := oauthGithubTestServer(t)
	defer ts.Close()
	svc := newTestOAuthService(t)
	configureGithubProvider(t, svc, ts)

	_, err := svc.HandleCallback("github", "code")
	if err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if _, err := svc.userRepo.GetByName("alice"); err != nil {
		t.Fatalf("expected user to be auto-created: %v", err)
	}
}
