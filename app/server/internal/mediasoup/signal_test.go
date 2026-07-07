package mediasoup

import (
	"encoding/json"
	"sync"
	"testing"
)

type stubBridge struct {
	mu       sync.Mutex
	closedId string
	closed   chan struct{}
}

func (s *stubBridge) GetRouterCapabilities(roomID string) (json.RawMessage, error) {
	return nil, nil
}

func (s *stubBridge) CreateTransport(roomID, identity, direction string) (*TransportParams, error) {
	return nil, nil
}

func (s *stubBridge) ConnectTransport(roomID, transportID string, dtlsParameters json.RawMessage) error {
	return nil
}

func (s *stubBridge) Produce(roomID, transportID, kind string, rtpParameters, appData json.RawMessage) (*ProduceResult, error) {
	return nil, nil
}

func (s *stubBridge) Consume(roomID, transportID, producerID string, rtpCapabilities json.RawMessage) (*ConsumeResult, error) {
	return nil, nil
}

func (s *stubBridge) CloseParticipant(room, identity string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closedId = identity
	if s.closed != nil {
		s.closed <- struct{}{}
	}
	return nil, nil
}

func TestOnParticipantLeft_BroadcastsAndCloses(t *testing.T) {
	var (
		mu      sync.Mutex
		gotRoom string
		gotEvt  string
		gotId   string
	)
	bcast := func(room, event string, data interface{}) {
		mu.Lock()
		defer mu.Unlock()
		gotRoom = room
		gotEvt = event
		gotId = data.(map[string]interface{})["identity"].(string)
	}
	stub := &stubBridge{closed: make(chan struct{}, 1)}
	sig := &MediasoupSignal{bridge: stub, broadcast: bcast}

	sig.OnParticipantLeft("r1", "alice")

	<-stub.closed
	if stub.closedId != "alice" {
		t.Fatalf("CloseParticipant called with %q, want alice", stub.closedId)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotRoom != "r1" || gotEvt != "sfu:producer-closed" || gotId != "alice" {
		t.Fatalf("broadcast got room=%q evt=%q id=%q", gotRoom, gotEvt, gotId)
	}
}

func TestOnParticipantLeft_DedupsSecondCall(t *testing.T) {
	var (
		mu      sync.Mutex
		bcasts  int
	)
	bcast := func(room, event string, data interface{}) {
		mu.Lock()
		bcasts++
		mu.Unlock()
	}
	stub := &stubBridge{closed: make(chan struct{}, 4)}
	sig := &MediasoupSignal{bridge: stub, broadcast: bcast}

	sig.OnParticipantLeft("r1", "alice")
	<-stub.closed
	sig.OnParticipantLeft("r1", "alice")

	stub.mu.Lock()
	stub.closedId = ""
	stub.mu.Unlock()

	mu.Lock()
	defer mu.Unlock()
	if bcasts != 1 {
		t.Fatalf("expected 1 broadcast, got %d", bcasts)
	}
}

func TestOnParticipantLeft_RejoinClearsDedup(t *testing.T) {
	var (
		mu     sync.Mutex
		bcasts int
	)
	bcast := func(room, event string, data interface{}) {
		mu.Lock()
		bcasts++
		mu.Unlock()
	}
	stub := &stubBridge{closed: make(chan struct{}, 4)}
	sig := &MediasoupSignal{bridge: stub, broadcast: bcast}

	sig.OnParticipantLeft("r1", "alice")
	<-stub.closed

	mu.Lock()
	bcasts = 0
	mu.Unlock()

	sig.recentClose.Delete("r1\x00alice")
	sig.OnParticipantLeft("r1", "alice")
	<-stub.closed

	mu.Lock()
	defer mu.Unlock()
	if bcasts != 1 {
		t.Fatalf("after rejoin-clear, expected 1 broadcast, got %d", bcasts)
	}
}
