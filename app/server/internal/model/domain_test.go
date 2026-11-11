package model

import (
	"testing"
)

func TestDomain_BeforeCreate(t *testing.T) {
	g := &Domain{Name: "Test"}
	if err := g.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if g.UUID == "" {
		t.Fatal("expected UUID to be auto-generated")
	}
}

func TestDomain_BeforeCreate_PreservesExistingUUID(t *testing.T) {
	existingUUID := "550e8400-e29b-41d4-a716-446655440000"
	g := &Domain{Name: "Test", UUID: existingUUID}
	if err := g.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if g.UUID != existingUUID {
		t.Fatalf("expected UUID preserved, got %q", g.UUID)
	}
}

func TestDomainMember_BeforeCreate(t *testing.T) {
	m := &DomainMember{}
	if m.ID != 0 {
		t.Fatal("expected ID=0 for new DomainMember")
	}
}

func TestDomain_TableName(t *testing.T) {
	g := &Domain{}
	if got := g.TableName(); got != "domains" {
		t.Fatalf("expected domains, got %q", got)
	}
}

func TestDomainMember_TableName(t *testing.T) {
	m := &DomainMember{}
	if got := m.TableName(); got != "domain_members" {
		t.Fatalf("expected domain_members, got %q", got)
	}
}
