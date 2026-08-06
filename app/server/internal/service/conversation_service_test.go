package service

import (
	"testing"
	"time"

	"context"
	"errors"
	"sync"

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

type conversationTestBus struct {
	mu         sync.Mutex
	publishErr error
	rooms      []string
}

func (b *conversationTestBus) PublishNamespace(_ context.Context, _ string, _ interface{}) error {
	return nil
}
func (b *conversationTestBus) PublishInternal(_ context.Context, _ string, _ interface{}) error {
	return nil
}
func (b *conversationTestBus) PublishRoom(_ context.Context, room, event string, _ interface{}) error {
	if b.publishErr != nil {
		return b.publishErr
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rooms = append(b.rooms, room+":"+event)
	return nil
}
func (b *conversationTestBus) Mode() string       { return "test" }
func (b *conversationTestBus) IsConnected() bool  { return true }
func (b *conversationTestBus) InstanceID() string { return "test" }
func (b *conversationTestBus) Close() error       { return nil }

func TestConversationService_GetMessages_ReturnsUnreadCount(t *testing.T) {
	svc, db := setupConversationServiceTestDB(t)
	convID := "conv-unread"
	if err := db.Create(&model.ConversationParticipant{
		ConversationID: convID,
		IdentityA:      "alice",
		IdentityB:      "bob",
		UnreadCountA:   3,
		UnreadCountB:   7,
	}).Error; err != nil {
		t.Fatal(err)
	}

	out, err := svc.GetMessages(convID, "alice", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.UnreadCount != 3 {
		t.Fatalf("alice unread = %d, want 3", out.UnreadCount)
	}
	out, err = svc.GetMessages(convID, "bob", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.UnreadCount != 7 {
		t.Fatalf("bob unread = %d, want 7", out.UnreadCount)
	}
}

func TestConversationService_SendDirect_PersistsWhenPublishFails(t *testing.T) {
	svc, db := setupConversationServiceTestDB(t)
	bus := &conversationTestBus{publishErr: errors.New("nats disconnected")}
	svc.SetEventBus(bus)

	dto, err := svc.SendDirect("alice", "bob", "offline hello", "nonce-1")
	if err != nil {
		t.Fatalf("SendDirect: %v", err)
	}
	if dto == nil {
		t.Fatal("expected dto")
	}

	var cp model.ConversationParticipant
	if err := db.First(&cp, "conversation_id = ?", dto.ConversationID).Error; err != nil {
		t.Fatal(err)
	}
	if cp.UnreadCountB != 1 {
		t.Fatalf("receiver unread = %d, want 1", cp.UnreadCountB)
	}
	var count int64
	if err := db.Model(&model.Message{}).Where("conversation_id = ?", dto.ConversationID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("persisted messages = %d, want 1", count)
	}
}

func TestConversationService_SendDirect_EnqueuesPersistJob(t *testing.T) {
	svc, db := setupConversationServiceTestDB(t)
	queue := &fakeQueue{}
	svc.SetJobQueue(queue)

	dto, err := svc.SendDirect("alice", "bob", "job hello", "nonce-1")
	if err != nil {
		t.Fatalf("SendDirect: %v", err)
	}
	if len(queue.jobs) != 1 {
		t.Fatalf("expected 1 persist job, got %d", len(queue.jobs))
	}
	job := queue.jobs[0]
	if job.Type != "chat.private.persist" {
		t.Fatalf("expected chat.private.persist, got %s", job.Type)
	}

	var count int64
	if err := db.Model(&model.Message{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("message must not be written before job consumption, count=%d", count)
	}

	if err := svc.PersistPrivateFromJob(job.Payload); err != nil {
		t.Fatalf("PersistPrivateFromJob: %v", err)
	}
	var msg model.Message
	if err := db.First(&msg, "uuid = ?", dto.UUID).Error; err != nil {
		t.Fatalf("message not persisted: %v", err)
	}
	var cp model.ConversationParticipant
	if err := db.First(&cp, "conversation_id = ?", dto.ConversationID).Error; err != nil {
		t.Fatalf("conversation not upserted: %v", err)
	}
	if cp.UnreadCountB != 1 {
		t.Fatalf("receiver unread = %d, want 1", cp.UnreadCountB)
	}

	// Idempotent replay must not duplicate the message.
	if err := svc.PersistPrivateFromJob(job.Payload); err != nil {
		t.Fatalf("PersistPrivateFromJob replay: %v", err)
	}
	if err := db.Model(&model.Message{}).Where("uuid = ?", dto.UUID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("replayed private job duplicated message, count=%d", count)
	}
}

func TestConversationService_SendDirect_RejectsSyncWriteOnDataPlane(t *testing.T) {
	svc, _ := setupConversationServiceTestDB(t)
	queue := &fakeQueue{err: errors.New("nats unavailable")}
	svc.SetJobQueue(queue)
	svc.SetSyncWriteAllowed(false)

	if _, err := svc.SendDirect("alice", "bob", "no fallback", ""); err == nil {
		t.Fatal("expected data-plane sync-write rejection")
	}
}
