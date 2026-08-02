package service

import (
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupConversationServiceTestDB(t *testing.T) (*ConversationService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.ConversationParticipant{}); err != nil {
		t.Fatal(err)
	}
	svc := NewConversationService(
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
	)
	return svc, db
}

func TestConversationService_GetMessages_UsesConversationID(t *testing.T) {
	svc, db := setupConversationServiceTestDB(t)
	convID := "conv-service"
	cp := &model.ConversationParticipant{
		ConversationID: convID,
		IdentityA:      "alice",
		IdentityB:      "bob",
	}
	if err := db.Create(cp).Error; err != nil {
		t.Fatal(err)
	}

	convPtr := convID
	base := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 2; i++ {
		msg := &model.Message{
			AuthorID:         "alice",
			Content:          "dm",
			CreatedAt:        base.Add(time.Duration(i) * time.Second),
			ConversationType: "private",
			ConversationID:   &convPtr,
		}
		if err := db.Create(msg).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.Message{
		RoomUUID:  "room-uuid",
		AuthorID:  "alice",
		Content:   "room",
		CreatedAt: base.Add(3 * time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}

	out, err := svc.GetMessages(convID, "alice", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("want 2 dm messages, got %d", len(out.Messages))
	}
	for _, m := range out.Messages {
		if m.RoomUUID != "" {
			t.Fatalf("room message leaked into dm history: %+v", m)
		}
		if m.ConversationID != convID {
			t.Fatalf("want conversation_id %q, got %q", convID, m.ConversationID)
		}
	}
}

func TestConversationService_MarkRead_ResetsOwnUnread(t *testing.T) {
	svc, db := setupConversationServiceTestDB(t)
	convID := "conv-markread"
	if err := db.Create(&model.ConversationParticipant{
		ConversationID: convID,
		IdentityA:      "alice",
		IdentityB:      "bob",
		UnreadCountA:   3,
		UnreadCountB:   5,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.MarkRead(convID, "alice"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	var saved model.ConversationParticipant
	if err := db.First(&saved, "conversation_id = ?", convID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.UnreadCountA != 0 || saved.UnreadCountB != 5 {
		t.Fatalf("unexpected unread counts after own mark read: A=%d B=%d", saved.UnreadCountA, saved.UnreadCountB)
	}
}

func TestConversationService_MarkRead_RejectsNonParticipant(t *testing.T) {
	svc, db := setupConversationServiceTestDB(t)
	convID := "conv-markread"
	if err := db.Create(&model.ConversationParticipant{
		ConversationID: convID,
		IdentityA:      "alice",
		IdentityB:      "bob",
		UnreadCountA:   3,
		UnreadCountB:   5,
	}).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.MarkRead(convID, "eve")
	assertErrorCode(t, err, pkg.FORBIDDEN)

	var saved model.ConversationParticipant
	if err := db.First(&saved, "conversation_id = ?", convID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.UnreadCountA != 3 || saved.UnreadCountB != 5 {
		t.Fatalf("non-participant changed unread counts: A=%d B=%d", saved.UnreadCountA, saved.UnreadCountB)
	}
}

func TestConversationService_MarkRead_UnknownConversation(t *testing.T) {
	svc, _ := setupConversationServiceTestDB(t)
	assertErrorCode(t, svc.MarkRead("missing", "alice"), pkg.NOT_FOUND)
}
