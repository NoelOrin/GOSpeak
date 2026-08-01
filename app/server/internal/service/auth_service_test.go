package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAuthServiceTest(t *testing.T) *AuthService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	repo := repository.NewUserRepository(db)
	user := &model.User{
		UUID:         "uuid-alice",
		Name:         "alice",
		DisplayName:  "Alice",
		Role:         "user",
		TokenVersion: 1,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return NewAuthService(repo, nil, nil)
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
	access, err := svc.RefreshFromToken(refresh)
	if err != nil {
		t.Fatalf("RefreshFromToken: %v", err)
	}
	if access == "" {
		t.Fatal("expected new access token")
	}
}
