package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestReproDropLegacy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// 手工建一张带旧列的表（模拟生产老库）
	if err := db.Exec(`CREATE TABLE domains (
		id integer primary key, uuid text, name text, icon_url text, description text,
		owner_uuid text, invite_code text, is_public numeric,
		allow_guest numeric, guest_can_listen numeric, guest_can_speak numeric,
		guest_can_message numeric, guest_limit integer, created_at datetime, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE users (id integer primary key, uuid text, name text, is_guest numeric)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE domain_guest_bans (id integer primary key, domain_uuid text, user_uuid text)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := dropLegacyGuestSchema(db); err != nil {
		t.Fatalf("dropLegacyGuestSchema: %v", err)
	}
	t.Log("OK")
}
