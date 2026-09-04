package pkg

import "testing"

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret(""); got != "" {
		t.Fatalf("empty = %q", got)
	}
	if got := MaskSecret("abc"); got != "***" {
		t.Fatalf("short = %q", got)
	}
	if got := MaskSecret("abcdefgh"); got != "ab***fgh" {
		t.Fatalf("long = %q", got)
	}
}

func TestKeepSecret(t *testing.T) {
	if got := KeepSecret("", "old"); got != "old" {
		t.Fatalf("keep existing = %q", got)
	}
	if got := KeepSecret("new", "old"); got != "new" {
		t.Fatalf("override = %q", got)
	}
}
