package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/config"
)

// covUser creates a user with a unique email derived from name, failing the
// test on error. It reuses the openTestStore helper's store.
func covUser(t *testing.T, st *Store, name string) *User {
	t.Helper()
	u, err := st.CreateUser(context.Background(), &User{Name: name, Email: name + "@cov.test"})
	if err != nil {
		t.Fatalf("create user %s: %v", name, err)
	}
	return u
}

// covDirectExpense inserts a non-group expense with the given participants.
func covDirectExpense(t *testing.T, st *Store, name string, date string, payer int64, parts []ExpenseParticipant) *Expense {
	t.Helper()
	var amount int64
	for _, p := range parts {
		if p.Amount > 0 {
			amount += p.Amount
		}
	}
	e := &Expense{
		Name: name, Category: "general", Amount: amount, SplitType: "EQUAL",
		ExpenseDate: date, Currency: "USD", PaidBy: payer, AddedBy: payer,
	}
	created, err := st.CreateExpense(context.Background(), e, parts)
	if err != nil {
		t.Fatalf("create expense %s: %v", name, err)
	}
	return created
}

// --- cross-engine rebinding --------------------------------------------------

func TestCovRebind(t *testing.T) {
	// SQLite keeps `?` untouched.
	sqlite := &Store{Engine: config.EngineSQLite}
	if got := sqlite.rebind("SELECT * FROM t WHERE a = ? AND b = ?"); got != "SELECT * FROM t WHERE a = ? AND b = ?" {
		t.Errorf("sqlite rebind changed query: %q", got)
	}
	// Postgres rewrites each `?` to a positional $n in order.
	pg := &Store{Engine: config.EnginePostgres}
	if got := pg.rebind("SELECT * FROM t WHERE a = ? AND b = ? OR c = ?"); got != "SELECT * FROM t WHERE a = $1 AND b = $2 OR c = $3" {
		t.Errorf("postgres rebind = %q", got)
	}
	// No placeholders: unchanged on both engines.
	if got := pg.rebind("SELECT 1"); got != "SELECT 1" {
		t.Errorf("postgres rebind of placeholderless query = %q", got)
	}
}

// --- id / time generators ----------------------------------------------------

func TestCovIDGenerators(t *testing.T) {
	u1 := NewUUID()
	u2 := NewUUID()
	if len(u1) != 36 {
		t.Fatalf("uuid length = %d, want 36", len(u1))
	}
	if u1[8] != '-' || u1[13] != '-' || u1[18] != '-' || u1[23] != '-' {
		t.Errorf("uuid dashes misplaced: %q", u1)
	}
	if u1[14] != '4' {
		t.Errorf("uuid version nibble = %c, want 4", u1[14])
	}
	if !strings.ContainsRune("89ab", rune(u1[19])) {
		t.Errorf("uuid variant nibble = %c, want one of 8/9/a/b", u1[19])
	}
	if u1 == u2 {
		t.Errorf("two UUIDs collided: %q", u1)
	}

	nano := NewNanoID(21)
	if len(nano) != 21 {
		t.Fatalf("nanoid length = %d, want 21", len(nano))
	}
	for _, c := range nano {
		if !strings.ContainsRune(nanoAlphabet, c) {
			t.Errorf("nanoid contains out-of-alphabet rune %q", c)
		}
	}

	// FromNow returns an ISO timestamp strictly after the current instant.
	if FromNow(time.Hour) <= nowISO() {
		t.Errorf("FromNow(1h) not in the future")
	}
}

// --- users -------------------------------------------------------------------

