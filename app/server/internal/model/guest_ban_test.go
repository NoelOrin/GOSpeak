package model

import (
	"testing"
	"time"
)

func TestDomainGuestBan_IsActive(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	if !(&DomainGuestBan{ExpiresAt: nil}).IsActive() {
		t.Fatal("nil ExpiresAt must be active (permanent)")
	}
	if !(&DomainGuestBan{ExpiresAt: &future}).IsActive() {
		t.Fatal("future expiry must be active")
	}
	if (&DomainGuestBan{ExpiresAt: &past}).IsActive() {
		t.Fatal("past expiry must be inactive")
	}
}
