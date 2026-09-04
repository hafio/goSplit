// Package backup dumps a whole GoSplit instance -- every table plus the upload
// tree -- into one sealed archive, and restores it as an atomic, ID-preserving
// wipe-and-replace. It works across both supported engines: values are
// normalized to an engine-independent wire form (see codec.go), and every
// statement is written with `?` placeholders and rebound via store.Rebind.
//
// A restore consumes a file that could come from anywhere, so the archive is
// treated as hostile input at every layer. schema.go is the load-bearing part
// of that: it is the ONLY place a table or column identifier is ever a Go
// string literal feeding SQL text. Nothing read out of an archive is ever
// interpolated into a statement -- archive strings are only map-lookup keys or
// bound parameters.
package backup

import "fmt"

// ColumnKind is a column's logical type. It decides both the scan destination
// and the wire encoding, so neither is ever inferred from the archive's own
// contents or from whatever a driver happens to hand back.
type ColumnKind int

const (
	// KindText is a NOT NULL text column.
	KindText ColumnKind = iota
	// KindNullText is a nullable text column. NULL and "" stay distinct.
	KindNullText
	// KindInt64 is a NOT NULL integer column. Money is int64 minor units, so
	// the wire form is a quoted decimal string -- see codec.go.
	KindInt64
	// KindNullInt64 is a nullable integer column (a foreign key, typically).
	KindNullInt64
	// KindBool is a NOT NULL boolean. Physically INTEGER 0/1 on SQLite and
	// BOOLEAN on Postgres; the wire form is JSON true/false on both.
	KindBool
	// KindJSONText is a NOT NULL text column holding JSON. Carried verbatim,
	// never parsed and re-serialized, so key order and whitespace survive.
	KindJSONText
)

// Column is one column of a backed-up table.
type Column struct {
	Name string
	Kind ColumnKind
}

// Table describes one backed-up table.
type Table struct {
	Name string
	// Columns is the fixed order used for both the SELECT and the INSERT, and
	// for the positional row arrays on the wire. Changing it changes the
	// archive format.
	Columns []Column
	// Sequence is the Postgres identity sequence to resynchronize after an
	// ID-preserving restore, or "" when the table has no serial column.
	// Without the resync the next insert collides with a restored id.
	Sequence string
	// PKLen is how many LEADING columns form the primary key. Every table
	// above lists its key columns first, so a dump can order by exactly those
	// -- deterministic output (which lets a round-trip test assert equality
	// rather than set membership) while still using the primary-key index,
	// instead of sorting on every column including large JSON payloads.
	PKLen int
}

// PKColumns returns the primary-key column names.
func (t Table) PKColumns() []string { return t.ColumnNames()[:t.PKLen] }

// ColumnNames returns the column names in Columns order.
func (t Table) ColumnNames() []string {
	out := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		out[i] = c.Name
	}
	return out
}

