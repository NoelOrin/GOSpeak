package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateMessageAuthorUUID_BackfillsExistingRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Message{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{UUID: "uuid-alice", Name: "alice"}).Error; err != nil {
		t.Fatal(err)
	}
	rows := []model.Message{
		{AuthorID: "alice", Content: "legacy"},
		{AuthorID: "alice", AuthorUUID: "uuid-alice", Content: "new"},
		{AuthorID: "ghost", Content: "unmatched"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := migrateMessageAuthorUUID(db); err != nil {
		t.Fatalf("migrateMessageAuthorUUID: %v", err)
	}

	var saved []model.Message
	if err := db.Find(&saved).Error; err != nil {
		t.Fatal(err)
	}
	byContent := make(map[string]string, len(saved))
	for _, m := range saved {
		byContent[m.Content] = m.AuthorUUID
	}
	if got := byContent["legacy"]; got != "uuid-alice" {
		t.Errorf("legacy message author_uuid = %q, want uuid-alice", got)
	}
	if got := byContent["new"]; got != "uuid-alice" {
		t.Errorf("new message author_uuid = %q, want uuid-alice", got)
	}
	if got := byContent["unmatched"]; got != "" {
		t.Errorf("unmatched message author_uuid = %q, want empty", got)
	}
}
