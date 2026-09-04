package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
)

// These cover the store additions the backup feature relies on. They live here
// rather than in internal/backup because they are store behaviour, and because
// OpenNoMigrate's whole point is what it does NOT do -- which only a test
// inside this package can observe.

// TestOpenNoMigrateLeavesSchemaAlone is the load-bearing one. `gosplit backup`
// runs as a separate process against a database a live server may be serving,
// typically right before a version upgrade. If it migrated, taking a backup
// would mutate production as a side effect -- including re-running a blanket
// data migration like 0005.
func TestOpenNoMigrateLeavesSchemaAlone(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	cfg := &config.Config{DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite}

	st, err := OpenNoMigrate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenNoMigrate: %v", err)
	}
	defer func() { _ = st.Close() }()

	// No migrations ran, so there is no schema at all.
	applied, err := st.AppliedMigrations(context.Background())
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("OpenNoMigrate applied %d migration(s): %v", len(applied), applied)
	}
	// A query against a table that only a migration creates must fail, which
	// is what gives an operator an actionable error rather than silence.
	if _, err := st.DB.QueryContext(context.Background(), `SELECT id FROM users`); err == nil {
		t.Error("the users table exists, so migrations ran after all")
	}
}

// TestOpenMigratesAndOpenNoMigrateDoesNot runs both against the same file, so
// the difference is unambiguous.
func TestOpenMigratesAndOpenNoMigrateDoesNot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shared.db")
	cfg := &config.Config{DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite}

	migrated, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	applied, err := migrated.AppliedMigrations(context.Background())
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	embedded, err := migrated.EmbeddedMigrations()
	if err != nil {
		t.Fatalf("EmbeddedMigrations: %v", err)
	}
	_ = migrated.Close()

	if len(applied) != len(embedded) {
		t.Fatalf("Open applied %d of %d embedded migrations", len(applied), len(embedded))
	}
	if len(embedded) == 0 {
		t.Fatal("no embedded migrations found")
	}

	// Reopening without migrating sees the same set, and reports nothing
	// pending.
	plain, err := OpenNoMigrate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenNoMigrate: %v", err)
	}
	defer func() { _ = plain.Close() }()
	pending, err := plain.PendingMigrations(context.Background())
	if err != nil {
		t.Fatalf("PendingMigrations: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("a fully migrated database reports %d pending: %v", len(pending), pending)
	}
}

// TestPendingMigrationsOnEmptyDatabase asserts every embedded migration is
// reported as pending when nothing has been applied. This is what makes
// `gosplit backup` able to refuse rather than migrate.
func TestPendingMigrationsOnEmptyDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	cfg := &config.Config{DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite}

	st, err := OpenNoMigrate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenNoMigrate: %v", err)
	}
	defer func() { _ = st.Close() }()

	embedded, err := st.EmbeddedMigrations()
	if err != nil {
		t.Fatalf("EmbeddedMigrations: %v", err)
	}
	pending, err := st.PendingMigrations(context.Background())
	if err != nil {
		t.Fatalf("PendingMigrations: %v", err)
	}
	if len(pending) != len(embedded) {
		t.Errorf("pending = %d, want all %d embedded", len(pending), len(embedded))
	}
}

// TestEmbeddedMigrationsAreSortedAndNamed asserts the set is ordered and
// carries filenames, since the archive's compatibility gate compares exactly
// these strings.
func TestEmbeddedMigrationsAreSortedAndNamed(t *testing.T) {
	st := openTestStore(t)

	names, err := st.EmbeddedMigrations()
	if err != nil {
		t.Fatalf("EmbeddedMigrations: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no embedded migrations")
	}
	for i, n := range names {
		if !strings.HasSuffix(n, ".sql") {
			t.Errorf("migration %d is %q, want a .sql filename", i, n)
		}
		if i > 0 && names[i-1] >= n {
			t.Errorf("migrations are not sorted: %q then %q", names[i-1], n)
		}
	}
	if names[0] != "0001_init.sql" {
		t.Errorf("first migration is %q, want 0001_init.sql", names[0])
	}
}

