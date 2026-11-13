package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

func TestValidateBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty clears the setting", "", false},
		{"blank clears the setting", "   ", false},
		{"https absolute", "https://gospeak.example.com", false},
		{"http absolute", "http://gospeak.example.com", false},
		{"explicit port and path", "https://gospeak.example.com:8443/gospeak", false},
		{"loopback with port", "http://127.0.0.1:8998", false},
		{"ipv6 loopback", "http://[::1]:8998", false},
		{"uppercase scheme", "HTTPS://GOSPEAK.EXAMPLE.COM", false},
		{"surrounding whitespace", "  https://gospeak.example.com  ", false},
		{"no scheme", "gospeak.example.com", true},
		{"scheme-like host and port", "gospeak.example.com:8998", true},
		{"relative path", "/gospeak", true},
		{"dot relative path", "../gospeak", true},
		{"empty host", "http://", true},
		{"ftp scheme", "ftp://gospeak.example.com", true},
		{"websocket scheme", "ws://gospeak.example.com", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"file scheme", "file:///etc/passwd", true},
		{"unparsable ipv6", "http://[::1", true},
		{"invalid escape", "http://%zz", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBaseURL(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("validateBaseURL(%q) = nil, want an error", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateBaseURL(%q) = %v, want nil", tc.in, err)
			}
		})
	}
}

func TestSetBaseURLRejectsInvalidURLAndKeepsState(t *testing.T) {
	const stored = "https://old.example.com"

	serverConfigMu.Lock()
	previous := serverBaseURL
	serverBaseURL = stored
	serverConfigMu.Unlock()
	t.Cleanup(func() {
		serverConfigMu.Lock()
		serverBaseURL = previous
		serverConfigMu.Unlock()
	})

	for _, in := range []string{
		"gospeak.example.com",
		"/relative/path",
		"ftp://gospeak.example.com",
		"ws://gospeak.example.com",
		"http://",
		"http://[::1",
	} {
		err := (ServerConfig{}).SetBaseURL(in)
		if err == nil {
			t.Fatalf("SetBaseURL(%q) = nil, want an error", in)
		}
		if got := currentBaseURL(); got != stored {
			t.Fatalf("SetBaseURL(%q) changed the base URL to %q, want %q", in, got, stored)
		}
	}
}

func TestSetBaseURLAcceptsEmpty(t *testing.T) {
	dir := t.TempDir()
	mygo.App.SetPath(mygo.PathUserData, dir)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathUserData, "") })

	serverConfigMu.Lock()
	previous := serverBaseURL
	serverBaseURL = ""
	serverConfigMu.Unlock()
	t.Cleanup(func() {
		serverConfigMu.Lock()
		serverBaseURL = previous
		serverConfigMu.Unlock()
	})

	if err := (ServerConfig{}).SetBaseURL(""); err != nil {
		t.Fatalf("SetBaseURL(\"\") = %v, want nil", err)
	}
	if got := currentBaseURL(); got != "" {
		t.Fatalf("currentBaseURL() = %q, want empty", got)
	}

	data, err := os.ReadFile(filepath.Join(dir, serverConfigFileName))
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	var cfg serverConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode persisted config: %v", err)
	}
	if cfg.BaseURL != "" {
		t.Fatalf("persisted baseUrl = %q, want empty", cfg.BaseURL)
	}
}

func TestSetBaseURLInvalidDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	mygo.App.SetPath(mygo.PathUserData, dir)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathUserData, "") })

	serverConfigMu.Lock()
	previous := serverBaseURL
	serverBaseURL = ""
	serverConfigMu.Unlock()
	t.Cleanup(func() {
		serverConfigMu.Lock()
		serverBaseURL = previous
		serverConfigMu.Unlock()
	})

	if err := (ServerConfig{}).SetBaseURL("not a url"); err == nil {
		t.Fatal("SetBaseURL(\"not a url\") = nil, want an error")
	}
	if _, err := os.Stat(filepath.Join(dir, serverConfigFileName)); !os.IsNotExist(err) {
		t.Fatalf("invalid URL wrote %s (stat err = %v)", serverConfigFileName, err)
	}
}
