package service

import (
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
