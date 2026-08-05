package signal

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/service"
)

type serviceNotFoundRoomStore struct {
	mockRoomStore
}

func (m *serviceNotFoundRoomStore) GetByDomainAndName(domainUUID, name string) (*model.Room, error) {
	return nil, service.ErrRoomNotFound
}

func TestHub_CheckRoomLimit_ServiceNotFoundIsNotDBFailure(t *testing.T) {
	hub := NewHub(&serviceNotFoundRoomStore{}, nil, nil, nil)

	full, limit, count, err := hub.CheckRoomLimit("domain-a", "missing-room")
	if err != nil {
		t.Fatalf("service room not found should be treated as absent, got err=%v", err)
	}
	if full || limit != 0 || count != 0 {
		t.Fatalf("expected empty unlimited room, got full=%v limit=%d count=%d", full, limit, count)
	}
}
