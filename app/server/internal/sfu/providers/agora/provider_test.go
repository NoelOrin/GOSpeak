package agora

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
var _ sfu.TimedMuteProvider = (*Service)(nil)
var _ sfu.MuteRuleStoreSetter = (*Service)(nil)

func TestUnsupportedOperations(t *testing.T) {
	svc := &Service{}
	if _, err := svc.GenerateAdminToken(); !errors.Is(err, pkg.ErrSFUNotSupported) {
		t.Fatalf("GenerateAdminToken: want ErrSFUNotSupported, got %v", err)
	}
}

func TestProviderName(t *testing.T) {
	if got := (&Service{}).ProviderName(); got != "agora" {
		t.Fatalf("ProviderName = %q, want agora", got)
	}
}

func TestCapabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if caps.MuteLevel != sfu.EnforcementDegraded || caps.KickLevel != sfu.EnforcementDegraded {
		t.Fatalf("unexpected levels: %+v", caps)
	}
	if !reflect.DeepEqual(caps, sfu.CapabilitiesFor("agora")) {
		t.Fatalf("Capabilities() = %+v, want %+v", caps, sfu.CapabilitiesFor("agora"))
	}
}

func TestMuteRuleKey(t *testing.T) {
	if got := muteRuleKey("r", "u"); got != "r|u" {
		t.Fatalf("key=%q", got)
	}
}

func TestMuteWithoutConfigFails(t *testing.T) {
	svc := &Service{}
	if err := svc.MuteParticipantTimed("room", "user", "", true, 60); err == nil {
		t.Fatal("expected error when unconfigured")
	}
	if err := svc.MuteParticipant("room", "user", "", true); err == nil {
		t.Fatal("expected error when unconfigured")
	}
	if err := svc.RemoveParticipant("room", "user"); err == nil {
		t.Fatal("expected error when unconfigured")
	}
	if err := svc.DeleteRoom("room"); err == nil {
		t.Fatal("expected error when unconfigured")
	}
}

func newAgoraTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Service{
		appID:          "app-1",
		customerID:     "cid",
		customerSecret: "secret",
		muteRules:      sfu.NewMemoryMuteRuleStore(),
		rest: &RESTClient{
			appID:          "app-1",
			customerID:     "cid",
			customerSecret: "secret",
			baseURL:        srv.URL,
			client:         http.DefaultClient,
		},
	}
}

func assertAppErrorCode(t *testing.T, err error, want pkg.ErrCode) {
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

func TestMuteParticipant_RESTError(t *testing.T) {
	svc := newAgoraTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	assertAppErrorCode(t, svc.MuteParticipant("room", "user", "", true), pkg.SFU_ERROR)
}

func TestMuteParticipant_RESTSuccess(t *testing.T) {
	svc := newAgoraTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/dev/v1/kicking-rule" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"rules": []interface{}{}})
			return
		}
		if r.URL.Path != "/dev/v1/kicking-rule" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 42})
	})
	if err := svc.MuteParticipant("room", "user", "", true); err != nil {
		t.Fatalf("MuteParticipant: %v", err)
	}
}

func TestRemoveParticipant_RESTError(t *testing.T) {
	svc := newAgoraTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	assertAppErrorCode(t, svc.RemoveParticipant("room", "user"), pkg.SFU_ERROR)
}

func TestRemoveParticipant_RESTSuccess(t *testing.T) {
	svc := newAgoraTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dev/v1/kicking-rule" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 7})
	})
	if err := svc.RemoveParticipant("room", "user"); err != nil {
		t.Fatalf("RemoveParticipant: %v", err)
	}
}

func TestDeleteRoom_RESTError(t *testing.T) {
	svc := newAgoraTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dev/v1/channel/app-1/room" || r.Method != http.MethodDelete {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	assertAppErrorCode(t, svc.DeleteRoom("room"), pkg.SFU_ERROR)
}

func TestDeleteRoom_RESTSuccess(t *testing.T) {
	svc := newAgoraTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dev/v1/channel/app-1/room" || r.Method != http.MethodDelete {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := svc.DeleteRoom("room"); err != nil {
		t.Fatalf("DeleteRoom: %v", err)
	}
}