func TestCovUserCRUD(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	// CreateUser applies defaults when fields are empty.
	u, err := st.CreateUser(ctx, &User{Name: "Mixed", Email: "  Mixed@X.com  "})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "mixed@x.com" {
		t.Errorf("email not normalized: %q", u.Email)
	}
	if u.Role != "USER" || u.Currency != "USD" || u.DefaultCurrency != "USD" ||
		u.PreferredLanguage != "en" || u.ThemeColor != "burgundy" {
		t.Errorf("defaults not applied: %+v", u)
	}

	// GetUserByEmail is case- and whitespace-insensitive.
	if g, err := st.GetUserByEmail(ctx, "  MIXED@X.COM "); err != nil || g.ID != u.ID {
		t.Errorf("GetUserByEmail: got %v err %v", g, err)
	}

	// Not-found paths wrap sql.ErrNoRows into ErrNotFound.
	if _, err := st.GetUser(ctx, 99999); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUser(missing) err = %v, want ErrNotFound", err)
	}
	if _, err := st.GetUserByEmail(ctx, "nobody@x.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUserByEmail(missing) err = %v, want ErrNotFound", err)
	}

	// SetPassword persists a hash.
	if err := st.SetPassword(ctx, u.ID, "argon2id$hash"); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.GetUser(ctx, u.ID); !g.PasswordHash.Valid || g.PasswordHash.String != "argon2id$hash" {
		t.Errorf("password not set: %+v", g.PasswordHash)
	}

	// SetRole updates the role.
	if err := st.SetRole(ctx, u.ID, "ADMIN"); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.GetUser(ctx, u.ID); g.Role != "ADMIN" {
		t.Errorf("role = %q, want ADMIN", g.Role)
	}

	// MarkEmailVerified sets a timestamp, and is idempotent (only when NULL).
	if err := st.MarkEmailVerified(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	g1, _ := st.GetUser(ctx, u.ID)
	if !g1.EmailVerified.Valid {
		t.Fatalf("email_verified not set")
	}
	if err := st.MarkEmailVerified(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	g2, _ := st.GetUser(ctx, u.ID)
	if g2.EmailVerified.String != g1.EmailVerified.String {
		t.Errorf("MarkEmailVerified not idempotent: %q -> %q", g1.EmailVerified.String, g2.EmailVerified.String)
	}

	// SetDeactivated toggles the account state (IsActive reflects it).
	if err := st.SetDeactivated(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.GetUser(ctx, u.ID); g.IsActive() {
		t.Errorf("expected deactivated account")
	}
	if err := st.SetDeactivated(ctx, u.ID, false); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.GetUser(ctx, u.ID); !g.IsActive() {
		t.Errorf("expected reactivated account")
	}

	// SetHiddenFriends stores JSON; nil normalizes to an empty list.
	if err := st.SetHiddenFriends(ctx, u.ID, []int64{7, 9}); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.GetUser(ctx, u.ID); len(g.HiddenFriendIDs) != 2 || g.HiddenFriendIDs[0] != 7 || g.HiddenFriendIDs[1] != 9 {
		t.Errorf("hidden friends = %v, want [7 9]", g.HiddenFriendIDs)
	}
	if err := st.SetHiddenFriends(ctx, u.ID, nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.GetUser(ctx, u.ID); len(g.HiddenFriendIDs) != 0 {
		t.Errorf("hidden friends after nil = %v, want empty", g.HiddenFriendIDs)
	}

	// ListUsers returns all users ordered by id.
	covUser(t, st, "Second")
	all, err := st.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID > all[1].ID {
		t.Errorf("ListUsers not ordered by id: %+v", all)
	}

	// DeleteUser removes the row.
	if err := st.DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetUser(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUser after delete = %v, want ErrNotFound", err)
	}
}

// --- groups ------------------------------------------------------------------

func TestCovGroupCRUD(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	owner := covUser(t, st, "Owner")
	member := covUser(t, st, "Member")

	// CreateGroup assigns a public id and auto-adds the creator as a member.
	g, err := st.CreateGroup(ctx, &Group{Name: "Trip", CreatedBy: owner.ID,
		SplitwiseGroupID: sql.NullString{String: "sw-1", Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.PublicID) != 12 {
		t.Errorf("auto public id length = %d, want 12", len(g.PublicID))
	}
	if ok, _ := st.IsGroupMember(ctx, g.ID, owner.ID); !ok {
		t.Errorf("creator not auto-added as member")
	}
	if ok, _ := st.IsGroupMember(ctx, g.ID, member.ID); ok {
		t.Errorf("non-member reported as member")
	}

	// Lookups by the various keys.
	if got, err := st.GetGroupByPublicID(ctx, g.PublicID); err != nil || got.ID != g.ID {
		t.Errorf("GetGroupByPublicID: %v err %v", got, err)
	}
	if got, err := st.GetGroupBySplitwiseID(ctx, "sw-1"); err != nil || got.ID != g.ID {
		t.Errorf("GetGroupBySplitwiseID: %v err %v", got, err)
	}

	// Not-found paths.
	if _, err := st.GetGroup(ctx, 99999); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGroup(missing) = %v", err)
	}
	if _, err := st.GetGroupByPublicID(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGroupByPublicID(missing) = %v", err)
	}
	if _, err := st.GetGroupBySplitwiseID(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGroupBySplitwiseID(missing) = %v", err)
	}

	// Membership add (idempotent), list ordering, and removal.
	if err := st.AddGroupMember(ctx, g.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g.ID, member.ID); err != nil { // idempotent
		t.Fatal(err)
	}
	members, err := st.GroupMembers(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("GroupMembers = %d, want 2", len(members))
	}
	if members[0].Name != "Member" || members[1].Name != "Owner" { // ordered by name
		t.Errorf("GroupMembers not name-ordered: %q, %q", members[0].Name, members[1].Name)
	}
	if err := st.RemoveGroupMember(ctx, g.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.IsGroupMember(ctx, g.ID, member.ID); ok {
		t.Errorf("member still present after removal")
	}

	// UpdateGroup mutates name/image/currency.
	g.Name, g.DefaultCurrency = "Renamed", "EUR"
	g.Image = sql.NullString{String: "img.png", Valid: true}
	if err := st.UpdateGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetGroup(ctx, g.ID); got.Name != "Renamed" || got.DefaultCurrency != "EUR" || got.Image.String != "img.png" {
		t.Errorf("UpdateGroup not persisted: %+v", got)
	}

	// SetGroupSimplify toggles the flag.
	if err := st.SetGroupSimplify(ctx, g.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetGroup(ctx, g.ID); !got.SimplifyDebts {
		t.Errorf("simplify_debts not set")
	}

	// SetGroupArchived + ListGroupsForUser archived filter.
	g2, err := st.CreateGroup(ctx, &Group{Name: "Second", CreatedBy: owner.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetGroupArchived(ctx, g2.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetGroup(ctx, g2.ID); !got.IsArchived() {
		t.Errorf("group not archived")
	}
	active, err := st.ListGroupsForUser(ctx, owner.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != g.ID {
		t.Errorf("ListGroupsForUser(excl archived) = %+v, want only g", active)
	}
	allG, err := st.ListGroupsForUser(ctx, owner.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(allG) != 2 {
		t.Errorf("ListGroupsForUser(incl archived) = %d, want 2", len(allG))
	}

	// Unarchive path (archived=false sets NULL).
	if err := st.SetGroupArchived(ctx, g2.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetGroup(ctx, g2.ID); got.IsArchived() {
		t.Errorf("group still archived after unarchive")
	}
}

// --- sessions ----------------------------------------------------------------

func TestCovSessions(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	u := covUser(t, st, "Sessioner")

	sess, err := st.CreateSession(ctx, "tok-valid", u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID != u.ID || sess.Token != "tok-valid" {
		t.Errorf("CreateSession returned %+v", sess)
	}
	if got, err := st.GetSession(ctx, "tok-valid"); err != nil || got.UserID != u.ID {
		t.Errorf("GetSession valid: %v err %v", got, err)
	}

	// Unknown token -> ErrNotFound.
	if _, err := st.GetSession(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession(missing) = %v", err)
	}

	// Expired session: GetSession lazily deletes and reports ErrNotFound.
	if _, err := st.CreateSession(ctx, "tok-exp", u.ID, -time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetSession(ctx, "tok-exp"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession(expired) = %v, want ErrNotFound", err)
	}

	// DeleteSession (logout).
	if err := st.DeleteSession(ctx, "tok-valid"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetSession(ctx, "tok-valid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession after delete = %v", err)
	}

	// DeleteUserSessions removes all of a user's sessions.
	_, _ = st.CreateSession(ctx, "tok-a", u.ID, time.Hour)
	_, _ = st.CreateSession(ctx, "tok-b", u.ID, time.Hour)
	if err := st.DeleteUserSessions(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetSession(ctx, "tok-a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("session survived DeleteUserSessions")
	}

	// DeleteExpiredSessions purges only expired rows.
	_, _ = st.CreateSession(ctx, "tok-live", u.ID, time.Hour)
	_, _ = st.CreateSession(ctx, "tok-dead", u.ID, -time.Hour)
	if err := st.DeleteExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetSession(ctx, "tok-live"); err != nil || got.UserID != u.ID {
		t.Errorf("live session removed by DeleteExpiredSessions: %v", err)
	}
}

// --- verification tokens -----------------------------------------------------

func TestCovVerificationTokens(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	// Identifier is normalized on write; a wrong purpose does not consume it.
	if err := st.CreateVerificationToken(ctx, "  Foo@Bar.COM ", "vt1", "verify", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UseVerificationToken(ctx, "vt1", "reset"); !errors.Is(err, ErrNotFound) {
		t.Errorf("wrong purpose = %v, want ErrNotFound", err)
	}
	id, err := st.UseVerificationToken(ctx, "vt1", "verify")
	if err != nil || id != "foo@bar.com" {
		t.Errorf("UseVerificationToken = %q err %v, want foo@bar.com", id, err)
	}
	// Single-use: a second use fails.
	if _, err := st.UseVerificationToken(ctx, "vt1", "verify"); !errors.Is(err, ErrNotFound) {
		t.Errorf("token reused: %v", err)
	}

	// Unknown token -> ErrNotFound.
	if _, err := st.UseVerificationToken(ctx, "nope", "verify"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown token = %v", err)
	}

	// Expired token is consumed but reported as ErrNotFound.
	if err := st.CreateVerificationToken(ctx, "e@x.com", "vt2", "reset", -time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UseVerificationToken(ctx, "vt2", "reset"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired token = %v, want ErrNotFound", err)
	}
	// It was deleted (single-use), so a second attempt is still ErrNotFound.
	if _, err := st.UseVerificationToken(ctx, "vt2", "reset"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired token not consumed: %v", err)
	}

	// DeleteExpiredTokens runs cleanly.
	if err := st.CreateVerificationToken(ctx, "z@x.com", "vt3", "magic", -time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteExpiredTokens(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UseVerificationToken(ctx, "vt3", "magic"); !errors.Is(err, ErrNotFound) {
		t.Errorf("token survived DeleteExpiredTokens: %v", err)
	}
}

// --- expenses: lifecycle -----------------------------------------------------

func TestCovExpenseLifecycle(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")

	e := covDirectExpense(t, st, "Dinner", "2025-01-10", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 300}, {UserID: b.ID, Amount: -300}})

	// GetExpense round-trips; unknown id -> ErrNotFound.
	if got, err := st.GetExpense(ctx, e.ID); err != nil || got.Name != "Dinner" {
		t.Errorf("GetExpense: %v err %v", got, err)
	}
	if _, err := st.GetExpense(ctx, "no-such-id"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetExpense(missing) = %v", err)
	}

	// GetParticipants is ordered by user_id.
	parts, err := st.GetParticipants(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || parts[0].UserID != a.ID || parts[0].Amount != 300 || parts[1].Amount != -300 {
		t.Errorf("GetParticipants = %+v", parts)
	}

	// UpdateExpense replaces fields and the participant set.
	e.Name = "Brunch"
	e.Amount = 100
	if err := st.UpdateExpense(ctx, e, []ExpenseParticipant{{UserID: a.ID, Amount: 100}, {UserID: b.ID, Amount: -100}}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetExpense(ctx, e.ID)
	if got.Name != "Brunch" || got.Amount != 100 {
		t.Errorf("UpdateExpense fields not persisted: %+v", got)
	}
	np, _ := st.GetParticipants(ctx, e.ID)
	if len(np) != 2 || np[0].Amount != 100 || np[1].Amount != -100 {
		t.Errorf("participants after update = %+v", np)
	}

	// SoftDeleteExpense marks the row (GetExpense still returns it).
	if err := st.SoftDeleteExpense(ctx, e.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	del, _ := st.GetExpense(ctx, e.ID)
	if !del.DeletedAt.Valid || !del.DeletedBy.Valid || del.DeletedBy.Int64 != b.ID {
		t.Errorf("soft delete not recorded: %+v", del)
	}
}

func TestCovConversionPair(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")

	from := &Expense{Name: "conv from", Category: "general", Amount: 10000, SplitType: "CURRENCY_CONVERSION",
		ExpenseDate: "2025-02-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID}
	fromParts := []ExpenseParticipant{{UserID: a.ID, Amount: 10000}, {UserID: b.ID, Amount: -10000}}
	to := &Expense{Name: "conv to", Category: "general", Amount: 9000, SplitType: "CURRENCY_CONVERSION",
		ExpenseDate: "2025-02-01", Currency: "EUR", PaidBy: b.ID, AddedBy: b.ID}
	toParts := []ExpenseParticipant{{UserID: a.ID, Amount: -9000}, {UserID: b.ID, Amount: 9000}}

	fromID, toID, err := st.CreateConversionPair(ctx, from, fromParts, to, toParts)
	if err != nil {
		t.Fatal(err)
	}
	if fromID == "" || toID == "" || fromID == toID {
		t.Fatalf("bad ids: from=%q to=%q", fromID, toID)
	}
	gotFrom, err := st.GetExpense(ctx, fromID)
	if err != nil {
		t.Fatal(err)
	}
	if !gotFrom.ConversionToID.Valid || gotFrom.ConversionToID.String != toID {
		t.Errorf("from.conversion_to_id = %+v, want %q", gotFrom.ConversionToID, toID)
	}
	if _, err := st.GetExpense(ctx, toID); err != nil {
		t.Errorf("to expense not found: %v", err)
	}
}

// --- expenses: listing + collapse -------------------------------------------

func TestCovExpenseListing(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")
	g, err := st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	mkGroup := func(name string) *Expense {
		e := &Expense{Name: name, Category: "general", Amount: 1000, SplitType: "EQUAL",
			ExpenseDate: "2025-01-05", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
			GroupID: sql.NullInt64{Int64: g.ID, Valid: true}}
		created, err := st.CreateExpense(ctx, e, []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
		if err != nil {
			t.Fatal(err)
		}
		return created
	}
	g1 := mkGroup("Group A")
	mkGroup("Group B")

	// ListGroupExpenses excludes soft-deleted rows.
	if err := st.SoftDeleteExpense(ctx, g1.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	ge, err := st.ListGroupExpenses(ctx, g.ID, ExpenseFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ge) != 1 || ge[0].Name != "Group B" {
		t.Errorf("ListGroupExpenses = %+v, want only Group B", ge)
	}

	// ListActivity includes expenses involving the user (incl. soft-deleted).
	act, err := st.ListActivity(ctx, a.ID, ExpenseFilter{})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range act {
		names[e.Name] = true
	}
	if !names["Group A"] || !names["Group B"] {
		t.Errorf("ListActivity missing expenses: %v", names)
	}
}

func TestCovCollapseHistorical(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")

	e1 := covDirectExpense(t, st, "old1", "2025-01-01", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 300}, {UserID: b.ID, Amount: -300}})
	e2 := covDirectExpense(t, st, "old2", "2025-01-02", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 200}, {UserID: b.ID, Amount: -200}})

	cutoff := "2025-06-01"
	list, err := st.ListDirectCollapsible(ctx, a.ID, b.ID, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("ListDirectCollapsible = %d, want 2", len(list))
	}

	synthetic := &Expense{Name: "Historical Transactions", Category: "general", Amount: 500, SplitType: "EQUAL",
		ExpenseDate: "2025-01-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID}
	batch := HistoricalBatch{
		Expense:      synthetic,
		Participants: []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}},
		OriginIDs:    []string{e1.ID, e2.ID},
	}
	if err := st.CollapseToHistorical(ctx, []HistoricalBatch{batch}); err != nil {
		t.Fatal(err)
	}
	if synthetic.ID == "" {
		t.Fatalf("synthetic expense id not assigned")
	}
	// Originals are moved out of the live table.
	if _, err := st.GetExpense(ctx, e1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("origin e1 still live: %v", err)
	}
	if _, err := st.GetExpense(ctx, e2.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("origin e2 still live: %v", err)
	}
	// The synthetic replacement is live.
	if got, err := st.GetExpense(ctx, synthetic.ID); err != nil || got.Name != "Historical Transactions" {
		t.Errorf("synthetic expense missing: %v err %v", got, err)
	}
}

func TestCovListGroupCollapsible(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")
	g, err := st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	e := &Expense{Name: "old group", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2025-01-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
		GroupID: sql.NullInt64{Int64: g.ID, Valid: true}}
	if _, err := st.CreateExpense(ctx, e, []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}}); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListGroupCollapsible(ctx, g.ID, "2025-06-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "old group" {
		t.Errorf("ListGroupCollapsible = %+v, want one row", list)
	}
	// A cutoff before the expense date yields nothing.
	empty, err := st.ListGroupCollapsible(ctx, g.ID, "2024-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("ListGroupCollapsible(early cutoff) = %+v, want empty", empty)
	}
}

// --- recurrences -------------------------------------------------------------

func TestCovRecurrences(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")
	tmpl := covDirectExpense(t, st, "template", "2025-01-01", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 100}, {UserID: b.ID, Amount: -100}})

	r, err := st.CreateRecurrence(ctx, &ExpenseRecurrence{
		CronExpression: "0 0 * * *", JobName: "job-1", TemplateExpenseID: tmpl.ID,
		CreatedBy: a.ID, NextRunAt: sql.NullString{String: "2025-01-01T00:00:00.000Z", Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == 0 || r.JobName != "job-1" {
		t.Errorf("CreateRecurrence = %+v", r)
	}

	if got, err := st.GetRecurrence(ctx, r.ID); err != nil || got.TemplateExpenseID != tmpl.ID {
		t.Errorf("GetRecurrence: %v err %v", got, err)
	}
	if _, err := st.GetRecurrence(ctx, 99999); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetRecurrence(missing) = %v", err)
	}

	if lst, err := st.ListRecurrencesForUser(ctx, a.ID); err != nil || len(lst) != 1 {
		t.Errorf("ListRecurrencesForUser = %+v err %v", lst, err)
	}

	// Due when next_run_at <= now; not due once advanced into the future.
	due, err := st.ListDueRecurrences(ctx, nowISO())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Fatalf("ListDueRecurrences = %d, want 1", len(due))
	}
	if err := st.AdvanceRecurrence(ctx, r.ID, FromNow(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	due2, err := st.ListDueRecurrences(ctx, nowISO())
	if err != nil {
		t.Fatal(err)
	}
	if len(due2) != 0 {
		t.Errorf("ListDueRecurrences after advance = %d, want 0", len(due2))
	}

	// DeleteRecurrence is scoped to the owner.
	if err := st.DeleteRecurrence(ctx, r.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetRecurrence(ctx, r.ID); err != nil {
		t.Errorf("recurrence deleted by non-owner: %v", err)
	}
	if err := st.DeleteRecurrence(ctx, r.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetRecurrence(ctx, r.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetRecurrence after owner delete = %v", err)
	}
}

// --- friends -----------------------------------------------------------------

func TestCovFriends(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")

	// Self-friendship is a no-op.
	if err := st.AddFriend(ctx, a.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.AreFriends(ctx, a.ID, a.ID); ok {
		t.Errorf("self reported as friend")
	}

	// AddFriend is bidirectional and idempotent.
	if err := st.AddFriend(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.AddFriend(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.AreFriends(ctx, a.ID, b.ID); !ok {
		t.Errorf("a not friends with b")
	}
	if ok, _ := st.AreFriends(ctx, b.ID, a.ID); !ok {
		t.Errorf("friendship not bidirectional")
	}
	if lst, err := st.ListFriends(ctx, a.ID); err != nil || len(lst) != 1 || lst[0].ID != b.ID {
		t.Errorf("ListFriends = %+v err %v", lst, err)
	}

	// RemoveFriend clears both directions.
	if err := st.RemoveFriend(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.AreFriends(ctx, a.ID, b.ID); ok {
		t.Errorf("friendship survived removal (a->b)")
	}
	if ok, _ := st.AreFriends(ctx, b.ID, a.ID); ok {
		t.Errorf("friendship survived removal (b->a)")
	}
}

// --- bank / push / rates caches ---------------------------------------------

func TestCovBankData(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	u := covUser(t, st, "A")

	if _, err := st.GetBankData(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetBankData(missing) = %v", err)
	}
	if err := st.PutBankData(ctx, u.ID, `{"v":1}`); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetBankData(ctx, u.ID); err != nil || got != `{"v":1}` {
		t.Errorf("GetBankData = %q err %v", got, err)
	}
	// Upsert overwrites.
	if err := st.PutBankData(ctx, u.ID, `{"v":2}`); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetBankData(ctx, u.ID); got != `{"v":2}` {
		t.Errorf("PutBankData did not upsert: %q", got)
	}
}

func TestCovPushSubscriptions(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	u := covUser(t, st, "A")

	// Empty id list short-circuits to nil.
	if got, err := st.ListPushSubscriptions(ctx, nil); err != nil || got != nil {
		t.Errorf("ListPushSubscriptions(nil) = %v err %v", got, err)
	}

	if err := st.SavePushSubscription(ctx, u.ID, "ep1", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if err := st.SavePushSubscription(ctx, u.ID, "ep2", `{"b":1}`); err != nil {
		t.Fatal(err)
	}
	subs, err := st.ListPushSubscriptions(ctx, []int64{u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("ListPushSubscriptions = %d, want 2", len(subs))
	}

	// Upsert on (user, endpoint) updates the subscription without adding a row.
	if err := st.SavePushSubscription(ctx, u.ID, "ep1", `{"a":2}`); err != nil {
		t.Fatal(err)
	}
	subs2, _ := st.ListPushSubscriptions(ctx, []int64{u.ID})
	if len(subs2) != 2 {
		t.Errorf("upsert added a row: %d", len(subs2))
	}
	found := false
	for _, s := range subs2 {
		if s.Endpoint == "ep1" && s.Subscription == `{"a":2}` {
			found = true
		}
	}
	if !found {
		t.Errorf("ep1 subscription not updated: %+v", subs2)
	}

	// DeletePushSubscription removes one endpoint.
	if err := st.DeletePushSubscription(ctx, u.ID, "ep1"); err != nil {
		t.Fatal(err)
	}
	subs3, _ := st.ListPushSubscriptions(ctx, []int64{u.ID})
	if len(subs3) != 1 || subs3[0].Endpoint != "ep2" {
		t.Errorf("after delete = %+v, want only ep2", subs3)
	}
}

func TestCovRatesCache(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	if _, err := st.GetCachedRate(ctx, "USD", "EUR", "2025-01-01"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetCachedRate(missing) = %v", err)
	}
	// Currencies are upper-cased on write and read.
	if err := st.PutCachedRate(ctx, "usd", "eur", "2025-01-01", "1.10"); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetCachedRate(ctx, "usd", "eur", "2025-01-01"); err != nil || got != "1.10" {
		t.Errorf("GetCachedRate(lower) = %q err %v", got, err)
	}
	if got, err := st.GetCachedRate(ctx, "USD", "EUR", "2025-01-01"); err != nil || got != "1.10" {
		t.Errorf("GetCachedRate(upper) = %q err %v", got, err)
	}
	// Upsert on the composite key.
	if err := st.PutCachedRate(ctx, "USD", "EUR", "2025-01-01", "1.20"); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetCachedRate(ctx, "USD", "EUR", "2025-01-01"); got != "1.20" {
		t.Errorf("PutCachedRate did not upsert: %q", got)
	}
	// DeleteRatesBefore purges older dates.
	if err := st.DeleteRatesBefore(ctx, "2025-06-01"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetCachedRate(ctx, "USD", "EUR", "2025-01-01"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rate survived DeleteRatesBefore: %v", err)
	}
}

// --- balances: friend + group views -----------------------------------------

func TestCovFriendAndGroupBalances(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")
	g, err := st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	// a paid 1000, b owes 500. In balance_view a sees b at +500.
	e := &Expense{Name: "grocery", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2025-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
		GroupID: sql.NullInt64{Int64: g.ID, Valid: true}}
	if _, err := st.CreateExpense(ctx, e, []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}}); err != nil {
		t.Fatal(err)
	}

	fb, err := st.FriendBalance(ctx, a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fb) != 1 || fb[0].FriendID != b.ID || fb[0].Currency != "USD" || fb[0].Amount != 500 {
		t.Errorf("FriendBalance = %+v, want b/USD/500", fb)
	}

	gb, err := st.GroupBalances(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Two directed rows (a->b +500 and b->a -500), both non-zero.
	if len(gb) != 2 {
		t.Fatalf("GroupBalances = %d rows, want 2", len(gb))
	}
	var found bool
	for _, row := range gb {
		if row.UserID == a.ID && row.FriendID == b.ID && row.Amount == 500 {
			found = true
		}
	}
	if !found {
		t.Errorf("GroupBalances missing a->b +500 row: %+v", gb)
	}
}

// --- scheduler locks ---------------------------------------------------------

func TestCovAcquireLock(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	// First acquisition succeeds; the same holder can renew.
	if ok, err := st.AcquireLock(ctx, "leader", "A", time.Minute); err != nil || !ok {
		t.Fatalf("initial acquire: ok=%v err=%v", ok, err)
	}
	if ok, err := st.AcquireLock(ctx, "leader", "A", time.Minute); err != nil || !ok {
		t.Fatalf("renew: ok=%v err=%v", ok, err)
	}
	// A different holder cannot steal an unexpired lock.
	if ok, err := st.AcquireLock(ctx, "leader", "B", time.Minute); err != nil || ok {
		t.Fatalf("contention: ok=%v err=%v, want false", ok, err)
	}

	// Expired lock can be taken over by another holder.
	if ok, err := st.AcquireLock(ctx, "exp", "A", -time.Minute); err != nil || !ok {
		t.Fatalf("expired insert: ok=%v err=%v", ok, err)
	}
	if ok, err := st.AcquireLock(ctx, "exp", "B", time.Minute); err != nil || !ok {
		t.Fatalf("expired takeover: ok=%v err=%v, want true", ok, err)
	}
	// Now B holds it unexpired, so A is refused.
	if ok, err := st.AcquireLock(ctx, "exp", "A", time.Minute); err != nil || ok {
		t.Fatalf("post-takeover contention: ok=%v err=%v, want false", ok, err)
	}
}

// --- model helper flags ------------------------------------------------------

func TestCovModelFlags(t *testing.T) {
	if !(&User{Role: "ADMIN"}).IsAdmin() {
		t.Error("ADMIN should be admin")
	}
	if (&User{Role: "USER"}).IsAdmin() {
		t.Error("USER should not be admin")
	}
	if !(&User{}).IsActive() {
		t.Error("user without deactivated_at should be active")
	}
	if (&User{DeactivatedAt: sql.NullString{String: "2025-01-01", Valid: true}}).IsActive() {
		t.Error("deactivated user should not be active")
	}
	if (&Group{}).IsArchived() {
		t.Error("group without archived_at should not be archived")
	}
	if !(&Group{ArchivedAt: sql.NullString{String: "2025-01-01", Valid: true}}).IsArchived() {
		t.Error("group with archived_at should be archived")
	}
}
