package repository

import (
	"testing"
	"time"

	"GOSpeak/internal/model"
)

func TestMessageRepo_ListBeforeConversation(t *testing.T) {
	db := setupMsgDB(t)
	repo := NewMessageRepository(db)
	room := "room-conv"
	convID := "conv-1"
	convPtr := convID
	base := time.Now().UTC().Add(-time.Hour)

	for i := 0; i < 5; i++ {
		created := base.Add(time.Duration(i) * time.Second)
		roomMsg := &model.Message{
			RoomUUID:  room,
			AuthorID:  "u1",
			Content:   "room",
			CreatedAt: created,
		}
		if err := repo.Create(roomMsg); err != nil {
			t.Fatal(err)
		}
		privateMsg := &model.Message{
			AuthorID:         "u1",
			Content:          "dm",
			CreatedAt:        created,
			ConversationType: "private",
			ConversationID:   &convPtr,
		}
		if err := repo.Create(privateMsg); err != nil {
			t.Fatal(err)
		}
	}

	items, hasMore, err := repo.ListBeforeConversation(convID, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 got %d", len(items))
	}
	if !hasMore {
		t.Fatal("want hasMore")
	}
	for _, m := range items {
		if m.RoomUUID != "" {
			t.Fatalf("room message leaked into dm history: %+v", m)
		}
		if m.ConversationID == nil || *m.ConversationID != convID {
			t.Fatalf("want conversation_id %q, got %+v", convID, m.ConversationID)
		}
	}
	if items[0].CreatedAt.After(items[1].CreatedAt) {
		t.Fatal("want ASC order in returned page")
	}

	next, _, err := repo.ListBeforeConversation(convID, items[0].UUID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) == 0 {
		t.Fatal("want older private page")
	}

	roomItems, _, err := repo.ListBefore(room, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(roomItems) != 5 {
		t.Fatalf("room query should stay room-only, got %d", len(roomItems))
	}
}
