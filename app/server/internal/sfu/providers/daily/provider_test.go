package daily

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

var _ sfu.Provider = (*Service)(nil)
var _ sfu.ClientInfoProvider = (*Service)(nil)

func TestUnsupportedOperations(t *testing.T) {
	svc := &Service{}
	if err := svc.MuteParticipant("room", "user", "", true); !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("MuteParticipant: want ErrSFUNotSupported, got %v", err)
	}
}

func TestProviderName(t *testing.T) {
	if got := (&Service{}).ProviderName(); got != "daily" {
		t.Fatalf("ProviderName = %q, want daily", got)
	}
}

func TestCapabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if caps.ServerKick != true {
		t.Fatalf("ServerKick=%v, want true", caps.ServerKick)
	}
	if caps.ServerMute != false {
		t.Fatalf("ServerMute=%v, want false", caps.ServerMute)
	}
	if !reflect.DeepEqual(caps, sfu.CapabilitiesFor("daily")) {
		t.Fatalf("Capabilities() = %+v, want %+v", caps, sfu.CapabilitiesFor("daily"))
	}
}

func newDailyTestService(serverURL string) *Service {
	return &Service{
		apiKey: "key",
		client: &Client{
			baseURL: serverURL,
			apiKey:  "key",
			http:    http.DefaultClient,
		},
	}
}

func assertDailyAppErrorCode(t *testing.T, err error, want pkg.ErrCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %d, got nil", want)
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("error code = %d, want %d: %v", appErr.Code, want, err)
	}
}

func TestRemoveParticipant_ListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rooms/r1/presence" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	svc := newDailyTestService(server.URL)
	assertDailyAppErrorCode(t, svc.RemoveParticipant("r1", "alice"), pkg.SFU_ERROR)
}

func TestRemoveParticipant_RemoveError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rooms/r1/presence" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"participants": []map[string]interface{}{{"id": "pid-1", "user_name": "alice"}},
			})
			return
		}
		if r.URL.Path == "/rooms/r1/participants/pid-1/remove" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	svc := newDailyTestService(server.URL)
	assertDailyAppErrorCode(t, svc.RemoveParticipant("r1", "alice"), pkg.SFU_ERROR)
}

func TestDeleteRoom_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rooms/r1" || r.Method != http.MethodDelete {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	svc := newDailyTestService(server.URL)
	assertDailyAppErrorCode(t, svc.DeleteRoom("r1"), pkg.SFU_ERROR)
}
