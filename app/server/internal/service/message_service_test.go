package service

import (
	"context"
	"sync"
	"testing"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type memBus struct {
	mu      sync.Mutex
	room    string
	event   string
	payload interface{}
	calls   int
}

func (b *memBus) PublishRoom(_ context.Context, room, event string, payload interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.room, b.event, b.payload = room, event, payload
	return nil
}

func setupMsgSvc(t *testing.T) (*MessageService, *memBus) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:msg_svc?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&model.Message{})
	bus := &memBus{}
	svc := NewMessageService(repository.NewMessageRepository(db), bus)
	return svc, bus
}

func TestMessageService_Send_PublishesAndPersists(t *testing.T) {
	svc, bus := setupMsgSvc(t)
	dto, err := svc.Send(context.Background(), MessageSendInput{
		RoomKey:        "lobby",
		SenderIdentity: "alice",
		SenderDisplay:  "Alice",
		SenderRole:     "user",
		Content:        "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dto.ID == "" || dto.Content != "hello" {
		t.Fatalf("%+v", dto)
	}
	if bus.calls != 1 || bus.event != "message:new" || bus.room != "lobby" {
		t.Fatalf("bus=%+v", bus)
	}
	list, err := svc.List(context.Background(), "lobby", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Messages) != 1 || list.Messages[0].ID != dto.ID {
		t.Fatalf("%+v", list)
	}
}

func TestMessageService_Send_RejectsEmptyAndTooLong(t *testing.T) {
	svc, _ := setupMsgSvc(t)
	if _, err := svc.Send(context.Background(), MessageSendInput{RoomKey: "r", SenderIdentity: "a", Content: ""}); err == nil {
		t.Fatal("empty")
	}
	long := make([]rune, 501)
	for i := range long {
		long[i] = 'x'
	}
	if _, err := svc.Send(context.Background(), MessageSendInput{RoomKey: "r", SenderIdentity: "a", Content: string(long)}); err == nil {
		t.Fatal("long")
	}
	if utf8.RuneCountInString(string(long)) != 501 {
		t.Fatal("fixture")
	}
}
