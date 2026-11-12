package livekit

import (
	"reflect"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

var _ sfu.Provider = (*Service)(nil)

func TestProviderName(t *testing.T) {
	if got := (&Service{}).ProviderName(); got != "livekit" {
		t.Fatalf("ProviderName = %q, want livekit", got)
	}
}

func TestCapabilities(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if caps.ServerKick != true {
		t.Fatalf("ServerKick=%v, want true", caps.ServerKick)
	}
	if caps.ServerMute != true {
		t.Fatalf("ServerMute=%v, want true", caps.ServerMute)
	}
	if !reflect.DeepEqual(caps, sfu.CapabilitiesFor("livekit")) {
		t.Fatalf("Capabilities() = %+v, want %+v", caps, sfu.CapabilitiesFor("livekit"))
	}
}

func TestUnconfiguredOperations(t *testing.T) {
	svc := &Service{}
	if err := svc.MuteParticipant("room", "user", "track", true); err == nil {
		t.Fatal("expected MuteParticipant error when unconfigured")
	} else if appErr, ok := err.(*pkg.AppError); !ok || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("MuteParticipant error = %v, want SFU_NOT_CONFIGURED", err)
	}
	if err := svc.RemoveParticipant("room", "user"); err == nil {
		t.Fatal("expected RemoveParticipant error when unconfigured")
	} else if appErr, ok := err.(*pkg.AppError); !ok || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("RemoveParticipant error = %v, want SFU_NOT_CONFIGURED", err)
	}
	if err := svc.DeleteRoom("room"); err == nil {
		t.Fatal("expected DeleteRoom error when unconfigured")
	} else if appErr, ok := err.(*pkg.AppError); !ok || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("DeleteRoom error = %v, want SFU_NOT_CONFIGURED", err)
	}
}

// GetHost 必须返回下发给浏览器的地址，而不是后端 admin API 用的内网地址。
// 容器部署时二者不同：浏览器无法解析 ws://livekit:7880 这类服务名。
func TestGetHostReturnsPublicURL(t *testing.T) {
	svc := NewService(&config.Config{
		LiveKitHost:      "ws://livekit:7880",
		LiveKitPublicURL: "wss://lk.example.com",
		LiveKitKey:       "key",
		LiveKitSecret:    "secret",
	})
	if got := svc.GetHost(); got != "wss://lk.example.com" {
		t.Fatalf("GetHost = %q, want public url", got)
	}
}

// 未配置 public url 时 GetHost 必须返回空，而不是回退到 LIVEKIT_HOST：
// 容器内网地址（ws://livekit:7880）浏览器解析不了，回退只会让连接失败。
// 空值由 handler 按请求 Host 推导同源 /livekit 地址。
func TestGetHostEmptyWithoutPublicURL(t *testing.T) {
	svc := NewService(&config.Config{
		LiveKitHost: "ws://livekit:7880",
		LiveKitKey:  "key",
	})
	if got := svc.GetHost(); got != "" {
		t.Fatalf("GetHost = %q, want empty (no fallback to internal host)", got)
	}
}
