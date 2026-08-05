package service

import (
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func newTestAuthService(t *testing.T) *AuthService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	return NewAuthService(repository.NewUserRepository(db), nil, nil)
}

func setupAuthServiceTest(t *testing.T) *AuthService {
	t.Helper()
	svc := newTestAuthService(t)
	user := &model.User{
		UUID:         "uuid-alice",
		Name:         "alice",
		DisplayName:  "Alice",
		Role:         "user",
		TokenVersion: 1,
	}
	if err := svc.userRepo.Create(user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return svc
}

func createUser(t *testing.T, svc *AuthService, name, password string, version uint) *model.User {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &model.User{
		UUID:         "uuid-" + name,
		Name:         name,
		DisplayName:  name,
		Password:     string(hashed),
		Role:         "user",
		TokenVersion: version,
	}
	if err := svc.userRepo.Create(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func newTestAuthServiceWithEmail(t *testing.T) *AuthService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.EmailConfig{}, &model.EmailVerificationCode{}); err != nil {
		t.Fatalf("migrate auth/email models: %v", err)
	}
	userRepo := repository.NewUserRepository(db)
	baseCfg := &config.Config{
		EmailEnabled:    true,
		SMTPHost:        "smtp.test",
		SMTPPort:        "587",
		SMTPUsername:    "test",
		SMTPPassword:    "test",
		SMTPFrom:        "noreply@test.local",
		EmailCodeSecret: "test-secret",
	}
	resolve := func() (*config.Config, error) { return baseCfg, nil }
	emailConfigService := NewEmailConfigService(repository.NewEmailConfigRepository(db), baseCfg)
	emailVerificationService := NewEmailVerificationService(
		repository.NewEmailVerificationCodeRepository(db),
		userRepo,
		NewEmailService(resolve),
		resolve,
	)
	return NewAuthService(userRepo, emailConfigService, emailVerificationService)
}

func TestAuthService_RefreshFromToken_RejectsNonRefreshTokens(t *testing.T) {
	svc := setupAuthServiceTest(t)

	access, err := pkg.GenerateToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := svc.RefreshFromToken(access); err == nil {
		t.Fatal("access token must not be usable as refresh token")
	} else {
		checkAppErrCode(t, err, pkg.TOKEN_WRONG)
	}

	ticket, err := pkg.GenerateWSTicket("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}
	if _, err := svc.RefreshFromToken(ticket); err == nil {
		t.Fatal("ws ticket must not be usable as refresh token")
	} else {
		checkAppErrCode(t, err, pkg.TOKEN_WRONG)
	}
}

func TestAuthService_RefreshFromToken_AcceptsRefreshToken(t *testing.T) {
	svc := setupAuthServiceTest(t)

	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	resp, err := svc.RefreshFromToken(refresh)
	if err != nil {
		t.Fatalf("RefreshFromToken: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatal("expected new access token")
	}
}

func TestRefreshFromToken_RotatesAndDetectsReuse(t *testing.T) {
	svc := setupAuthServiceTest(t)
	refresh, err := pkg.GenerateRefreshToken("alice", "Alice", "uuid-alice", "user", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	resp, err := svc.RefreshFromToken(refresh)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if resp.RefreshToken == "" || resp.RefreshToken == refresh {
		t.Fatal("expected rotated refresh token")
	}
	if _, err := svc.RefreshFromToken(refresh); err == nil {
		t.Fatal("expected reuse rejection")
	}
}

func TestAuthTokenStateMachine(t *testing.T) {
	t.Run("change password revokes old token", func(t *testing.T) {
		svc := newTestAuthService(t)
		user := createUser(t, svc, "alice", "old-pass", 1)
		middleware.SetTokenVersionChecker(svc)
		t.Cleanup(func() { middleware.SetTokenVersionChecker(nil) })

		if err := svc.ChangePassword("alice", "old-pass", "new-pass"); err != nil {
			t.Fatalf("ChangePassword: %v", err)
		}
		token, _ := pkg.GenerateToken(user.Name, user.DisplayName, user.UUID, user.Role, 1)
		if _, code := middleware.VerifyToken(token); code != pkg.TOKEN_REVOKED {
			t.Fatalf("expected TOKEN_REVOKED, got %d", code)
		}
	})

	t.Run("reset password requires email verification", func(t *testing.T) {
		svc := newTestAuthServiceWithEmail(t)
		if err := svc.ResetPassword("alice@example.com", "bad-code", "new-pass"); err == nil {
			t.Fatal("expected verification failure")
		}
	})
}