// tables is every table in the archive, ordered so that inserting
// front-to-back never violates a foreign key and deleting back-to-front never
// does either.
//
// Strict ordering is used on both engines rather than deferring constraint
// checks: Postgres SET CONSTRAINTS ALL DEFERRED only affects constraints
// declared DEFERRABLE and the schema declares plain REFERENCES, while SQLite's
// PRAGMA foreign_keys is a no-op inside a transaction and the DSN pins it on
// (see store.sqliteDSN). Ordering is the only mechanism that behaves the same
// on both.
//
// Kinds are transcribed from internal/store/migrations/{sqlite,postgres}/
// 0001..0006.sql, whose column names and nullability mirror each other exactly.
//
// balance_view is deliberately absent. It is derived from expenses joined to
// expense_participants; backing it up would create a second source of truth for
// money, and restoring it would let stored balances contradict the rows they
// are computed from.
var tables = []Table{
	{
		Name:     "users",
		PKLen:    1,
		Sequence: "users_id_seq",
		Columns: []Column{
			{"id", KindInt64},
			{"name", KindText},
			{"email", KindText},
			{"email_verified", KindNullText},
			{"password_hash", KindNullText},
			{"image", KindNullText},
			{"currency", KindText},
			{"default_currency", KindText},
			{"preferred_language", KindText},
			{"role", KindText},
			{"deactivated_at", KindNullText},
			{"banking_id", KindNullText},
			{"hidden_friend_ids", KindJSONText},
			{"created_at", KindText},
			{"theme_color", KindText},
		},
	},
	{
		Name:     "groups",
		PKLen:    1,
		Sequence: "groups_id_seq",
		Columns: []Column{
			{"id", KindInt64},
			{"public_id", KindText},
			{"name", KindText},
			{"image", KindNullText},
			{"created_by", KindInt64},
			{"default_currency", KindText},
			{"simplify_debts", KindBool},
			{"archived_at", KindNullText},
			{"splitwise_group_id", KindNullText},
			{"created_at", KindText},
		},
	},
	{
		Name:  "group_users",
		PKLen: 2,
		Columns: []Column{
			{"group_id", KindInt64},
			{"user_id", KindInt64},
		},
	},
	{
		Name:  "friendships",
		PKLen: 2,
		Columns: []Column{
			{"owner_id", KindInt64},
			{"friend_id", KindInt64},
			{"created_at", KindText},
		},
	},
	// group_default_splits and friend_default_splits are migrated on both
	// engines but have no Go code path at all. They are backed up anyway: R2 is
	// every table verbatim, and a table the app does not read today is exactly
	// the kind of thing a partial backup loses without anyone noticing.
	{
		Name:  "group_default_splits",
		PKLen: 1,
		Columns: []Column{
			{"group_id", KindInt64},
			{"split_type", KindText},
			{"shares", KindJSONText},
		},
	},
	{
		Name:  "friend_default_splits",
		PKLen: 2,
		Columns: []Column{
			{"owner_id", KindInt64},
			{"friend_id", KindInt64},
			{"split_type", KindText},
			{"shares", KindJSONText},
		},
	},
	{
		// conversion_to_id and moved_from_id point at other expenses but are
		// not declared foreign keys, so there is no ordering constraint within
		// the table and rows can be inserted in any order.
		Name:  "expenses",
		PKLen: 1,
		Columns: []Column{
			{"id", KindText},
			{"name", KindText},
			{"category", KindText},
			{"amount", KindInt64},
			{"split_type", KindText},
			{"expense_date", KindText},
			{"currency", KindText},
			{"paid_by", KindInt64},
			{"added_by", KindInt64},
			{"updated_by", KindNullInt64},
			{"group_id", KindNullInt64},
			{"file_key", KindNullText},
			{"transaction_id", KindNullText},
			{"recurrence_id", KindNullInt64},
			{"conversion_to_id", KindNullText},
			{"moved_from_id", KindNullText},
			{"deleted_at", KindNullText},
			{"deleted_by", KindNullInt64},
			{"created_at", KindText},
			{"updated_at", KindText},
			{"note", KindText},
			{"version", KindInt64},
		},
	},
	{
		// amount is signed and the rows of one expense sum to zero. A restore
		// that got these wrong would silently misstate every balance, which is
		// why the per-table row count is verified against the manifest.
		Name:  "expense_participants",
		PKLen: 2,
		Columns: []Column{
			{"expense_id", KindText},
			{"user_id", KindInt64},
			{"amount", KindInt64},
		},
	},
	{
		Name:  "expense_split_inputs",
		PKLen: 1,
		Columns: []Column{
			{"expense_id", KindText},
			{"inputs", KindJSONText},
		},
	},
	{
		// Migrated but unused: notes actually live in expenses.note.
		Name:     "expense_notes",
		PKLen:    1,
		Sequence: "expense_notes_id_seq",
		Columns: []Column{
			{"id", KindInt64},
			{"expense_id", KindText},
			{"user_id", KindInt64},
			{"note", KindText},
			{"created_at", KindText},
		},
	},
	{
		Name:     "expense_recurrences",
		PKLen:    1,
		Sequence: "expense_recurrences_id_seq",
		Columns: []Column{
			{"id", KindInt64},
			{"cron_expression", KindText},
			{"job_name", KindText},
			{"template_expense_id", KindText},
			{"notified", KindBool},
			{"created_by", KindInt64},
			{"created_at", KindText},
			{"next_run_at", KindNullText},
		},
	},
	{
		Name:  "sessions",
		PKLen: 1,
		Columns: []Column{
			{"token", KindText},
			{"user_id", KindInt64},
			{"expires", KindText},
			{"created_at", KindText},
		},
	},
	{
		// identifier is an email string, not a foreign key.
		Name:  "verification_tokens",
		PKLen: 2,
		Columns: []Column{
			{"identifier", KindText},
			{"token", KindText},
			{"purpose", KindText},
			{"expires", KindText},
		},
	},
	{
		Name:  "push_notifications",
		PKLen: 2,
		Columns: []Column{
			{"user_id", KindInt64},
			{"endpoint", KindText},
			{"subscription", KindJSONText},
		},
	},
	{
		Name:  "cached_bank_data",
		PKLen: 1,
		Columns: []Column{
			{"user_id", KindInt64},
			{"data", KindJSONText},
			{"updated_at", KindText},
		},
	},
	{
		Name:  "cached_currency_rates",
		PKLen: 3,
		Columns: []Column{
			{"from_currency", KindText},
			{"to_currency", KindText},
			{"rate_date", KindText},
			{"rate", KindText},
		},
	},
	{
		// Restored verbatim per R2. A row carrying a future expires_at held by
		// a holder that no longer exists self-heals within the scheduler's lock
		// TTL, because AcquireLock overwrites an expired row.
		Name:  "scheduler_locks",
		PKLen: 1,
		Columns: []Column{
			{"id", KindText},
			{"holder", KindText},
			{"expires_at", KindText},
		},
	},
	{
		// key and value are non-reserved in both dialects, but every identifier
		// this package emits is double-quoted anyway (see quoteIdent), so a
		// reserved word could not break a statement.
		Name:  "app_metadata",
		PKLen: 1,
		Columns: []Column{
			{"key", KindText},
			{"value", KindText},
		},
	},
	{
		// Write-only in the rest of the codebase: the only existing code path
		// is the INSERTs in store/expenses.go, with no SELECT and no Go struct.
		// A dump built from typed store methods would lose the entire archive
		// silently. Generic column-kind scanning captures it for free.
		//
		// No foreign keys, by design (see migration 0004): the collapse must
		// never be blocked by a self-referential conversion or move link.
		// Note there is no version column here -- migration 0006 added that to
		// expenses only.
		Name:  "archived_expenses",
		PKLen: 1,
		Columns: []Column{
			{"id", KindText},
			{"name", KindText},
			{"category", KindText},
			{"amount", KindInt64},
			{"split_type", KindText},
			{"expense_date", KindText},
			{"currency", KindText},
			{"paid_by", KindInt64},
			{"added_by", KindInt64},
			{"updated_by", KindNullInt64},
			{"group_id", KindNullInt64},
			{"file_key", KindNullText},
			{"transaction_id", KindNullText},
			{"recurrence_id", KindNullInt64},
			{"conversion_to_id", KindNullText},
			{"moved_from_id", KindNullText},
			{"deleted_at", KindNullText},
			{"deleted_by", KindNullInt64},
			{"created_at", KindText},
			{"updated_at", KindText},
			{"note", KindText},
			{"archive_expense_id", KindText},
			{"archived_at", KindText},
		},
	},
	{
		Name:  "archived_expense_participants",
		PKLen: 2,
		Columns: []Column{
			{"expense_id", KindText},
			{"user_id", KindInt64},
			{"amount", KindInt64},
		},
	},
	{
		// Backed up and restored so the recorded migration set travels with the
		// data. ValidateArchive requires the archive's set to match the live
		// database's exactly, which is what stops a data migration like
		// 0005_simplify_debts_default.sql -- a blanket
		// "UPDATE groups SET simplify_debts = 1" -- from re-firing against
		// restored rows on a later boot and clobbering every group's flag.
		Name:  "schema_migrations",
		PKLen: 1,
		Columns: []Column{
			{"version", KindText},
			{"applied_at", KindText},
		},
	},
}

