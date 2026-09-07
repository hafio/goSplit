// Package store is the data-access layer. It targets both SQLite (default,
// pure-Go modernc.org/sqlite) and PostgreSQL from one codebase: queries are
// written with `?` placeholders and rebound to `$n` for Postgres. Timestamps
// are ISO-8601 UTC strings on both engines (a single Go scanning path).
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hafio/gosplit/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib" // "pgx" driver
	_ "modernc.org/sqlite"             // "sqlite" driver
)

//go:embed migrations/sqlite/*.sql migrations/postgres/*.sql
var migrationsFS embed.FS

// Store wraps the database handle and remembers which engine is in use.
type Store struct {
	DB     *sql.DB
	Engine config.Engine
}

// Open connects to the configured database, applies PRAGMAs (SQLite), and runs
// migrations. The caller owns Close.
func Open(ctx context.Context, cfg *config.Config) (*Store, error) {
	s, err := connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := s.migrate(ctx); err != nil {
		_ = s.DB.Close()
		return nil, err
	}
	return s, nil
}

// OpenNoMigrate connects exactly as Open does but never applies migrations.
// It exists for out-of-band commands (backup, and a restore's read-only
// validate pass) that run as a separate process against a database a live
// server may still be serving: Open migrates unconditionally, so a bare
// `gosplit backup` -- typically run right before a version upgrade -- would
// otherwise apply every pending migration as a side effect of taking a backup,
// with no confirmation. Callers hitting a never-initialized database get an
// actionable error from their first query rather than a silent migration.
func OpenNoMigrate(ctx context.Context, cfg *config.Config) (*Store, error) {
	return connect(ctx, cfg)
}

// connect opens and pings the configured database. Shared by Open and
// OpenNoMigrate; it never migrates.
func connect(ctx context.Context, cfg *config.Config) (*Store, error) {
	var (
		db  *sql.DB
		err error
	)
	switch cfg.Engine {
	case config.EngineSQLite:
		dsn := sqliteDSN(cfg.DatabaseURL)
		if dir := sqliteDir(dsn); dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		db, err = sql.Open("sqlite", dsn)
		if err == nil {
			// SQLite is single-writer; one connection avoids "database is locked".
			db.SetMaxOpenConns(1)
		}
	case config.EnginePostgres:
		db, err = sql.Open("pgx", cfg.DatabaseURL)
	default:
		return nil, fmt.Errorf("store: unknown engine %q", cfg.Engine)
	}
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{DB: db, Engine: cfg.Engine}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.DB.Close() }

// SQLiteReadOnlyDSN builds a DSN for a second, read-only handle on the same
// file as cfg.DatabaseURL. A dump needs one: the server's pool is pinned to a
// single connection (see connect), so holding a long read transaction on it
// would stall every request, while WAL lets an independent reader run without
// blocking the writer. Returns "" for a non-SQLite engine or an in-memory
// database, neither of which supports a second handle on the same data.
func SQLiteReadOnlyDSN(cfg *config.Config) string {
	if cfg.Engine != config.EngineSQLite {
		return ""
	}
	path := sqlitePath(cfg.DatabaseURL)
	if path == "" || strings.HasPrefix(path, ":memory:") {
		return ""
	}
	return "file:" + path +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&mode=ro"
}

// SQLiteDataDir reports the directory holding the SQLite database file, so
// startup can create it and `gosplit paths` can report on it. Returns "" for a
// non-SQLite engine or an in-memory database, neither of which has one.
func SQLiteDataDir(cfg *config.Config) string {
	if cfg.Engine != config.EngineSQLite {
		return ""
	}
	path := sqlitePath(cfg.DatabaseURL)
	if path == "" || strings.HasPrefix(path, ":memory:") {
		return ""
	}
	return filepath.Dir(path)
}

// sqlitePath strips the scheme and any query from a SQLite DATABASE_URL,
// leaving the bare file path. The query goes so that callers control the
// pragmas themselves.
func sqlitePath(url string) string {
	path := url
	switch {
	case strings.HasPrefix(url, "file:"):
		path = strings.TrimPrefix(url, "file:")
	case strings.HasPrefix(url, "sqlite://"):
		path = strings.TrimPrefix(url, "sqlite://")
	case url == "":
		path = "./data/gosplit.db"
	}
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return path
}

// sqliteDSN normalizes a DATABASE_URL into a modernc.org/sqlite DSN with WAL,
// a busy timeout, and foreign keys enabled.
func sqliteDSN(url string) string {
	return "file:" + sqlitePath(url) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)"
}

// sqliteDir extracts the directory of the SQLite file from a DSN, so it can be
// created before opening. Returns "" for in-memory or dir-less paths.
func sqliteDir(dsn string) string {
	path := strings.TrimPrefix(dsn, "file:")
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if path == "" || strings.HasPrefix(path, ":memory:") {
		return ""
	}
	return filepath.Dir(path)
}

// Rebind exposes rebind to packages outside store (internal/backup builds one
// portable `?`-style statement per table and runs it on either engine, rather
// than carrying a second copy of the placeholder rule).
func (s *Store) Rebind(query string) string { return s.rebind(query) }

// rebind converts `?` placeholders to `$1,$2,...` for Postgres; SQLite keeps
// `?`. Callers always write portable `?`-style SQL.
func (s *Store) rebind(query string) string {
	if s.Engine != config.EnginePostgres {
		return query
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(fmt.Sprintf("%d", n))
		} else {
			b.WriteByte(query[i])
		}
	}
	return b.String()
}

// EmbeddedMigrations returns the migration filenames this build carries for
// the active engine, sorted. Exported so out-of-band commands can tell whether
// a database is behind the binary without applying anything.
func (s *Store) EmbeddedMigrations() ([]string, error) {
	dir := "migrations/" + string(s.Engine)
	entries, err := migrationsFS.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("store: read migrations %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// AppliedMigrations returns the migration filenames recorded in the database,
// sorted. It returns no error when schema_migrations does not exist yet: a
// never-initialized database has simply applied nothing.
func (s *Store) AppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// PendingMigrations returns the migrations this build carries that the
// database has not recorded, sorted.
func (s *Store) PendingMigrations(ctx context.Context) ([]string, error) {
	embedded, err := s.EmbeddedMigrations()
	if err != nil {
		return nil, err
	}
	applied, err := s.AppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(applied))
	for _, v := range applied {
		have[v] = true
	}
	var pending []string
	for _, v := range embedded {
		if !have[v] {
			pending = append(pending, v)
		}
	}
	return pending, nil
}

// migrate applies embedded migrations for the active engine that have not yet
// been recorded in schema_migrations. Each migration runs in its own tx.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`,
	); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	dir := "migrations/" + string(s.Engine)
	entries, err := migrationsFS.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("store: read migrations %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	applied := map[string]bool{}
	rows, err := s.DB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("store: read applied migrations: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	for _, name := range names {
		if applied[name] {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile(dir + "/" + name)
		if err != nil {
			return fmt.Errorf("store: read %s: %w", name, err)
		}
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			s.rebind(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`),
			name, nowISO(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit %s: %w", name, err)
		}
	}
	return nil
}
