package srs

import (
	"strings"
	"testing"
)

func TestGenerateStreamName_Deterministic(t *testing.T) {
	a := GenerateStreamName("room-1", "alice")
	b := GenerateStreamName("room-1", "alice")
	if a != b {
		t.Fatalf("same input should produce same stream: %q vs %q", a, b)
	}
}

func TestGenerateStreamName_DifferentInput(t *testing.T) {
	a := GenerateStreamName("room-1", "alice")
	b := GenerateStreamName("room-1", "bob")
	c := GenerateStreamName("room-2", "alice")
	if a == b {
		t.Fatal("different identity should produce different stream")
	}
	if a == c {
		t.Fatal("different room should produce different stream")
	}
}

func TestGenerateStreamName_Format(t *testing.T) {
	s := GenerateStreamName("room-1", "alice")
	if !strings.HasPrefix(s, "gs-") {
		t.Fatalf("stream should have gs- prefix: %q", s)
	}
	if len(s) != 3+12 {
		t.Fatalf("stream should be gs- + 12 chars, got len %d: %q", len(s), s)
	}
	for _, r := range s[3:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z')) {
			t.Fatalf("stream suffix must be base36 [0-9a-z], got %q in %q", r, s)
		}
	}
}

func TestGenerateStreamName_ASCIISafe(t *testing.T) {
	s := GenerateStreamName("聊天室", "张三")
	for _, r := range s {
		if r > 127 {
			t.Fatalf("stream must be ASCII-safe, got non-ASCII %q in %q", r, s)
		}
	}
}

func TestGenerateStreamToken_AndValidate(t *testing.T) {
	stream := GenerateStreamName("room-1", "alice")
	secret := "deadbeef"
	tok, err := GenerateStreamToken(stream, secret)
	if err != nil {
		t.Fatalf("expected token, got error: %v", err)
	}
	if tok == "" {
		t.Fatal("token should not be empty")
	}
	if !ValidateStreamToken(stream, tok, secret) {
		t.Fatal("valid token should validate")
	}
}

func TestGenerateStreamToken_EmptySecretRejected(t *testing.T) {
	stream := GenerateStreamName("room-1", "alice")
	tok, err := GenerateStreamToken(stream, "")
	if err == nil {
		t.Fatalf("empty secret should error, got token %q", tok)
	}
	if tok != "" {
		t.Fatalf("expected empty token on error, got %q", tok)
	}
	if ValidateStreamToken(stream, "anything", "") {
		t.Fatal("empty secret should never validate")
	}
}

func TestValidateStreamToken_WrongSecret(t *testing.T) {
	stream := GenerateStreamName("room-1", "alice")
	tok, _ := GenerateStreamToken(stream, "secret-a")
	if ValidateStreamToken(stream, tok, "secret-b") {
		t.Fatal("token with wrong secret should not validate")
	}
}

func TestValidateStreamToken_WrongStream(t *testing.T) {
	tok, _ := GenerateStreamToken("gs-aaa", "secret")
	if ValidateStreamToken("gs-bbb", tok, "secret") {
		t.Fatal("token bound to different stream should not validate")
	}
}

func TestValidateStreamToken_WrongAlgRejected(t *testing.T) {
	// ValidateStreamToken only accepts HS256 JWT bound to stream.
	if ValidateStreamToken("gs-aaa", "not-a-jwt", "secret") {
		t.Fatal("non-jwt token should not validate")
	}
}
