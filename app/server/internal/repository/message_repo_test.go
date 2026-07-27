package repository

import (
	"GOSpeak/internal/model"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

func setupMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Message{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMessageRepository_CreateAndList(t *testing.T) {
	db := setupMessageTestDB(t)
	repo := NewMessageRepository(db)
	room := "11111111-1111-1111-1111-111111111111"
	id1 := ulid.Make().String()
	time.Sleep(2 * time.Millisecond)
	id2 := ulid.Make().String()

	for _, m := range []*model.Message{
		{ID: id1, RoomUUID: room, SenderIdentity: "alice", SenderDisplay: "Alice", SenderRole: "user", Content: "hi", Status: model.MessageStatusActive},
		{ID: id2, RoomUUID: room, SenderIdentity: "bob", SenderDisplay: "Bob", SenderRole: "user", Content: "yo", Status: model.MessageStatusActive},
	} {
		if err := repo.Create(m); err != nil {
			t.Fatal(err)
		}
	}

	list, err := repo.ListByRoom(room, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].ID != id1 || list[1].ID != id2 {
		t.Fatalf("order wrong: %+v", list)
	}

	older, err := repo.ListByRoom(room, id2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(older) != 1 || older[0].ID != id1 {
		t.Fatalf("cursor page: %+v", older)
	}
}
