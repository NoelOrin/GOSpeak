package mediasoup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMockWorker(t *testing.T, handler http.HandlerFunc) *BridgeClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewBridgeClient(srv.URL)
}

func TestListParticipants(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/participants", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"participants": []map[string]interface{}{
				{"identity": "alice", "producerCount": 1, "hasSendTransport": true, "hasRecvTransport": true},
			},
		})
	})
	b := newMockWorker(t, mux.ServeHTTP)

	got, err := b.ListParticipants("r1")
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(got) != 1 || got[0].Identity != "alice" || got[0].ProducerCount != 1 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestCloseParticipant(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/participants/alice/close", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "closedProducerIds": []string{"p1"}})
	})
	b := newMockWorker(t, mux.ServeHTTP)

	got, err := b.CloseParticipant("r1", "alice")
	if err != nil {
		t.Fatalf("CloseParticipant: %v", err)
	}
	if len(got) != 1 || got[0] != "p1" {
		t.Fatalf("unexpected closedProducerIds: %v", got)
	}
}

func TestCloseParticipant_NotFoundIsNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/participants/ghost/close", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"participant not found"}`))
	})
	b := newMockWorker(t, mux.ServeHTTP)

	got, err := b.CloseParticipant("r1", "ghost")
	if err != nil {
		t.Fatalf("404 should map to nil error, got: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil closedProducerIds, got %v", got)
	}
}

func TestPauseProducer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms/r1/producers/p1/pause", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	b := newMockWorker(t, mux.ServeHTTP)

	if err := b.PauseProducer("r1", "p1"); err != nil {
		t.Fatalf("PauseProducer: %v", err)
	}
}
