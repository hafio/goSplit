package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// testSecret is long enough to satisfy the SESSION_SECRET length rule.
const testSecret = "test-session-secret-0123456789abcdef"

// newTestRunner builds a Runner over a real temp-file SQLite store, in the same
// shape as store's own openTestStore helper (internal/store/golden_test.go).
// A file rather than :memory: on purpose: the dump takes a second read-only
// handle on the same path, which only a real file supports.
func newTestRunner(t *testing.T) (*Runner, *store.Store, *config.Config) {
	t.Helper()
	return newTestRunnerIn(t, t.TempDir())
}

// newTestRunnerIn is newTestRunner over a caller-chosen directory, so a test
// can stand up a SECOND, independent instance -- which is what restoring into
// a fresh database actually looks like.
func newTestRunnerIn(t *testing.T, dir string) (*Runner, *store.Store, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		DatabaseURL:              "file:" + filepath.Join(dir, "test.db"),
		Engine:                   config.EngineSQLite,
		SessionSecret:            testSecret,
		AppVersion:               "v0.0.0-test",
		UploadDir:                filepath.Join(dir, "uploads"),
		BackupDir:                filepath.Join(dir, "backups"),
		UploadMaxFileSizeMB:      5,
		RestoreMaxUploadMB:       500,
		RestoreMaxArchiveBytes:   1 << 30,
		RestoreMaxEntries:        10000,
		RestoreMaxTableFileBytes: 64 << 20,
		RestoreMaxJSONLLineBytes: 1 << 20,
	}
	st, err := store.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	r := New(st, cfg)
	// A fixed clock keeps archive names and timestamps deterministic.
	fixed := time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC)
	r.Now = func() time.Time { return fixed }
	return r, st, cfg
}

