package model

import (
	"testing"
)

func TestGuild_BeforeCreate(t *testing.T) {
	g := &Guild{Name: "Test"}
	if err := g.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if g.UUID == "" {
		t.Fatal("expected UUID to be auto-generated")
	}
	if len(g.InviteCode) != 8 {
		t.Fatalf("expected InviteCode len=8, got %d (%q)", len(g.InviteCode), g.InviteCode)
	}
}

func TestGuild_BeforeCreate_PreservesExistingUUID(t *testing.T) {
	existingUUID := "550e8400-e29b-41d4-a716-446655440000"
	g := &Guild{Name: "Test", UUID: existingUUID}
	if err := g.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if g.UUID != existingUUID {
		t.Fatalf("expected UUID preserved, got %q", g.UUID)
	}
}

func TestGuildMember_BeforeCreate(t *testing.T) {
	m := &GuildMember{}
	if m.ID != 0 {
		t.Fatal("expected ID=0 for new GuildMember")
	}
}

func TestGenerateInviteCode(t *testing.T) {
	code := generateInviteCode()
	if len(code) != 8 {
		t.Fatalf("expected len=8, got %d", len(code))
	}
	for _, c := range code {
		if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '9')) {
			t.Fatalf("unexpected char %c in invite code", c)
		}
	}
}

func TestGuild_TableName(t *testing.T) {
	g := &Guild{}
	if got := g.TableName(); got != "guilds" {
		t.Fatalf("expected guilds, got %q", got)
	}
}

func TestGuildMember_TableName(t *testing.T) {
	m := &GuildMember{}
	if got := m.TableName(); got != "guild_members" {
		t.Fatalf("expected guild_members, got %q", got)
	}
}