// TestAppliedMigrationsMatchesEmbedded asserts a migrated database records
// exactly the filenames the binary carries -- the equality the restore gate
// depends on.
func TestAppliedMigrationsMatchesEmbedded(t *testing.T) {
	st := openTestStore(t)

	embedded, err := st.EmbeddedMigrations()
	if err != nil {
		t.Fatalf("EmbeddedMigrations: %v", err)
	}
	applied, err := st.AppliedMigrations(context.Background())
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if strings.Join(embedded, ",") != strings.Join(applied, ",") {
		t.Errorf("applied set differs from embedded:\n applied:  %v\n embedded: %v", applied, embedded)
	}
}

// TestRebindMatchesUnexported asserts the exported wrapper is the same
// rewriter internal callers use, so internal/backup cannot drift from it.
func TestRebindMatchesUnexported(t *testing.T) {
	for _, engine := range []config.Engine{config.EngineSQLite, config.EnginePostgres} {
		s := &Store{Engine: engine}
		for _, q := range []string{
			`SELECT 1`,
			`INSERT INTO t (a, b) VALUES (?, ?)`,
			`SELECT * FROM t WHERE a = ? AND b = ? OR c = ?`,
		} {
			if got, want := s.Rebind(q), s.rebind(q); got != want {
				t.Errorf("%s: Rebind(%q) = %q, want %q", engine, q, got, want)
			}
		}
	}
}

// TestRebindRewritesForPostgresOnly pins the actual behaviour, since the
// backup package builds every statement with `?` and relies on this.
func TestRebindRewritesForPostgresOnly(t *testing.T) {
	q := `INSERT INTO t (a, b, c) VALUES (?, ?, ?)`

	sqlite := (&Store{Engine: config.EngineSQLite}).Rebind(q)
	if sqlite != q {
		t.Errorf("SQLite rebind changed the query: %q", sqlite)
	}
	pg := (&Store{Engine: config.EnginePostgres}).Rebind(q)
	if !strings.Contains(pg, "$1") || !strings.Contains(pg, "$3") || strings.Contains(pg, "?") {
		t.Errorf("Postgres rebind = %q, want $1..$3 and no question marks", pg)
	}
}

// TestSQLiteReadOnlyDSN covers the second handle a dump takes. It must be
// read-only and must refuse the cases where a second handle is meaningless.
func TestSQLiteReadOnlyDSN(t *testing.T) {
	dsn := SQLiteReadOnlyDSN(&config.Config{
		Engine:      config.EngineSQLite,
		DatabaseURL: "file:./data/gosplit.db",
	})
	if dsn == "" {
		t.Fatal("a file-backed SQLite database should yield a read-only DSN")
	}
	if !strings.Contains(dsn, "mode=ro") {
		t.Errorf("DSN is not read-only: %q", dsn)
	}
	if strings.Contains(dsn, "journal_mode") {
		t.Errorf("a read-only handle must not set journal_mode: %q", dsn)
	}
	if !strings.Contains(dsn, "./data/gosplit.db") {
		t.Errorf("DSN lost the path: %q", dsn)
	}

	// Postgres pools freely, so it has no use for this.
	if got := SQLiteReadOnlyDSN(&config.Config{
		Engine:      config.EnginePostgres,
		DatabaseURL: "postgres://u:p@h/db",
	}); got != "" {
		t.Errorf("Postgres returned a SQLite DSN: %q", got)
	}
	// An in-memory database has no second handle on the same data.
	if got := SQLiteReadOnlyDSN(&config.Config{
		Engine:      config.EngineSQLite,
		DatabaseURL: "file::memory:",
	}); got != "" {
		t.Errorf("an in-memory database returned a DSN: %q", got)
	}
}

// TestSQLiteReadOnlyDSNIsUsable opens the real handle, so a malformed DSN
// cannot pass on string inspection alone.
func TestSQLiteReadOnlyDSNIsUsable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ro.db")
	cfg := &config.Config{DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite}

	st, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()

	dsn := SQLiteReadOnlyDSN(cfg)
	if dsn == "" {
		t.Fatal("no read-only DSN")
	}
	ro, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open read-only handle: %v", err)
	}
	defer func() { _ = ro.Close() }()

	// Reads work.
	var n int
	if err := ro.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("read through the read-only handle: %v", err)
	}
	// Writes do not, which is the guarantee that makes a dump safe to run
	// against a live database.
	if _, err := ro.Exec(`INSERT INTO app_metadata ("key", "value") VALUES ('x', 'y')`); err == nil {
		t.Error("the read-only handle accepted a write")
	}
}
