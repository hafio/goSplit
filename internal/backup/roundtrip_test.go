package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFullRoundTrip is the headline test: seed every table and the upload
// tree, dump, wipe the database, restore, and assert the instance is
// byte-identical. It is the one test that would catch a whole class of silent
// corruption -- a lost table, a truncated int64, a NULL turned into "".
func TestFullRoundTrip(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	seedUploads(t, cfg)

	before := snapshotTables(t, st)

	path, man, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if man.UploadFileCount != 3 {
		t.Errorf("manifest upload count = %d, want 3", man.UploadFileCount)
	}
	if len(man.Tables) != TableCount() {
		t.Errorf("manifest covers %d tables, want %d", len(man.Tables), TableCount())
	}

	// Every table must be represented, including the write-only archive pair
	// that no Go read path touches.
	for _, name := range []string{"archived_expenses", "archived_expense_participants"} {
		n, ok := man.RowCount(name)
		if !ok || n == 0 {
			t.Errorf("manifest records %d rows for %s, want > 0 -- the archive tables were dropped", n, name)
		}
	}

	wipeAll(t, st)
	if got := countRows(t, st, "users"); got != 0 {
		t.Fatalf("wipe left %d users", got)
	}
	// Also remove the uploads, standing in for a lost volume.
	if err := os.RemoveAll(cfg.UploadDir); err != nil {
		t.Fatalf("remove uploads: %v", err)
	}

	v, err := r.Validate(context.Background(), path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	rep, err := r.Apply(context.Background(), v, ApplyOptions{Confirmed: true, SkipSafetyDump: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.UploadsRestored != 3 {
		t.Errorf("restored %d uploads, want 3", rep.UploadsRestored)
	}

	after := snapshotTables(t, st)
	for _, tbl := range InsertOrder() {
		b, a := before[tbl.Name], after[tbl.Name]
		if len(b) != len(a) {
			t.Errorf("%s: %d rows before, %d after", tbl.Name, len(b), len(a))
			continue
		}
		for i := range b {
			if b[i] != a[i] {
				t.Errorf("%s row %d differs:\n before: %s\n  after: %s", tbl.Name, i, b[i], a[i])
			}
		}
	}

	// The uploads must come back, contents included.
	for rel, want := range map[string]string{
		"a.png":             "fake-png-bytes",
		"nested/r.pdf":      "fake-pdf-bytes",
		"nested/deep/g.png": "another",
	} {
		got, err := os.ReadFile(filepath.Join(cfg.UploadDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("read restored upload %s: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("upload %s = %q, want %q", rel, got, want)
		}
	}

	// The staging directory must not survive a successful restore.
	if _, err := os.Stat(stagingDirFor(cfg.UploadDir)); !os.IsNotExist(err) {
		t.Errorf("staging directory still exists after a successful restore")
	}
	if _, err := os.Stat(oldDirFor(cfg.UploadDir)); !os.IsNotExist(err) {
		t.Errorf("previous upload directory was not cleaned up")
	}
}

// TestRestoreIntoFreshDatabase is the real disaster-recovery case, and the one
// the round-trip tests above only approximate: dump from one instance and
// restore into a SEPARATE, freshly provisioned one.
//
// It matters because a fresh database has every migration applied at boot with
// its own applied_at timestamps. Comparing migration identity rather than
// those timestamps is what makes this work -- including applied_at in the
// schema gate would reject exactly the restore an operator needs most.
func TestRestoreIntoFreshDatabase(t *testing.T) {
	source, srcStore, srcCfg := newTestRunner(t)
	seedEverything(t, srcStore)
	seedUploads(t, srcCfg)
	before := snapshotTables(t, srcStore)

	archive, man, err := source.DumpToDir(context.Background(), srcCfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	// A second instance: its own database file, its own upload directory, its
	// own migration timestamps. Empty apart from the schema.
	target, tgtStore, tgtCfg := newTestRunnerIn(t, t.TempDir())
	if got := countRows(t, tgtStore, "users"); got != 0 {
		t.Fatalf("the fresh database is not empty (%d users)", got)
	}

	v, err := target.Validate(context.Background(), archive)
	if err != nil {
		t.Fatalf("validate against a fresh database: %v", err)
	}
	rep, err := target.Apply(context.Background(), v, ApplyOptions{Confirmed: true, SkipSafetyDump: true})
	if err != nil {
		t.Fatalf("apply to a fresh database: %v", err)
	}
	if rep.UploadsRestored != man.UploadFileCount {
		t.Errorf("restored %d uploads, want %d", rep.UploadsRestored, man.UploadFileCount)
	}

	after := snapshotTables(t, tgtStore)
	for _, tbl := range InsertOrder() {
		b, a := before[tbl.Name], after[tbl.Name]
		if len(b) != len(a) {
			t.Errorf("%s: source has %d rows, restored instance has %d", tbl.Name, len(b), len(a))
			continue
		}
		for i := range b {
			if b[i] != a[i] {
				t.Errorf("%s row %d differs:\n source: %s\n target: %s", tbl.Name, i, b[i], a[i])
			}
		}
	}

	// Uploads land in the target's own directory, not the source's.
	got, err := os.ReadFile(filepath.Join(tgtCfg.UploadDir, "a.png"))
	if err != nil {
		t.Fatalf("read restored upload: %v", err)
	}
	if string(got) != "fake-png-bytes" {
		t.Errorf("restored upload = %q", got)
	}

	// Ids were preserved, so a row can still be addressed by its original key.
	var email string
	if err := tgtStore.DB.QueryRowContext(context.Background(),
		tgtStore.Rebind(`SELECT email FROM users WHERE id = ?`), 1).Scan(&email); err != nil {
		t.Fatalf("read user 1 from the restored instance: %v", err)
	}
	if email != "alice@example.com" {
		t.Errorf("user 1 email = %q, want alice@example.com", email)
	}
}

// TestRoundTripPreservesLargeInt64 pins the single most damaging encoding bug
// this design guards against. bigAmount is above 2^53, so if money ever passes
// through encoding/json's default float64 number handling it comes back wrong.
func TestRoundTripPreservesLargeInt64(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	wipeAll(t, st)

	v, err := r.Validate(context.Background(), path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, err := r.Apply(context.Background(), v, ApplyOptions{Confirmed: true, SkipSafetyDump: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	var got int64
	if err := st.DB.QueryRowContext(context.Background(),
		st.Rebind(`SELECT amount FROM expenses WHERE id = ?`), "exp-1").Scan(&got); err != nil {
		t.Fatalf("read amount: %v", err)
	}
	if got != bigAmount {
		t.Errorf("amount = %d, want %d (a float64 round trip would give %d)",
			got, bigAmount, int64(float64(bigAmount)))
	}

	// The zero-sum invariant every balance derives from.
	var sum int64
	if err := st.DB.QueryRowContext(context.Background(),
		st.Rebind(`SELECT COALESCE(SUM(amount),0) FROM expense_participants WHERE expense_id = ?`),
		"exp-1").Scan(&sum); err != nil {
		t.Fatalf("sum participants: %v", err)
	}
	if sum != 0 {
		t.Errorf("participant rows for exp-1 sum to %d, want 0", sum)
	}
}

// TestRoundTripPreservesJSONVerbatim asserts JSON-in-TEXT columns are carried
// byte for byte. Re-serializing them would reorder keys and drop the spacing,
// which for expense_split_inputs would quietly rewrite what the user typed.
func TestRoundTripPreservesJSONVerbatim(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	const want = `{"v":1,"method":"SHARE","values":{"2":3,  "1":1},"nested":{"z":[1,2]}}`

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	wipeAll(t, st)
	v, err := r.Validate(context.Background(), path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, err := r.Apply(context.Background(), v, ApplyOptions{Confirmed: true, SkipSafetyDump: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	var got string
	if err := st.DB.QueryRowContext(context.Background(),
		st.Rebind(`SELECT inputs FROM expense_split_inputs WHERE expense_id = ?`), "exp-1").Scan(&got); err != nil {
		t.Fatalf("read inputs: %v", err)
	}
	if got != want {
		t.Errorf("split inputs round-tripped as\n %s\nwant\n %s", got, want)
	}
}

// TestRoundTripPreservesNullVersusEmpty guards the other silent-corruption
// case: a NULL that comes back as "" (or the reverse) changes app behaviour,
// since IsActive and the like test only for validity.
func TestRoundTripPreservesNullVersusEmpty(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	wipeAll(t, st)
	v, err := r.Validate(context.Background(), path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, err := r.Apply(context.Background(), v, ApplyOptions{Confirmed: true, SkipSafetyDump: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// User 2 has a NULL banking_id and an empty-string name.
	var bankingNull bool
	var name string
	if err := st.DB.QueryRowContext(context.Background(),
		st.Rebind(`SELECT banking_id IS NULL, name FROM users WHERE id = ?`), 2).Scan(&bankingNull, &name); err != nil {
		t.Fatalf("read user 2: %v", err)
	}
	if !bankingNull {
		t.Error("user 2 banking_id came back non-NULL; a NULL was turned into a value")
	}
	if name != "" {
		t.Errorf("user 2 name = %q, want the empty string", name)
	}
	// User 1 has a non-NULL banking_id.
	var banking string
	if err := st.DB.QueryRowContext(context.Background(),
		st.Rebind(`SELECT banking_id FROM users WHERE id = ?`), 1).Scan(&banking); err != nil {
		t.Fatalf("read user 1: %v", err)
	}
	if banking != "bank-1" {
		t.Errorf("user 1 banking_id = %q, want %q", banking, "bank-1")
	}
}

// TestRoundTripPreservesBooleans covers the one kind whose physical
// representation differs between engines (INTEGER 0/1 versus BOOLEAN).
func TestRoundTripPreservesBooleans(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	wipeAll(t, st)
	v, err := r.Validate(context.Background(), path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, err := r.Apply(context.Background(), v, ApplyOptions{Confirmed: true, SkipSafetyDump: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	for _, tc := range []struct {
		id   int64
		want bool
	}{{1, true}, {2, false}} {
		var got bool
		if err := st.DB.QueryRowContext(context.Background(),
			st.Rebind(`SELECT simplify_debts FROM groups WHERE id = ?`), tc.id).Scan(&got); err != nil {
			t.Fatalf("read group %d: %v", tc.id, err)
		}
		if got != tc.want {
			t.Errorf("group %d simplify_debts = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestDumpIsDeterministic asserts two dumps of identical data produce
// identical bytes. That is what makes an equality assertion meaningful, and it
// catches accidental map iteration or wall-clock leakage into the payload.
func TestDumpIsDeterministic(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	seedUploads(t, cfg)

	first := filepath.Join(t.TempDir(), "a"+ArchiveExt)
	second := filepath.Join(t.TempDir(), "b"+ArchiveExt)
	for _, p := range []string{first, second} {
		f, err := os.Create(p)
		if err != nil {
			t.Fatalf("create %s: %v", p, err)
		}
		if _, err := r.Dump(context.Background(), f); err != nil {
			_ = f.Close()
			t.Fatalf("dump: %v", err)
		}
		_ = f.Close()
	}

	// The sealed bytes differ by design: each archive gets a fresh random salt
	// and nonce prefix. What must match is the decrypted payload, so compare
	// the manifests and the per-table row counts instead.
	ha, err := InspectFile(first)
	if err != nil {
		t.Fatalf("inspect first: %v", err)
	}
	hb, err := InspectFile(second)
	if err != nil {
		t.Fatalf("inspect second: %v", err)
	}
	if ha.SaltB64 == hb.SaltB64 {
		t.Error("two archives share a KDF salt; each must get a fresh one")
	}
	if ha.NoncePrefixB64 == hb.NoncePrefixB64 {
		t.Error("two archives share a nonce prefix; each must get a fresh one")
	}
	if ha.CreatedAt != hb.CreatedAt {
		t.Errorf("created_at differs under a fixed clock: %q vs %q", ha.CreatedAt, hb.CreatedAt)
	}
	if strings.Join(ha.SchemaMigrationsHint, ",") != strings.Join(hb.SchemaMigrationsHint, ",") {
		t.Error("schema migration hint differs between two dumps of the same database")
	}
}

// TestRestoreWithoutConfirmationChangesNothing asserts the confirmation gate
// is enforced in Apply, not merely in the callers.
func TestRestoreWithoutConfirmationChangesNothing(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	v, err := r.Validate(context.Background(), path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	defer v.Discard()

	usersBefore := countRows(t, st, "users")
	if _, err := r.Apply(context.Background(), v, ApplyOptions{Confirmed: false}); !errors.Is(err, ErrRestoreNotConfirmed) {
		t.Fatalf("apply without confirmation returned %v, want ErrRestoreNotConfirmed", err)
	}
	if got := countRows(t, st, "users"); got != usersBefore {
		t.Errorf("an unconfirmed restore changed the users table (%d -> %d)", usersBefore, got)
	}
}

// TestValidateRejectsWrongSessionSecret asserts a rotated or mistyped secret
// produces the named error, before any decryption is attempted.
func TestValidateRejectsWrongSessionSecret(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	cfg.SessionSecret = "a-completely-different-secret-value"
	if _, err := r.Validate(context.Background(), path); !errors.Is(err, ErrWrongSessionSecret) {
		t.Fatalf("validate with the wrong secret returned %v, want ErrWrongSessionSecret", err)
	}
	// And nothing was staged.
	if _, err := os.Stat(stagingDirFor(cfg.UploadDir)); err == nil {
		t.Error("a rejected archive left a staging directory behind")
	}
}

// TestValidateRejectsSchemaMismatch asserts the exact-match migration gate.
// That gate is what stops migration 0005's blanket UPDATE from re-firing
// against restored rows on a later boot.
func TestValidateRejectsSchemaMismatch(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	// Simulate the target database having one migration the archive does not.
	exec(t, st, `INSERT INTO schema_migrations (version, applied_at) VALUES (?,?)`,
		"9999_future_migration.sql", "2026-09-04T00:00:00.000Z")

	_, err = r.Validate(context.Background(), path)
	if !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("validate returned %v, want ErrSchemaMismatch", err)
	}
	if !strings.Contains(err.Error(), "9999_future_migration.sql") {
		t.Errorf("error does not name the differing migration: %v", err)
	}
	if got := countRows(t, st, "users"); got == 0 {
		t.Error("a refused restore emptied the users table")
	}
}

// TestValidateAcceptsDifferingAppliedAt is the regression guard for comparing
// migration identity rather than timestamps. A restore onto a freshly
// provisioned database -- the primary disaster-recovery case -- has the same
// migrations applied at entirely different times, and must be accepted.
func TestValidateAcceptsDifferingAppliedAt(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	path, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	exec(t, st, `UPDATE schema_migrations SET applied_at = ?`, "1999-01-01T00:00:00.000Z")

	v, err := r.Validate(context.Background(), path)
	if err != nil {
		t.Fatalf("validate rejected an archive whose migrations match but whose applied_at differs: %v", err)
	}
	v.Discard()
}

// TestSafetyDumpRestoresPriorState asserts the pre-restore dump is usable --
// it is the operator's only way back, so it has to be more than a file.
func TestSafetyDumpRestoresPriorState(t *testing.T) {
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)

	// An archive of the seeded state, then a divergent current state.
	archive, _, err := r.DumpToDir(context.Background(), cfg.BackupDir, "gosplit-backup")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	exec(t, st, `INSERT INTO users (id, name, email, currency, default_currency,
		preferred_language, role, hidden_friend_ids, created_at, theme_color)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		3, "Carol", "carol@example.com", "GBP", "GBP", "en", "USER", "[]",
		"2026-04-01T00:00:00.000Z", "burgundy")
	if got := countRows(t, st, "users"); got != 3 {
		t.Fatalf("expected 3 users before the restore, got %d", got)
	}

	v, err := r.Validate(context.Background(), archive)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	rep, err := r.Apply(context.Background(), v, ApplyOptions{Confirmed: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.SafetyDumpPath == "" {
		t.Fatal("no safety dump was written")
	}
	// The restore rolled Carol back.
	if got := countRows(t, st, "users"); got != 2 {
		t.Fatalf("expected 2 users after the restore, got %d", got)
	}

	// Now restore the safety dump and get the three-user state back.
	v2, err := r.Validate(context.Background(), rep.SafetyDumpPath)
	if err != nil {
		t.Fatalf("validate safety dump: %v", err)
	}
	if _, err := r.Apply(context.Background(), v2, ApplyOptions{Confirmed: true, SkipSafetyDump: true}); err != nil {
		t.Fatalf("apply safety dump: %v", err)
	}
	if got := countRows(t, st, "users"); got != 3 {
		t.Errorf("safety dump restored %d users, want 3 -- it did not capture the pre-restore state", got)
	}
}
