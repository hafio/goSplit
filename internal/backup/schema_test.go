package backup

import (
	"context"
	"sort"
	"strings"
	"testing"
)

// TestRegistryCoversEveryLiveTable is the drift guard. The registry is hand
// written, so without this a future migration could add a table and every
// backup from then on would silently omit it.
func TestRegistryCoversEveryLiveTable(t *testing.T) {
	_, st, _ := newTestRunner(t)

	rows, err := st.DB.QueryContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var live []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		live = append(live, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(live) == 0 {
		t.Fatal("found no tables; the migrations did not run")
	}

	inRegistry := map[string]bool{}
	for _, tbl := range InsertOrder() {
		inRegistry[tbl.Name] = true
	}
	for _, name := range live {
		if !inRegistry[name] {
			t.Errorf("table %q exists in the schema but is missing from the backup registry -- every backup would silently omit it", name)
		}
	}

	isLive := map[string]bool{}
	for _, n := range live {
		isLive[n] = true
	}
	for _, tbl := range InsertOrder() {
		if !isLive[tbl.Name] {
			t.Errorf("registry lists table %q, which does not exist in the schema", tbl.Name)
		}
	}
}

// TestRegistryColumnsMatchLiveSchema checks names, order and nullability per
// table, so a migration that adds or renames a column is caught here rather
// than by a restore failing in production.
func TestRegistryColumnsMatchLiveSchema(t *testing.T) {
	_, st, _ := newTestRunner(t)

	for _, tbl := range InsertOrder() {
		t.Run(tbl.Name, func(t *testing.T) {
			rows, err := st.DB.QueryContext(context.Background(),
				`SELECT name, "notnull", pk FROM pragma_table_info(?)`, tbl.Name)
			if err != nil {
				t.Fatalf("pragma_table_info: %v", err)
			}
			defer func() { _ = rows.Close() }()

			type col struct {
				name    string
				notNull bool
				pk      int
			}
			var live []col
			for rows.Next() {
				var c col
				if err := rows.Scan(&c.name, &c.notNull, &c.pk); err != nil {
					t.Fatalf("scan: %v", err)
				}
				live = append(live, c)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("rows: %v", err)
			}
			if len(live) == 0 {
				t.Fatalf("table %s has no columns", tbl.Name)
			}

			if len(live) != len(tbl.Columns) {
				t.Fatalf("schema has %d columns, registry has %d", len(live), len(tbl.Columns))
			}
			for i, c := range live {
				want := tbl.Columns[i]
				if c.name != want.Name {
					t.Errorf("column %d is %q in the schema but %q in the registry", i, c.name, want.Name)
					continue
				}
				// A primary key is never NULL whatever the notnull flag says:
				// SQLite reports notnull=0 for an INTEGER PRIMARY KEY, since it
				// aliases the rowid.
				if c.pk > 0 {
					continue
				}
				nullableInRegistry := want.Kind == KindNullText || want.Kind == KindNullInt64
				if c.notNull && nullableInRegistry {
					t.Errorf("column %q is NOT NULL in the schema but nullable in the registry", c.name)
				}
				if !c.notNull && !nullableInRegistry {
					t.Errorf("column %q is nullable in the schema but NOT NULL in the registry", c.name)
				}
			}
		})
	}
}

// TestRegistryPKLenMatchesLiveSchema asserts each table's declared PKLen names
// its real primary key, since the dump orders by exactly those columns.
func TestRegistryPKLenMatchesLiveSchema(t *testing.T) {
	_, st, _ := newTestRunner(t)

	for _, tbl := range InsertOrder() {
		rows, err := st.DB.QueryContext(context.Background(),
			`SELECT name FROM pragma_table_info(?) WHERE pk > 0 ORDER BY pk`, tbl.Name)
		if err != nil {
			t.Fatalf("%s: pragma_table_info: %v", tbl.Name, err)
		}
		var pk []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				_ = rows.Close()
				t.Fatalf("%s: scan: %v", tbl.Name, err)
			}
			pk = append(pk, n)
		}
		_ = rows.Close()

		got := tbl.PKColumns()
		sortedGot := append([]string(nil), got...)
		sortedPK := append([]string(nil), pk...)
		sort.Strings(sortedGot)
		sort.Strings(sortedPK)
		if strings.Join(sortedGot, ",") != strings.Join(sortedPK, ",") {
			t.Errorf("%s: registry PKLen names %v, schema primary key is %v", tbl.Name, got, pk)
		}
	}
}

