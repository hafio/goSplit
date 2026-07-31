package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/config"
)

// cov2ClosedStore opens a fresh SQLite test store and immediately closes its DB
// handle, so every subsequent query/exec returns "database is closed". This
// exercises the DB-error return paths that the happy-path round-1 tests skip.
func cov2ClosedStore(t *testing.T) *Store {
	t.Helper()
	st := openTestStore(t)
	if err := st.DB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	return st
}

// --- pure helpers in store.go / expenses.go / friends.go --------------------

func TestCov2PureHelpers(t *testing.T) {
	// sqliteDSN: each URL shape maps to a file: DSN with our pragmas.
	if got := sqliteDSN("sqlite://foo.db"); !strings.HasPrefix(got, "file:foo.db?") {
		t.Errorf("sqliteDSN(sqlite://) = %q", got)
	}
	if got := sqliteDSN(""); !strings.HasPrefix(got, "file:./data/gosplit.db?") {
		t.Errorf("sqliteDSN(empty) = %q", got)
	}
	// A pre-existing query string is stripped (the path[:i] branch).
	if got := sqliteDSN("file:foo.db?cache=shared"); !strings.HasPrefix(got, "file:foo.db?_pragma") {
		t.Errorf("sqliteDSN(with query) = %q", got)
	}

	// sqliteDir: dir of a real path, and "" for in-memory / dir-less.
	if got := sqliteDir("file:sub/foo.db?x=1"); got != "sub" {
		t.Errorf("sqliteDir(sub/foo.db) = %q, want sub", got)
	}
	if got := sqliteDir("file::memory:?x=1"); got != "" {
		t.Errorf("sqliteDir(:memory:) = %q, want empty", got)
	}
	if got := sqliteDir("file:"); got != "" {
		t.Errorf("sqliteDir(empty path) = %q, want empty", got)
	}

	// inPlaceholders: n<=0 yields empty; positive yields comma-joined ?.
	if got := inPlaceholders(0); got != "" {
		t.Errorf("inPlaceholders(0) = %q, want empty", got)
	}
	if got := inPlaceholders(3); got != "?, ?, ?" {
		t.Errorf("inPlaceholders(3) = %q", got)
	}

	// prefixCols: parenthesised expressions keep their inner comma, and
	// surrounding whitespace is trimmed (the depth +/- and trimSpace end-- paths).
	got := prefixCols("u", "id , coalesce(a, b) , name ")
	if !strings.Contains(got, "u.id") || !strings.Contains(got, "u.coalesce(a, b)") || !strings.Contains(got, "u.name") {
		t.Errorf("prefixCols with parens = %q", got)
	}
	if strings.Contains(got, "u.b") {
		t.Errorf("prefixCols split inside parens: %q", got)
	}

	// trimSpace: strips both ends.
	if got := trimSpace("  \t x \n "); got != "x" {
		t.Errorf("trimSpace = %q, want x", got)
	}
	if got := trimSpace("noedge"); got != "noedge" {
		t.Errorf("trimSpace(noedge) = %q", got)
	}
}

// --- ExpenseFilter.apply optional branches via ListActivity -----------------
//
// ListActivity (unlike the friend/group list views) does not reset Scope or
// GroupID, so it reaches the amount-max, scope, and specific-group branches of
// apply that round 1 never exercised.
func TestCov2ActivityFilterBranches(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "AF-A")
	b := covUser(t, st, "AF-B")
	g, err := st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	grp := &Expense{Name: "grp", Category: "general", Amount: 500, SplitType: "EQUAL",
		ExpenseDate: "2025-01-05", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
		GroupID: sql.NullInt64{Int64: g.ID, Valid: true}}
	if _, err := st.CreateExpense(ctx, grp, []ExpenseParticipant{{UserID: a.ID, Amount: 250}, {UserID: b.ID, Amount: -250}}); err != nil {
		t.Fatal(err)
	}
	covDirectExpense(t, st, "dir", "2025-01-06", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 150}, {UserID: b.ID, Amount: -150}})

	names := func(f ExpenseFilter) map[string]bool {
		es, err := st.ListActivity(ctx, a.ID, f)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]bool{}
		for _, e := range es {
			m[e.Name] = true
		}
		return m
	}

	// ScopeOnlyGroup -> group rows only.
	if m := names(ExpenseFilter{Scope: ScopeOnlyGroup}); !m["grp"] || m["dir"] {
		t.Errorf("ScopeOnlyGroup = %v, want only grp", m)
	}
	// ScopeOnlyNonGroup -> non-group rows only.
	if m := names(ExpenseFilter{Scope: ScopeOnlyNonGroup}); m["grp"] || !m["dir"] {
		t.Errorf("ScopeOnlyNonGroup = %v, want only dir", m)
	}
	// Specific-group selector.
	if m := names(ExpenseFilter{GroupID: &g.ID}); !m["grp"] || m["dir"] {
		t.Errorf("GroupID selector = %v, want only grp", m)
	}
	// AmountMax excludes the 500 group expense, keeps the 300-derived direct one.
	max := int64(400)
	if m := names(ExpenseFilter{AmountMax: &max}); m["grp"] || !m["dir"] {
		t.Errorf("AmountMax=400 = %v, want only dir", m)
	}
}

