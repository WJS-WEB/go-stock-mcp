package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"go-stock/backend/db"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// openReadOnlyDatabase opens the existing go-stock database without running
// AutoMigrate or the application's cleanup goroutine. The MCP process is a
// read-only facade and must not change the user's database at startup.
func openReadOnlyDatabase(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("database path is empty")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat database %q: %w", absPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("database path is a directory: %s", absPath)
	}

	// go-sqlite accepts SQLite URI filenames. Use mode=ro so even an
	// accidentally exposed write tool cannot mutate the configured database.
	dsn := "file:" + filepath.ToSlash(absPath) + "?mode=ro&_pragma=busy_timeout(10000)"
	dbLogger := gormLogger.New(
		log.New(os.Stderr, "", log.LstdFlags),
		gormLogger.Config{
			SlowThreshold:             3 * time.Second,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			LogLevel:                  gormLogger.Silent,
		},
	)

	opened, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                                   dbLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
	})
	if err != nil {
		return fmt.Errorf("open read-only database: %w", err)
	}

	sqlDB, err := opened.DB()
	if err != nil {
		return fmt.Errorf("get sql database: %w", err)
	}
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	db.Dao = opened
	return nil
}
