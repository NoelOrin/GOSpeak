package repository

import (
	"strings"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setOAuthCryptoKey(t *testing.T) {
	t.Helper()
	old := config.Current()
	config.SetCurrent(&config.Config{
		AppEnv:            "test",
		StorageEncryptKey: strings.Repeat("ab", 32),
	})
	t.Cleanup(func() { config.SetCurrent(old) })
}

func TestOAuthProviderRepository_EncryptsClientSecret(t *testing.T) {
	setOAuthCryptoKey(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.OAuthProvider{}); err != nil {
		t.Fatal(err)
	}

	repo := NewOAuthProviderRepository(db)
	provider := &model.OAuthProvider{Name: "github", ClientSecret: "plain-secret"}
	if err := repo.Create(provider); err != nil {
		t.Fatal(err)
	}

	var raw model.OAuthProvider
	if err := db.First(&raw, "name = ?", "github").Error; err != nil {
		t.Fatal(err)
	}
	if raw.ClientSecret == "plain-secret" {
		t.Fatal("client_secret was stored in plaintext")
	}
	if !strings.HasPrefix(raw.ClientSecret, oauthSecretPrefix) {
		t.Fatalf("client_secret missing encryption prefix: %q", raw.ClientSecret)
	}

	got, err := repo.GetByName("github")
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientSecret != "plain-secret" {
		t.Fatalf("GetByName returned %q, want plain-secret", got.ClientSecret)
	}
}

func TestOAuthProviderRepository_RejectsLegacyPlaintext(t *testing.T) {
	setOAuthCryptoKey(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.OAuthProvider{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.OAuthProvider{Name: "legacy", ClientSecret: "legacy-secret"}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := NewOAuthProviderRepository(db).GetByName("legacy"); err == nil {
		t.Fatal("legacy plaintext should not be accepted as an encrypted secret")
	}
}

func TestOAuthAccountRepository_EncryptsTokens(t *testing.T) {
	setOAuthCryptoKey(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthAccount{}); err != nil {
		t.Fatal(err)
	}

	repo := NewOAuthAccountRepository(db)
	user := &model.User{Name: "oauth-user"}
	account := &model.OAuthAccount{
		Provider:     "github",
		ProviderUID:  "uid-1",
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
	}
	if err := repo.CreateWithUser(user, account); err != nil {
		t.Fatal(err)
	}

	var raw model.OAuthAccount
	if err := db.First(&raw, "provider = ?", "github").Error; err != nil {
		t.Fatal(err)
	}
	if raw.AccessToken == "access-secret" || raw.RefreshToken == "refresh-secret" {
		t.Fatal("oauth tokens were stored in plaintext")
	}
	if !strings.HasPrefix(raw.AccessToken, oauthSecretPrefix) || !strings.HasPrefix(raw.RefreshToken, oauthSecretPrefix) {
		t.Fatalf("oauth tokens missing encryption prefix: access=%q refresh=%q", raw.AccessToken, raw.RefreshToken)
	}

	got, err := repo.GetByProviderAndUID("github", "uid-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "access-secret" || got.RefreshToken != "refresh-secret" {
		t.Fatalf("decrypted tokens mismatch: access=%q refresh=%q", got.AccessToken, got.RefreshToken)
	}

	got.AccessToken = "new-access"
	got.RefreshToken = "new-refresh"
	if err := repo.Update(got); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetByProviderAndUID("github", "uid-1")
	if err != nil {
		t.Fatal(err)
	}
	if updated.AccessToken != "new-access" || updated.RefreshToken != "new-refresh" {
		t.Fatalf("updated tokens mismatch: access=%q refresh=%q", updated.AccessToken, updated.RefreshToken)
	}
}