// TestRegistryExcludesBalanceView asserts the derived view stays out. Backing
// it up would create a second source of truth for money, and restoring it
// would let stored balances contradict the rows they are computed from.
func TestRegistryExcludesBalanceView(t *testing.T) {
	if _, ok := LookupTable("balance_view"); ok {
		t.Fatal("balance_view is in the registry; balances are derived and must never be restored")
	}
	for _, tbl := range InsertOrder() {
		if strings.Contains(tbl.Name, "balance") {
			t.Errorf("registry contains a balance table %q", tbl.Name)
		}
	}
}

// TestDeleteOrderIsExactReverse asserts deletion runs children first.
func TestDeleteOrderIsExactReverse(t *testing.T) {
	ins, del := InsertOrder(), DeleteOrder()
	if len(ins) != len(del) {
		t.Fatalf("insert order has %d tables, delete order %d", len(ins), len(del))
	}
	for i := range ins {
		if ins[i].Name != del[len(del)-1-i].Name {
			t.Fatalf("delete order is not the exact reverse at index %d: %q vs %q",
				i, del[len(del)-1-i].Name, ins[i].Name)
		}
	}
}

// TestInsertOrderSatisfiesForeignKeys asserts a referenced table always
// appears before the table referencing it, which is what makes ordered
// insertion safe on both engines.
func TestInsertOrderSatisfiesForeignKeys(t *testing.T) {
	_, st, _ := newTestRunner(t)

	position := map[string]int{}
	for i, tbl := range InsertOrder() {
		position[tbl.Name] = i
	}

	for _, tbl := range InsertOrder() {
		rows, err := st.DB.QueryContext(context.Background(),
			`SELECT "table" FROM pragma_foreign_key_list(?)`, tbl.Name)
		if err != nil {
			t.Fatalf("%s: pragma_foreign_key_list: %v", tbl.Name, err)
		}
		var refs []string
		for rows.Next() {
			var ref string
			if err := rows.Scan(&ref); err != nil {
				_ = rows.Close()
				t.Fatalf("%s: scan: %v", tbl.Name, err)
			}
			refs = append(refs, ref)
		}
		_ = rows.Close()

		for _, ref := range refs {
			if ref == tbl.Name {
				continue // self-reference imposes no ordering
			}
			refPos, ok := position[ref]
			if !ok {
				t.Errorf("%s references %s, which is not in the registry", tbl.Name, ref)
				continue
			}
			if refPos > position[tbl.Name] {
				t.Errorf("%s (position %d) references %s (position %d), which is inserted later",
					tbl.Name, position[tbl.Name], ref, refPos)
			}
		}
	}
}

// TestInsertOrderStartsWithUsers is a readability guard: users and groups are
// referenced by nearly everything, so they belong first.
func TestInsertOrderStartsWithUsers(t *testing.T) {
	order := InsertOrder()
	if order[0].Name != "users" {
		t.Errorf("first table is %q, want users", order[0].Name)
	}
	if order[1].Name != "groups" {
		t.Errorf("second table is %q, want groups", order[1].Name)
	}
}

