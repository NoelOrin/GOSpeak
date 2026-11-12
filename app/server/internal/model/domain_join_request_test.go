package model

import (
	"testing"
	"time"
)

func TestDomainJoinRequest_StatusConstants(t *testing.T) {
	cases := map[string]string{
		JoinRequestPending:  "pending",
		JoinRequestApproved: "approved",
		JoinRequestRejected: "rejected",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("status constant = %q, want %q", got, want)
		}
	}
}

func TestDomainJoinRequest_IsPending(t *testing.T) {
	if !(&DomainJoinRequest{Status: JoinRequestPending}).IsPending() {
		t.Fatal("pending request must report pending")
	}
	if (&DomainJoinRequest{Status: JoinRequestApproved}).IsPending() {
		t.Fatal("approved request must not report pending")
	}
	if (*DomainJoinRequest)(nil).IsPending() {
		t.Fatal("nil request must not report pending")
	}
}

func TestDomainJoinRequest_TableName(t *testing.T) {
	if got := (DomainJoinRequest{}).TableName(); got != "domain_join_requests" {
		t.Fatalf("table name = %q, want domain_join_requests", got)
	}
}

func TestDomainJoinRequestInfo_EmbedsStatus(t *testing.T) {
	now := time.Now()
	info := DomainJoinRequestInfo{
		ID:         1,
		DomainUUID: "d1",
		UserUUID:   "u1",
		Status:     JoinRequestPending,
		CreatedAt:  now,
		Name:       "申请人",
	}
	if info.Status != JoinRequestPending || info.Name != "申请人" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestDomain_RequireJoinApprovalDefaultsFalse(t *testing.T) {
	d := &Domain{Name: "realm", OwnerUUID: "owner-1"}
	if d.RequireJoinApproval {
		t.Fatal("require_join_approval must default to false")
	}
}
