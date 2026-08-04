package repository

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"GOSpeak/internal/config"
)

func TestSQLiteReadOnlyWorkerDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.db")
	writable, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open writable sqlite: %v", err)
	}
	if err := writable.Exec("CREATE TABLE IF NOT EXISTS worker_probe (id INTEGER)").Error; err != nil {
		t.Fatalf("seed writable sqlite: %v", err)
	}
	sqlDB, err := writable.DB()
	if err != nil {
		t.Fatalf("get sqlite conn: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close writable sqlite: %v", err)
	}

	cfg := &config.Config{
		DBType:      "SQLite",
		DBPath:      path,
		ClusterRole: "worker",
	}
	if err := InitDB(cfg); err != nil {
		t.Fatalf("InitDB worker: %v", err)
	}
	t.Cleanup(func() {
		if DB != nil {
			if sqlDB, err := DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})

	if err := DB.Exec("CREATE TABLE forbidden (id INTEGER)").Error; err == nil {
		t.Fatal("expected read-only SQLite write to fail")
	}
}
