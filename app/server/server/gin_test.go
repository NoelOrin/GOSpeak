package server

import (
	"reflect"
	"testing"

	"GOSpeak/internal/config"
)

func TestWSAllowedOriginsFallsBackToCORSOrigin(t *testing.T) {
	cfg := &config.Config{CORSOrigin: "*"}
	if got := wsAllowedOrigins(cfg); !reflect.DeepEqual(got, []string{"*"}) {
		t.Fatalf("expected CORS origin fallback, got %#v", got)
	}
}

func TestWSAllowedOriginsPrecedence(t *testing.T) {
	cfg := &config.Config{
		CORSOrigin:       "*",
		WSAllowedOrigins: "http://localhost:3000, https://voice.example.com",
	}
	if got := wsAllowedOrigins(cfg); !reflect.DeepEqual(got, []string{
		"http://localhost:3000",
		"https://voice.example.com",
	}) {
		t.Fatalf("expected explicit WS origins, got %#v", got)
	}
}
