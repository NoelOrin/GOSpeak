package service

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func testInviteKey() []byte {
	return DeriveInviteKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "jwt-fallback")
}

func TestDeriveInviteKey(t *testing.T) {
	withStorage := DeriveInviteKey("storage-key", "jwt-key")
	withJWT := DeriveInviteKey("", "jwt-key")

	if len(withStorage) != 32 {
		t.Fatalf("derived key length = %d, want 32", len(withStorage))
	}
	if string(withStorage) == string(withJWT) {
		t.Fatal("storage key and jwt fallback must derive different keys")
	}
	if string(DeriveInviteKey("storage-key", "other")) != string(withStorage) {
		t.Fatal("derivation must not depend on jwt key when storage key is set")
	}
	if string(DeriveInviteKey("", "a")) == string(DeriveInviteKey("", "b")) {
		t.Fatal("jwt fallback must depend on jwt key")
	}
}

func TestEncodeDecodeInviteCode_RoundTrip(t *testing.T) {
	key := testInviteKey()
	expires := time.Unix(1900000000, 0).UTC()
	code := InviteCode{
		DomainPrefix: DomainPrefixOf("domain-uuid-1"),
		InviteID:     42,
		ExpiresAt:    expires,
		MaxUses:      7,
	}

	raw, err := EncodeInviteCode(key, code)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(raw) != inviteCodeLength {
		t.Fatalf("code length = %d, want %d", len(raw), inviteCodeLength)
	}
	for _, r := range raw {
		if !strings.ContainsRune(inviteAlphabet, r) {
			t.Fatalf("code contains char %q outside alphabet", r)
		}
	}

	got, err := DecodeInviteCode(key, raw, time.Unix(1800000000, 0))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.InviteID != code.InviteID {
		t.Fatalf("invite id = %d, want %d", got.InviteID, code.InviteID)
	}
	if got.DomainPrefix != code.DomainPrefix {
		t.Fatal("domain prefix mismatch")
	}
	if got.MaxUses != code.MaxUses {
		t.Fatalf("max uses = %d, want %d", got.MaxUses, code.MaxUses)
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Fatalf("expires at = %v, want %v", got.ExpiresAt, expires)
	}
}

func TestEncodeDecodeInviteCode_NeverExpires(t *testing.T) {
	key := testInviteKey()
	raw, err := EncodeInviteCode(key, InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeInviteCode(key, raw, time.Unix(4000000000, 0))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("expires at = %v, want zero", got.ExpiresAt)
	}
}

func TestDecodeInviteCode_RejectsMalformed(t *testing.T) {
	key := testInviteKey()
	cases := map[string]string{
		"empty":     "",
		"too short": "ABCDEFGH",
		"too long":  strings.Repeat("A", inviteCodeLength+1),
		"bad chars": strings.Repeat("I", inviteCodeLength),
		"lowercase": strings.Repeat("a", inviteCodeLength),
		"space":     " " + strings.Repeat("A", inviteCodeLength-1),
	}
	for name, raw := range cases {
		if _, err := DecodeInviteCode(key, raw, time.Now()); !errors.Is(err, ErrInviteCodeMalformed) {
			t.Fatalf("%s: err = %v, want ErrInviteCodeMalformed", name, err)
		}
	}
}

func TestDecodeInviteCode_RejectsTamperedPayload(t *testing.T) {
	key := testInviteKey()
	raw, err := EncodeInviteCode(key, InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 9, MaxUses: 1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	tampered := []byte(raw)
	if tampered[0] == inviteAlphabet[0] {
		tampered[0] = inviteAlphabet[1]
	} else {
		tampered[0] = inviteAlphabet[0]
	}
	if _, err := DecodeInviteCode(key, string(tampered), time.Now()); !errors.Is(err, ErrInviteCodeInvalid) {
		t.Fatalf("err = %v, want ErrInviteCodeInvalid", err)
	}
}

func TestDecodeInviteCode_RejectsWrongKey(t *testing.T) {
	raw, err := EncodeInviteCode(testInviteKey(), InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 3})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	other := DeriveInviteKey("another-key", "jwt")
	if _, err := DecodeInviteCode(other, raw, time.Now()); !errors.Is(err, ErrInviteCodeInvalid) {
		t.Fatalf("err = %v, want ErrInviteCodeInvalid", err)
	}
}

func TestDecodeInviteCode_RejectsExpired(t *testing.T) {
	key := testInviteKey()
	raw, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf("d"),
		InviteID:     5,
		ExpiresAt:    time.Unix(1000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := DecodeInviteCode(key, raw, time.Unix(1001, 0)); !errors.Is(err, ErrInviteCodeExpired) {
		t.Fatalf("err = %v, want ErrInviteCodeExpired", err)
	}
	if _, err := DecodeInviteCode(key, raw, time.Unix(1000, 0)); err != nil {
		t.Fatalf("expiry boundary must be inclusive: %v", err)
	}
}

func TestEncodeInviteCode_MaxUsesBoundaries(t *testing.T) {
	key := testInviteKey()
	// MaxUses 是 uint16，上限由类型本身保证；这里只验证边界值可正常往返。
	for _, maxUses := range []uint16{0, 1, InviteMaxUsesLimit} {
		raw, err := EncodeInviteCode(key, InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 1, MaxUses: maxUses})
		if err != nil {
			t.Fatalf("max uses %d: encode: %v", maxUses, err)
		}
		got, err := DecodeInviteCode(key, raw, time.Now())
		if err != nil {
			t.Fatalf("max uses %d: decode: %v", maxUses, err)
		}
		if got.MaxUses != maxUses {
			t.Fatalf("max uses round trip = %d, want %d", got.MaxUses, maxUses)
		}
	}
}

func TestDomainPrefixOf_StableAndDistinct(t *testing.T) {
	a1 := DomainPrefixOf("domain-a")
	a2 := DomainPrefixOf("domain-a")
	b := DomainPrefixOf("domain-b")
	if a1 != a2 {
		t.Fatal("prefix must be deterministic")
	}
	if a1 == b {
		t.Fatal("different domains must produce different prefixes")
	}
}