// TestLookupTableRejectsHostileNames asserts the registry gate refuses
// anything an archive might carry. A name that does not resolve here never
// reaches SQL.
func TestLookupTableRejectsHostileNames(t *testing.T) {
	hostile := []string{
		"",
		"balance_view",
		"users; DROP TABLE users",
		`users" ; DROP TABLE "users`,
		"USERS",
		"Users",
		"../users",
		"..",
		"sqlite_master",
		"pg_catalog.pg_tables",
		"users\x00",
		"users ",
		" users",
		"unknown_table",
	}
	for _, name := range hostile {
		if _, ok := LookupTable(name); ok {
			t.Errorf("LookupTable(%q) resolved; it must not", name)
		}
	}
	// And a real one still resolves.
	if _, ok := LookupTable("users"); !ok {
		t.Error("LookupTable(\"users\") did not resolve")
	}
}

// TestIsBareIdent pins the identifier rule that lets quoteIdent skip escaping.
func TestIsBareIdent(t *testing.T) {
	good := []string{"users", "group_users", "expense_split_inputs", "a", "a1", "a_1"}
	bad := []string{
		"", "1abc", "_abc", "Users", "user-name", "user name", "user.name",
		`user"name`, "user;name", "user\x00", strings.Repeat("a", 64),
	}
	for _, s := range good {
		if !isBareIdent(s) {
			t.Errorf("isBareIdent(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if isBareIdent(s) {
			t.Errorf("isBareIdent(%q) = true, want false", s)
		}
	}
}

// TestQuoteIdent asserts the double-quoted form, which both engines accept and
// which makes a reserved word like app_metadata."key" safe.
func TestQuoteIdent(t *testing.T) {
	if got := quoteIdent("users"); got != `"users"` {
		t.Errorf("quoteIdent = %s, want %q", got, `"users"`)
	}
	if got := quoteIdents([]string{"a", "b", "c"}); got != `"a", "b", "c"` {
		t.Errorf("quoteIdents = %s", got)
	}
	if got := quoteIdents(nil); got != "" {
		t.Errorf("quoteIdents(nil) = %q, want empty", got)
	}
}

// TestSequencesOnlyOnAutoIncrementTables asserts a Postgres sequence is
// declared exactly for the tables that have a serial primary key, since a
// missed one means the next insert collides with a restored id.
func TestSequencesOnlyOnAutoIncrementTables(t *testing.T) {
	want := map[string]string{
		"users":               "users_id_seq",
		"groups":              "groups_id_seq",
		"expense_notes":       "expense_notes_id_seq",
		"expense_recurrences": "expense_recurrences_id_seq",
		"notifications":       "notifications_id_seq",
	}
	for _, tbl := range InsertOrder() {
		if got := tbl.Sequence; got != want[tbl.Name] {
			t.Errorf("%s: sequence = %q, want %q", tbl.Name, got, want[tbl.Name])
		}
	}
}

// TestSameMigrationSet covers the comparison and the message it produces.
func TestSameMigrationSet(t *testing.T) {
	tests := []struct {
		name     string
		archive  []string
		live     []string
		same     bool
		mentions []string
	}{
		{"identical", []string{"a", "b"}, []string{"a", "b"}, true, nil},
		{"order insensitive", []string{"b", "a"}, []string{"a", "b"}, true, nil},
		{"both empty", nil, nil, true, nil},
		{"archive ahead", []string{"a", "b"}, []string{"a"}, false, []string{"b"}},
		{"live ahead", []string{"a"}, []string{"a", "b"}, false, []string{"b"}},
		{"divergent", []string{"a", "x"}, []string{"a", "y"}, false, []string{"x", "y"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			same, diff := SameMigrationSet(tc.archive, tc.live)
			if same != tc.same {
				t.Fatalf("SameMigrationSet = %v, want %v (diff %q)", same, tc.same, diff)
			}
			for _, m := range tc.mentions {
				if !strings.Contains(diff, m) {
					t.Errorf("difference %q does not name %q", diff, m)
				}
			}
		})
	}
}

// TestTableCountMatchesRegistry keeps the exported count honest.
func TestTableCountMatchesRegistry(t *testing.T) {
	if TableCount() != len(InsertOrder()) {
		t.Errorf("TableCount = %d, registry has %d", TableCount(), len(InsertOrder()))
	}
}