// exec runs a statement and fails the test on error.
func exec(t *testing.T, st *store.Store, query string, args ...any) {
	t.Helper()
	if _, err := st.DB.ExecContext(context.Background(), st.Rebind(query), args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// bigAmount is deliberately above 2^53, where a float64 round trip starts
// losing integer precision. Any encoding that passes money through
// encoding/json's default number handling corrupts this value.
const bigAmount int64 = 9007199254740993

// seedEverything puts at least one row in every registry table, choosing values
// that exercise the awkward cases: NULL versus empty string, an int64 beyond
// float64 precision, both boolean values, and JSON text whose key order and
// spacing must survive byte for byte.
//
// Rows go in through raw SQL rather than the service layer so the archive
// tables are covered too: archived_expenses and archived_expense_participants
// have no Go read path and no struct, so nothing above the store could seed or
// verify them.
func seedEverything(t *testing.T, st *store.Store) {
	t.Helper()

	// Two users: one with every nullable column set, one with them all NULL,
	// so a restore that confuses NULL with "" is caught.
	exec(t, st, `INSERT INTO users (id, name, email, email_verified, password_hash, image,
		currency, default_currency, preferred_language, role, deactivated_at, banking_id,
		hidden_friend_ids, created_at, theme_color)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		1, "Alice", "alice@example.com", "2026-01-01T00:00:00.000Z", "argon2-hash", "/uploads/a.png",
		"USD", "USD", "en", "ADMIN", nil, "bank-1",
		`{"b":2,"a":1}`, "2026-01-01T00:00:00.000Z", "burgundy")
	exec(t, st, `INSERT INTO users (id, name, email, email_verified, password_hash, image,
		currency, default_currency, preferred_language, role, deactivated_at, banking_id,
		hidden_friend_ids, created_at, theme_color)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		2, "", "bob@example.com", nil, nil, nil,
		"EUR", "EUR", "de", "USER", "2026-02-02T00:00:00.000Z", nil,
		`[]`, "2026-01-02T00:00:00.000Z", "burgundy")

	// One group with simplify_debts true, one false.
	exec(t, st, `INSERT INTO groups (id, public_id, name, image, created_by, default_currency,
		simplify_debts, archived_at, splitwise_group_id, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		1, "pub-1", "Trip", nil, 1, "USD", true, nil, nil, "2026-01-03T00:00:00.000Z")
	exec(t, st, `INSERT INTO groups (id, public_id, name, image, created_by, default_currency,
		simplify_debts, archived_at, splitwise_group_id, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		2, "pub-2", "Flat", "/uploads/g.png", 1, "EUR", false, "2026-03-03T00:00:00.000Z", "sw-9", "2026-01-04T00:00:00.000Z")

	exec(t, st, `INSERT INTO group_users (group_id, user_id) VALUES (?,?)`, 1, 1)
	exec(t, st, `INSERT INTO group_users (group_id, user_id) VALUES (?,?)`, 1, 2)
	exec(t, st, `INSERT INTO friendships (owner_id, friend_id, created_at) VALUES (?,?,?)`,
		1, 2, "2026-01-05T00:00:00.000Z")

	// The two tables with no Go code path at all.
	exec(t, st, `INSERT INTO group_default_splits (group_id, split_type, shares) VALUES (?,?,?)`,
		1, "EQUAL", `{"1":1,"2":1}`)
	exec(t, st, `INSERT INTO friend_default_splits (owner_id, friend_id, split_type, shares) VALUES (?,?,?,?)`,
		1, 2, "EXACT", `{"1":500}`)

	// An expense whose amount exceeds float64 integer precision, with the
	// nullable columns populated, and a second with them all NULL.
	exec(t, st, `INSERT INTO expenses (id, name, category, amount, split_type, expense_date,
		currency, paid_by, added_by, updated_by, group_id, file_key, transaction_id,
		recurrence_id, conversion_to_id, moved_from_id, deleted_at, deleted_by,
		created_at, updated_at, note, version)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"exp-1", "Hotel", "travel", bigAmount, "EQUAL", "2026-01-06",
		"USD", 1, 1, 2, 1, "/uploads/r.pdf", "tx-1",
		nil, nil, nil, nil, nil,
		"2026-01-06T00:00:00.000Z", "2026-01-06T00:00:00.000Z", "a note", 3)
	exec(t, st, `INSERT INTO expenses (id, name, category, amount, split_type, expense_date,
		currency, paid_by, added_by, updated_by, group_id, file_key, transaction_id,
		recurrence_id, conversion_to_id, moved_from_id, deleted_at, deleted_by,
		created_at, updated_at, note, version)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"exp-2", "Coffee", "general", -250, "EXACT", "2026-01-07",
		"EUR", 2, 2, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		"2026-01-07T00:00:00.000Z", "2026-01-07T00:00:00.000Z", "", 1)

	// Participant rows sum to zero per expense, the invariant every balance is
	// derived from.
	exec(t, st, `INSERT INTO expense_participants (expense_id, user_id, amount) VALUES (?,?,?)`,
		"exp-1", 1, bigAmount)
	exec(t, st, `INSERT INTO expense_participants (expense_id, user_id, amount) VALUES (?,?,?)`,
		"exp-1", 2, -bigAmount)
	exec(t, st, `INSERT INTO expense_participants (expense_id, user_id, amount) VALUES (?,?,?)`,
		"exp-2", 1, -250)
	exec(t, st, `INSERT INTO expense_participants (expense_id, user_id, amount) VALUES (?,?,?)`,
		"exp-2", 2, 250)

	// Awkward JSON: keys out of alphabetical order, interior spacing, a nested
	// object. Re-serializing this would change the bytes.
	exec(t, st, `INSERT INTO expense_split_inputs (expense_id, inputs) VALUES (?,?)`,
		"exp-1", `{"v":1,"method":"SHARE","values":{"2":3,  "1":1},"nested":{"z":[1,2]}}`)

	exec(t, st, `INSERT INTO expense_notes (id, expense_id, user_id, note, created_at) VALUES (?,?,?,?,?)`,
		1, "exp-1", 1, "orphan table row", "2026-01-08T00:00:00.000Z")

	exec(t, st, `INSERT INTO expense_recurrences (id, cron_expression, job_name,
		template_expense_id, notified, created_by, created_at, next_run_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		1, "0 4 * * *", "rec-1", "exp-1", true, 1, "2026-01-09T00:00:00.000Z", "2026-02-01T00:00:00.000Z")
	exec(t, st, `INSERT INTO expense_recurrences (id, cron_expression, job_name,
		template_expense_id, notified, created_by, created_at, next_run_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		2, "0 5 * * *", "rec-2", "exp-2", false, 2, "2026-01-10T00:00:00.000Z", nil)

	exec(t, st, `INSERT INTO sessions (token, user_id, expires, created_at) VALUES (?,?,?,?)`,
		"sess-1", 1, "2026-12-01T00:00:00.000Z", "2026-01-11T00:00:00.000Z")
	exec(t, st, `INSERT INTO verification_tokens (identifier, token, purpose, expires) VALUES (?,?,?,?)`,
		"alice@example.com", "vt-1", "magic", "2026-12-02T00:00:00.000Z")
	exec(t, st, `INSERT INTO push_notifications (user_id, endpoint, subscription) VALUES (?,?,?)`,
		1, "https://push.example/1", `{"keys":{"p256dh":"x","auth":"y"}}`)
	exec(t, st, `INSERT INTO cached_bank_data (user_id, data, updated_at) VALUES (?,?,?)`,
		1, `{"accounts":[{"id":"a1"}]}`, "2026-01-12T00:00:00.000Z")
	exec(t, st, `INSERT INTO cached_currency_rates (from_currency, to_currency, rate_date, rate) VALUES (?,?,?,?)`,
		"USD", "EUR", "2026-01-13", "0.923456")
	exec(t, st, `INSERT INTO scheduler_locks (id, holder, expires_at) VALUES (?,?,?)`,
		"cleanup", "host-1", "2026-01-14T00:00:00.000Z")
	exec(t, st, `INSERT INTO app_metadata ("key", "value") VALUES (?,?)`, "seeded", "yes")

	// The write-only archive tables. Nothing above the store layer can reach
	// these, so a struct-based dump would drop them without a word.
	exec(t, st, `INSERT INTO archived_expenses (id, name, category, amount, split_type,
		expense_date, currency, paid_by, added_by, updated_by, group_id, file_key,
		transaction_id, recurrence_id, conversion_to_id, moved_from_id, deleted_at,
		deleted_by, created_at, updated_at, note, archive_expense_id, archived_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"arc-1", "Old dinner", "food", 1234, "EQUAL", "2025-06-01",
		"USD", 1, 1, nil, 1, nil, nil, nil, nil, nil, nil, nil,
		"2025-06-01T00:00:00.000Z", "2025-06-01T00:00:00.000Z", "", "exp-1", "2026-01-15T00:00:00.000Z")
	exec(t, st, `INSERT INTO archived_expense_participants (expense_id, user_id, amount) VALUES (?,?,?)`,
		"arc-1", 1, 617)
	exec(t, st, `INSERT INTO archived_expense_participants (expense_id, user_id, amount) VALUES (?,?,?)`,
		"arc-1", 2, -617)
}

// seedUploads writes a small upload tree, including a nested directory.
func seedUploads(t *testing.T, cfg *config.Config) {
	t.Helper()
	files := map[string]string{
		"a.png":             "fake-png-bytes",
		"nested/r.pdf":      "fake-pdf-bytes",
		"nested/deep/g.png": "another",
	}
	for rel, body := range files {
		p := filepath.Join(cfg.UploadDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// snapshotTables reads every registry table into a comparable form, driven by
// the registry itself so a new column is covered automatically.
func snapshotTables(t *testing.T, st *store.Store) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, tbl := range InsertOrder() {
		cols := tbl.ColumnNames()
		q := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s",
			quoteIdents(cols), quoteIdent(tbl.Name), quoteIdents(tbl.PKColumns()))
		rows, err := st.DB.QueryContext(context.Background(), q)
		if err != nil {
			t.Fatalf("select from %s: %v", tbl.Name, err)
		}
		for rows.Next() {
			dest := make([]any, len(cols))
			for i := range dest {
				dest[i] = new(sql.NullString)
			}
			if err := rows.Scan(dest...); err != nil {
				_ = rows.Close()
				t.Fatalf("scan %s: %v", tbl.Name, err)
			}
			parts := make([]string, len(cols))
			for i, d := range dest {
				v := d.(*sql.NullString)
				if !v.Valid {
					parts[i] = cols[i] + "=<NULL>"
					continue
				}
				parts[i] = cols[i] + "=" + v.String
			}
			out[tbl.Name] = append(out[tbl.Name], fmt.Sprint(parts))
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			t.Fatalf("rows %s: %v", tbl.Name, err)
		}
		_ = rows.Close()
	}
	return out
}

// wipeAll empties every registry table EXCEPT schema_migrations, standing in
// for a database whose data is gone.
//
// schema_migrations is deliberately left alone. It is in the backup registry
// (R2 is every table verbatim), but clearing it here would model a state that
// cannot occur in practice: a database carrying the full schema while claiming
// no migrations have been applied. A genuinely lost database is a fresh volume,
// and store.Open re-applies every migration at boot before a restore is ever
// attempted -- so the live set is always populated. Wiping it made Validate's
// exact-match schema gate fire, which was the gate doing its job on an
// impossible input. TestRestoreIntoFreshDatabase covers the real case.
func wipeAll(t *testing.T, st *store.Store) {
	t.Helper()
	for _, tbl := range DeleteOrder() {
		if tbl.Name == "schema_migrations" {
			continue
		}
		exec(t, st, "DELETE FROM "+quoteIdent(tbl.Name))
	}
}

// countRows returns a table's row count.
func countRows(t *testing.T, st *store.Store, table string) int64 {
	t.Helper()
	var n int64
	if err := st.DB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM "+quoteIdent(table)).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
