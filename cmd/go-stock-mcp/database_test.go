package main

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"go-stock/backend/db"
	"gorm.io/gorm"
)

func TestOpenReadOnlyDatabaseRejectsWrites(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stock.db")
	writable, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("create test database: %v", err)
	}
	if err := writable.Exec("CREATE TABLE sample (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create sample table: %v", err)
	}
	writableSQL, err := writable.DB()
	if err != nil {
		t.Fatalf("get writable sql db: %v", err)
	}
	if err := writableSQL.Close(); err != nil {
		t.Fatalf("close writable database: %v", err)
	}

	if err := openReadOnlyDatabase(dbPath); err != nil {
		t.Fatalf("openReadOnlyDatabase() error = %v", err)
	}
	readOnlySQL, err := db.Dao.DB()
	if err != nil {
		t.Fatalf("get read-only sql db: %v", err)
	}
	t.Cleanup(func() { _ = readOnlySQL.Close() })

	var count int64
	if err := db.Dao.Table("sample").Count(&count).Error; err != nil {
		t.Fatalf("read through read-only connection: %v", err)
	}
	if err := db.Dao.Exec("CREATE TABLE should_fail (id INTEGER)").Error; err == nil {
		t.Fatal("write unexpectedly succeeded through read-only connection")
	}
}
