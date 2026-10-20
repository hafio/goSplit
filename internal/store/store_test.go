package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
)

// TestOpenRollsBackAMigrationThatFails: a migration that cannot apply -- here
// 0001_init.sql meeting a table it creates -- fails Open with the file named,
// and leaves nothing of itself behind: none of its tables and no record of it,
// so once the cause is fixed the same migration applies cleanly.
func TestOpenRollsBackAMigrationThatFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	raw, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Exec(`CREATE TABLE friendships (x INTEGER)`); err != nil {
		t.Fatal(err)
	}

	_, err = Open(context.Background(), &config.Config{DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite})
	if err == nil || !strings.Contains(err.Error(), "0001_init.sql") {
		t.Fatalf("Open over a conflicting table = %v, want a failure naming 0001_init.sql", err)
	}
	var users, recorded int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := raw.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if users != 0 || recorded != 0 {
		t.Errorf("after the failed migration: users table %d, recorded migrations %d; want neither", users, recorded)
	}
}
