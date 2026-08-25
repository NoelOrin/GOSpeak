package repository

import (
	"testing"
	"time"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testGuestBanDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DomainGuestBan{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestGuestBanRepo_FindActive(t *testing.T) {
	db := testGuestBanDB(t)
	repo := NewGuestBanRepo(db)
	mustCreate := func(b *model.DomainGuestBan) {
		if err := repo.Create(b); err != nil {
			t.Fatal(err)
		}
	}
	mustCreate(&model.DomainGuestBan{DomainUUID: "d1", UserUUID: "u1"}) // 永久
	past := time.Now().Add(-time.Hour)
	mustCreate(&model.DomainGuestBan{DomainUUID: "d1", UserUUID: "u2", ExpiresAt: &past})

	if got := repo.FindActive("d1", "u1"); got == nil {
		t.Fatal("u1 must be banned")
	}
	if got := repo.FindActive("d1", "u2"); got != nil {
		t.Fatal("expired ban must not be active")
	}
	if got := repo.FindActive("d1", "u3"); got != nil {
		t.Fatal("unknown user must be unbanned")
	}
}
