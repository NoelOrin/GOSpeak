package repository

import (
	"testing"
	"time"

	"GOSpeak/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupMsgDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:msg_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.MessageReaction{}, &model.MessageMention{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMessageRepo_ListBefore(t *testing.T) {
	db := setupMsgDB(t)
	repo := NewMessageRepository(db)
	room := "room-1"
	for i := 0; i < 5; i++ {
		m := &model.Message{
			RoomUUID:  room,
			AuthorID:  "u1",
			Content:   "m",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := repo.Create(m); err != nil {
			t.Fatal(err)
		}
	}
	items, hasMore, err := repo.ListBefore(room, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 got %d", len(items))
	}
	if !hasMore {
		t.Fatal("want hasMore")
	}
	// items ASC: older first among the page of newest
	if items[0].CreatedAt.After(items[1].CreatedAt) {
		t.Fatal("want ASC order in returned page")
	}
	next, hasMore2, err := repo.ListBefore(room, items[0].UUID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) == 0 {
		t.Fatal("want older page")
	}
	_ = hasMore2
}