// --- insertExpenseTx default category + CollapseToHistorical empty-origin ----

func TestCov2InsertDefaultsAndCollapseNoOrigins(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "ID-A")
	b := covUser(t, st, "ID-B")

	// An empty Category is defaulted to "general" on insert.
	e := &Expense{Name: "nocat", Category: "", Amount: 100, SplitType: "EQUAL",
		ExpenseDate: "2025-01-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID}
	created, err := st.CreateExpense(ctx, e, []ExpenseParticipant{{UserID: a.ID, Amount: 100}, {UserID: b.ID, Amount: -100}})
	if err != nil {
		t.Fatal(err)
	}
	if created.Category != "general" {
		t.Errorf("default category = %q, want general", created.Category)
	}

	// A collapse batch with no OriginIDs inserts the synthetic expense and hits
	// the len(OriginIDs)==0 continue (no archival move).
	syn := &Expense{Name: "Synthetic", Category: "general", Amount: 100, SplitType: "EQUAL",
		ExpenseDate: "2025-01-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID}
	batch := HistoricalBatch{
		Expense:      syn,
		Participants: []ExpenseParticipant{{UserID: a.ID, Amount: 100}, {UserID: b.ID, Amount: -100}},
		OriginIDs:    nil,
	}
	if err := st.CollapseToHistorical(ctx, []HistoricalBatch{batch}); err != nil {
		t.Fatal(err)
	}
	if syn.ID == "" {
		t.Fatalf("synthetic id not assigned")
	}
	if got, err := st.GetExpense(ctx, syn.ID); err != nil || got.Name != "Synthetic" {
		t.Errorf("synthetic not created: %v err %v", got, err)
	}
}

// --- migrate is idempotent on an already-migrated store ---------------------
//
// A second migrate() run finds every migration recorded, exercising the
// applied[v]=true accumulation and the "already applied -> continue" skip.
func TestCov2MigrateIdempotent(t *testing.T) {
	st := openTestStore(t)
	if err := st.migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	// Running it a third time is still clean.
	if err := st.migrate(context.Background()); err != nil {
		t.Fatalf("third migrate: %v", err)
	}
}

// --- Open with an unrecognised engine ---------------------------------------

func TestCov2OpenUnknownEngine(t *testing.T) {
	_, err := Open(context.Background(), &config.Config{DatabaseURL: "x", Engine: config.Engine("mysql")})
	if err == nil || !strings.Contains(err.Error(), "unknown engine") {
		t.Fatalf("Open(unknown engine) err = %v, want unknown engine", err)
	}
}

// --- DB-error return paths across the package -------------------------------
//
// Every call below runs against a closed handle, so it must surface a non-nil
// error from its QueryContext/ExecContext/BeginTx failure path.
func TestCov2ClosedStoreErrors(t *testing.T) {
	ctx := context.Background()
	st := cov2ClosedStore(t)

	mustErr := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: expected error on closed store, got nil", name)
		}
	}

	// balances.go
	_, err := st.CumulatedBalances(ctx, 1)
	mustErr("CumulatedBalances", err)
	_, err = st.FriendBalance(ctx, 1, 2)
	mustErr("FriendBalance", err)
	_, err = st.GroupBalances(ctx, 1)
	mustErr("GroupBalances", err)
	_, err = st.UserGroupBalances(ctx, 1, 2)
	mustErr("UserGroupBalances", err)
	_, err = st.GroupMemberCounts(ctx, 1)
	mustErr("GroupMemberCounts", err)
	_, err = st.UserGroupNets(ctx, 1)
	mustErr("UserGroupNets", err)

	// friends.go
	mustErr("AddFriend", st.AddFriend(ctx, 1, 2))
	mustErr("RemoveFriend", st.RemoveFriend(ctx, 1, 2))
	_, err = st.ListFriends(ctx, 1)
	mustErr("ListFriends", err)
	_, err = st.AreFriends(ctx, 1, 2)
	mustErr("AreFriends", err)

	// groups.go
	_, err = st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: 1})
	mustErr("CreateGroup", err)
	_, err = st.GetGroup(ctx, 1)
	mustErr("GetGroup", err)
	_, err = st.GetGroupByPublicID(ctx, "p")
	mustErr("GetGroupByPublicID", err)
	_, err = st.GetGroupBySplitwiseID(ctx, "s")
	mustErr("GetGroupBySplitwiseID", err)
	mustErr("AddGroupMember", st.AddGroupMember(ctx, 1, 2))
	mustErr("RemoveGroupMember", st.RemoveGroupMember(ctx, 1, 2))
	_, err = st.IsGroupMember(ctx, 1, 2)
	mustErr("IsGroupMember", err)
	_, err = st.GroupMembers(ctx, 1)
	mustErr("GroupMembers", err)
	_, err = st.ListGroupsForUser(ctx, 1, true)
	mustErr("ListGroupsForUser", err)
	mustErr("SetGroupArchived", st.SetGroupArchived(ctx, 1, true))
	mustErr("SetGroupSimplify", st.SetGroupSimplify(ctx, 1, true))
	mustErr("UpdateGroup", st.UpdateGroup(ctx, &Group{ID: 1, Name: "x"}))

	// recurrences.go
	_, err = st.CreateRecurrence(ctx, &ExpenseRecurrence{JobName: "j", TemplateExpenseID: "t", CreatedBy: 1})
	mustErr("CreateRecurrence", err)
	_, err = st.GetRecurrence(ctx, 1)
	mustErr("GetRecurrence", err)
	_, err = st.ListRecurrencesForUser(ctx, 1)
	mustErr("ListRecurrencesForUser", err)
	_, err = st.ListDueRecurrences(ctx, nowISO())
	mustErr("ListDueRecurrences", err)
	mustErr("DeleteRecurrence", st.DeleteRecurrence(ctx, 1, 1))
	mustErr("AdvanceRecurrence", st.AdvanceRecurrence(ctx, 1, nowISO()))

	// expenses.go
	e := &Expense{Name: "x", Amount: 1, SplitType: "EQUAL", ExpenseDate: "2025-01-01", Currency: "USD", PaidBy: 1, AddedBy: 1}
	_, err = st.CreateExpense(ctx, e, []ExpenseParticipant{{UserID: 1, Amount: 0}})
	mustErr("CreateExpense", err)
	_, err = st.GetExpense(ctx, "id")
	mustErr("GetExpense", err)
	_, err = st.GetParticipants(ctx, "id")
	mustErr("GetParticipants", err)
	_, err = st.UserNetByExpense(ctx, 1, []string{"id"})
	mustErr("UserNetByExpense", err)
	mustErr("UpdateExpense", st.UpdateExpense(ctx, e, nil))
	mustErr("SoftDeleteExpense", st.SoftDeleteExpense(ctx, "id", 1))
	_, _, err = st.CreateConversionPair(ctx, e, nil, e, nil)
	mustErr("CreateConversionPair", err)
	mustErr("CollapseToHistorical", st.CollapseToHistorical(ctx, []HistoricalBatch{{Expense: e}}))
	_, err = st.ListActivity(ctx, 1, ExpenseFilter{})
	mustErr("ListActivity", err)
	_, err = st.ListGroupExpenses(ctx, 1, ExpenseFilter{})
	mustErr("ListGroupExpenses", err)
	_, err = st.ListFriendExpenses(ctx, 1, 2, ExpenseFilter{})
	mustErr("ListFriendExpenses", err)
	_, err = st.ListDirectCollapsible(ctx, 1, 2, "2025-01-01")
	mustErr("ListDirectCollapsible", err)
	_, err = st.ListGroupCollapsible(ctx, 1, "2025-01-01")
	mustErr("ListGroupCollapsible", err)

	// locks.go: the ExecContext failure returns (false, err).
	ok, err := st.AcquireLock(ctx, "id", "holder", time.Minute)
	mustErr("AcquireLock", err)
	if ok {
		t.Errorf("AcquireLock on closed store returned ok=true")
	}

	// db_helpers.go: the Postgres branch of insertReturningID (RETURNING id +
	// QueryRow scan). Reuse the closed handle but tag it Postgres so the branch
	// executes and errors on the closed connection -- no live Postgres needed.
	pg := &Store{DB: st.DB, Engine: config.EnginePostgres}
	_, err = pg.insertReturningID(ctx, "INSERT INTO groups (name) VALUES (?)", "groups", "x")
	mustErr("insertReturningID(postgres)", err)
}