// byName indexes tables for lookup. Built once at init so LookupTable does not
// scan, and so a duplicate name or column in the registry fails at startup
// rather than producing a silently unreachable entry.
var byName = func() map[string]Table {
	m := make(map[string]Table, len(tables))
	for _, t := range tables {
		if _, dup := m[t.Name]; dup {
			panic(fmt.Sprintf("backup: duplicate table %q in registry", t.Name))
		}
		if !isBareIdent(t.Name) {
			panic(fmt.Sprintf("backup: table name %q is not a bare identifier", t.Name))
		}
		if t.Sequence != "" && !isBareIdent(t.Sequence) {
			panic(fmt.Sprintf("backup: sequence name %q is not a bare identifier", t.Sequence))
		}
		if len(t.Columns) == 0 {
			panic(fmt.Sprintf("backup: table %q has no columns", t.Name))
		}
		if t.PKLen < 1 || t.PKLen > len(t.Columns) {
			panic(fmt.Sprintf("backup: table %q declares PKLen %d, which is not within its %d columns", t.Name, t.PKLen, len(t.Columns)))
		}
		seen := make(map[string]bool, len(t.Columns))
		for _, c := range t.Columns {
			if seen[c.Name] {
				panic(fmt.Sprintf("backup: duplicate column %q in table %q", c.Name, t.Name))
			}
			if !isBareIdent(c.Name) {
				panic(fmt.Sprintf("backup: column name %q in table %q is not a bare identifier", c.Name, t.Name))
			}
			seen[c.Name] = true
		}
		m[t.Name] = t
	}
	return m
}()

