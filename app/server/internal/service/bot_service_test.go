package service

import (
	"testing"
	"time"
)

func TestParseExpiryDuration_Days(t *testing.T) {
	d, err := parseExpiryDuration("30d")
	if err != nil {
		t.Fatalf("parse 30d: %v", err)
	}
	if d != 30*24*time.Hour {
		t.Fatalf("expected 30 days, got %v", d)
	}
}

func TestParseExpiryDuration_GoDuration(t *testing.T) {
	d, err := parseExpiryDuration("720h")
	if err != nil {
		t.Fatalf("parse 720h: %v", err)
	}
	if d != 720*time.Hour {
		t.Fatalf("expected 720h, got %v", d)
	}
}

func TestRandomHex_ReturnsHex(t *testing.T) {
	got, err := randomHex(16)
	if err != nil {
		t.Fatalf("randomHex: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("expected 32 hex chars for 16 bytes, got %d", len(got))
	}
	for _, c := range got {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			t.Fatalf("non-hex char %q in %q", c, got)
		}
	}
}