// InsertOrder returns every table in foreign-key-safe insert order.
func InsertOrder() []Table {
	out := make([]Table, len(tables))
	copy(out, tables)
	return out
}

// DeleteOrder returns every table in the exact reverse of InsertOrder, which
// is foreign-key-safe for deletion.
func DeleteOrder() []Table {
	out := make([]Table, len(tables))
	for i, t := range tables {
		out[len(tables)-1-i] = t
	}
	return out
}

// LookupTable resolves a table name against the registry. This is the gate that
// keeps an archive's own table names out of SQL: a name that does not resolve
// here is never used, and callers treat the miss as a hard failure.
func LookupTable(name string) (Table, bool) {
	t, ok := byName[name]
	return t, ok
}

// TableCount reports how many tables the archive covers.
func TableCount() int { return len(tables) }

// isBareIdent reports whether name is a lowercase snake_case identifier. Every
// identifier this package emits is checked once here, at package init, so the
// SQL builders can quote and trust it without re-validating. A failure means
// the registry above was edited wrongly -- it cannot mean untrusted input got
// in, because nothing from an archive ever reaches this function.
func isBareIdent(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		case c == '_' && i > 0:
		default:
			return false
		}
	}
	return true
}

// quoteIdent renders a registry identifier for a statement. Both engines
// accept the standard double-quoted form, so one spelling serves each, and it
// makes a column like app_metadata."key" safe regardless of what either
// dialect happens to reserve. Only values that passed isBareIdent at init ever
// reach here, so there is nothing to escape.
func quoteIdent(name string) string { return `"` + name + `"` }

// quoteIdents renders a list of registry identifiers, comma-separated.
func quoteIdents(names []string) string {
	out := make([]byte, 0, len(names)*16)
	for i, n := range names {
		if i > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, quoteIdent(n)...)
	}
	return string(out)
}
