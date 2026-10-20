package service

import (
	"context"
	"sync"
	"testing"

	"github.com/hafio/gosplit/internal/balance"
	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// chainFixture is the case netting exists for: in group g, a owes b 50.00 (b
// paid the hotel) and b owes c 50.00 (c paid the taxi), so the simplified debt
// is a single "a pays c 50.00".
type chainFixture struct {
	svc     *Service
	a, b, c *store.User
	g       *store.Group
}

func newChain(t *testing.T, simplify bool) chainFixture {
	t.Helper()
	ctx := context.Background()
	svc, _ := newTestService(t)
	f := chainFixture{svc: svc}
	f.a = covUser(t, svc, "A", "a@x.com")
	f.b = covUser(t, svc, "B", "b@x.com")
	f.c = covUser(t, svc, "C", "c@x.com")
	f.g, _ = svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: f.a.ID, DefaultCurrency: "USD", SimplifyDebts: simplify})
	_ = svc.Store.AddGroupMember(ctx, f.g.ID, f.b.ID)
	_ = svc.Store.AddGroupMember(ctx, f.g.ID, f.c.ID)
	f.expense(t, "Hotel", f.b.ID, []store.ExpenseParticipant{{UserID: f.b.ID, Amount: 5000}, {UserID: f.a.ID, Amount: -5000}})
	f.expense(t, "Taxi", f.c.ID, []store.ExpenseParticipant{{UserID: f.c.ID, Amount: 5000}, {UserID: f.b.ID, Amount: -5000}})
	return f
}

// expense writes a row straight through the store: no service hook runs. The
// name "settlement" makes it a SETTLEMENT row.
func (f chainFixture) expense(t *testing.T, name string, payer int64, parts []store.ExpenseParticipant) *store.Expense {
	t.Helper()
	kind := "EQUAL"
	if name == "settlement" {
		kind = "SETTLEMENT"
	}
	var amt int64
	for _, p := range parts {
		if p.Amount > 0 {
			amt += p.Amount
		}
	}
	gid := f.g.ID
	e, err := f.svc.Store.CreateExpense(context.Background(), &store.Expense{
		Name: name, Category: "general", Amount: amt, SplitType: kind, ExpenseDate: "2026-03-10",
		Currency: "USD", PaidBy: payer, AddedBy: payer, GroupID: nullInt(&gid),
	}, parts)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return e
}

// pairs reads the group's pair balances the way the netting writer does.
func pairsOf(t *testing.T, svc *Service, gid int64) []balance.Pair {
	t.Helper()
	bals, err := svc.Store.GroupBalances(context.Background(), gid)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]balance.Row, 0, len(bals))
	for _, b := range bals {
		rows = append(rows, balance.Row{UserID: b.UserID, FriendID: b.FriendID, Currency: b.Currency, Amount: b.Amount})
	}
	return balance.PairsFromRows(rows)
}

// assertSimplified checks the netting invariant on a group: its pairs are
// exactly its simplified debts (nothing left to rewire).
func assertSimplified(t *testing.T, svc *Service, gid int64) []balance.Pair {
	t.Helper()
	p := pairsOf(t, svc, gid)
	if rest := balance.Rewire(p); len(rest) != 0 {
		t.Fatalf("group %d pairs %v are not its simplified debts (still %v)", gid, p, rest)
	}
	return p
}

func nettingRows(t *testing.T, svc *Service) int {
	t.Helper()
	var n int
	if err := svc.Store.DB.QueryRow(`SELECT COUNT(*) FROM expenses WHERE split_type = 'NETTING'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// assertNettingZeroSum checks that every person's netting rows sum to zero, per
// currency: the entries never change what anyone owes in total.
func assertNettingZeroSum(t *testing.T, svc *Service) {
	t.Helper()
	rows, err := svc.Store.DB.Query(`SELECT ep.user_id, e.currency, SUM(ep.amount) FROM expense_participants ep
		JOIN expenses e ON e.id = ep.expense_id WHERE e.split_type = 'NETTING'
		GROUP BY ep.user_id, e.currency HAVING SUM(ep.amount) <> 0`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var cur string
		var sum int64
		_ = rows.Scan(&id, &cur, &sum)
		t.Errorf("netting rows for user %d sum to %d %s, want 0", id, sum, cur)
	}
}

func notifCountFor(t *testing.T, svc *Service, userID int64) int {
	t.Helper()
	var n int
	_ = svc.Store.DB.QueryRow(svc.Store.Rebind(`SELECT COUNT(*) FROM notifications WHERE user_id = ?`), userID).Scan(&n)
	return n
}

// TestSettleAllNetsTheChain: settling the whole group pays a -> c 50.00 and
// then rewires the three pairs, so every pairwise balance -- and every person's
// Balances page -- reads zero. Three NETTING rows, each person's sum zero, no
// extra notifications, and nothing more to do on a second pass.
func TestSettleAllNetsTheChain(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, false) // settle-up rewires whatever the display setting
	settled, err := f.svc.SettleAll(ctx, []Transfer{{FromID: f.a.ID, ToID: f.c.ID, Amount: 5000, Currency: "USD"}},
		f.g.ID, "2026-03-12", f.a.ID)
	if err != nil || len(settled) != 1 {
		t.Fatalf("SettleAll = %d settlements, %v; want 1", len(settled), err)
	}
	if n := nettingRows(t, f.svc); n != 3 {
		t.Fatalf("%d netting rows, want 3", n)
	}
	assertNettingZeroSum(t, f.svc)
	if p := assertSimplified(t, f.svc, f.g.ID); len(p) != 0 {
		t.Errorf("pairs after a full settle-up = %v, want none", p)
	}
	for _, u := range []*store.User{f.a, f.b, f.c} {
		if cum, _ := f.svc.Store.CumulatedBalances(ctx, u.ID); len(cum) != 0 {
			t.Errorf("%s still shows balances %v after the settle-up", u.Name, cum)
		}
	}
	// Notifications are the settle-up's own: c is told of the payment, b of nothing.
	if n := notifCountFor(t, f.svc, f.b.ID); n != 0 {
		t.Errorf("b got %d notifications; netting must not notify", n)
	}
	if n := notifCountFor(t, f.svc, f.c.ID); n != 1 {
		t.Errorf("c got %d notifications, want the one for the payment", n)
	}
	if n, err := f.svc.netGroup(ctx, f.g.ID, "", f.a.ID, "", false); err != nil || n != 0 {
		t.Errorf("second pass wrote %d entries (err %v), want 0", n, err)
	}
}

// TestSettleNetsOnlySimplifyingGroups: a single settlement re-nets a group that
// shows simplified debts (the partial-payment sign flip lands as "a owes c
// 30.00"), but not a group showing raw pairs, and never a direct settlement.
func TestSettleNetsOnlySimplifyingGroups(t *testing.T) {
	ctx := context.Background()
	on := newChain(t, true)
	gid := on.g.ID
	if _, err := on.svc.Settle(ctx, on.a.ID, on.c.ID, 2000, "USD", &gid, "2026-03-12", on.a.ID); err != nil {
		t.Fatal(err)
	}
	p := assertSimplified(t, on.svc, gid)
	if len(p) != 1 || p[0].A != on.a.ID || p[0].B != on.c.ID || p[0].Owed != -3000 {
		t.Errorf("pairs after a partial payment = %v, want only a owes c 30.00", p)
	}
	assertNettingZeroSum(t, on.svc)

	off := newChain(t, false)
	gid = off.g.ID
	if _, err := off.svc.Settle(ctx, off.a.ID, off.c.ID, 2000, "USD", &gid, "2026-03-12", off.a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := off.svc.Settle(ctx, off.a.ID, off.b.ID, 1000, "USD", nil, "2026-03-12", off.a.ID); err != nil {
		t.Fatal(err)
	}
	if n := nettingRows(t, off.svc); n != 0 {
		t.Errorf("%d netting rows in a raw-pairs group and a direct settlement, want 0", n)
	}
}

// TestSettleAllFailureWritesNoNetting: a settle-up that fails part-way returns
// its error as before and writes no netting.
func TestSettleAllFailureWritesNoNetting(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	_, err := f.svc.SettleAll(ctx, []Transfer{
		{FromID: f.a.ID, ToID: f.c.ID, Amount: 5000, Currency: "USD"},
		{FromID: f.b.ID, ToID: f.c.ID, Amount: 0, Currency: "USD"},
	}, f.g.ID, "2026-03-12", f.a.ID)
	if err != ErrSettlementAmount {
		t.Fatalf("err = %v, want ErrSettlementAmount", err)
	}
	if n := nettingRows(t, f.svc); n != 0 {
		t.Errorf("%d netting rows after a failed settle-up, want 0", n)
	}
}

// TestSettlementEditsReNet: editing a group settlement re-nets its group; moving
// it re-nets both groups when they simplify, and leaves a raw-pairs group alone.
func TestSettlementEditsReNet(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	gid := f.g.ID
	st, err := f.svc.Settle(ctx, f.a.ID, f.c.ID, 5000, "USD", &gid, "2026-03-12", f.a.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Same group, smaller amount: a still owes c 20.00, and the pairs say so.
	edited, err := f.svc.UpdateSettlement(ctx, st.ID, SettlementInput{Name: "Settlement", Amount: 3000, Currency: "USD",
		ExpenseDate: "2026-03-12", GroupID: &gid, ActorID: f.a.ID, Version: st.Version}, false)
	if err != nil {
		t.Fatal(err)
	}
	if p := assertSimplified(t, f.svc, gid); len(p) != 1 || p[0].Owed != -2000 {
		t.Errorf("pairs after the amount edit = %v, want a owes c 20.00", p)
	}

	// Move it to a second simplifying group: both groups end up simplified.
	g2, _ := f.svc.Store.CreateGroup(ctx, &store.Group{Name: "Two", CreatedBy: f.a.ID, DefaultCurrency: "USD", SimplifyDebts: true})
	_ = f.svc.Store.AddGroupMember(ctx, g2.ID, f.c.ID)
	moved, err := f.svc.UpdateSettlement(ctx, st.ID, SettlementInput{Name: "Settlement", Amount: 3000, Currency: "USD",
		ExpenseDate: "2026-03-12", GroupID: &g2.ID, ActorID: f.a.ID, Version: edited.Version}, true)
	if err != nil {
		t.Fatal(err)
	}
	assertSimplified(t, f.svc, gid)
	assertSimplified(t, f.svc, g2.ID)

	// Into a raw-pairs group: nothing is netted there.
	g3, _ := f.svc.Store.CreateGroup(ctx, &store.Group{Name: "Raw", CreatedBy: f.a.ID, DefaultCurrency: "USD"})
	_ = f.svc.Store.AddGroupMember(ctx, g3.ID, f.c.ID)
	if _, err := f.svc.UpdateSettlement(ctx, st.ID, SettlementInput{Name: "Settlement", Amount: 3000, Currency: "USD",
		ExpenseDate: "2026-03-12", GroupID: &g3.ID, ActorID: f.a.ID, Version: moved.Version}, true); err != nil {
		t.Fatal(err)
	}
	var inRaw int
	_ = f.svc.Store.DB.QueryRow(f.svc.Store.Rebind(`SELECT COUNT(*) FROM expenses WHERE split_type = 'NETTING' AND group_id = ?`), g3.ID).Scan(&inRaw)
	if inRaw != 0 {
		t.Errorf("%d netting rows in a raw-pairs group, want 0", inRaw)
	}
	assertNettingZeroSum(t, f.svc)
}

// TestDeleteGroupSettlementReNets: deleting a group settlement brings its debt
// back, and the group's pairs are rewired to the simplified debts again -- with
// every net back where it was before the payment.
func TestDeleteGroupSettlementReNets(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	gid := f.g.ID
	before := balance.Nets(pairsOf(t, f.svc, gid))
	st, err := f.svc.Settle(ctx, f.b.ID, f.c.ID, 2000, "USD", &gid, "2026-03-12", f.b.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSimplified(t, f.svc, gid)
	if err := f.svc.DeleteExpense(ctx, st.ID, f.b.ID, st.Version); err != nil {
		t.Fatal(err)
	}
	after := assertSimplified(t, f.svc, gid)
	if got := balance.Nets(after); got["USD"][f.a.ID] != before["USD"][f.a.ID] ||
		got["USD"][f.c.ID] != before["USD"][f.c.ID] {
		t.Errorf("nets after deleting the settlement = %v, want %v", got, before)
	}
	assertNettingZeroSum(t, f.svc)
}

// TestRecurringGroupSettlementReNets: a recurring settlement in a simplifying
// group is rewired like a manual one.
func TestRecurringGroupSettlementReNets(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	tmpl := f.expense(t, "settlement", f.a.ID, []store.ExpenseParticipant{{UserID: f.a.ID, Amount: 1000}, {UserID: f.c.ID, Amount: -1000}})
	rec, err := f.svc.CreateRecurrence(ctx, f.a.ID, tmpl.ID, "0 9 1 * *")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.generateOne(ctx, rec); err != nil {
		t.Fatal(err)
	}
	assertSimplified(t, f.svc, f.g.ID)
	assertNettingZeroSum(t, f.svc)
}

// TestNettingFailureKeepsTheSettlement: when the netting write fails, the
// settlement still stands and is reported as recorded -- a failure here must
// not invite a retry that pays twice -- and no partial netting remains.
func TestNettingFailureKeepsTheSettlement(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	if _, err := f.svc.Store.DB.Exec(`CREATE TRIGGER no_netting BEFORE INSERT ON expenses
		WHEN NEW.split_type = 'NETTING' BEGIN SELECT RAISE(ABORT, 'netting unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	gid := f.g.ID
	st, err := f.svc.Settle(ctx, f.a.ID, f.c.ID, 5000, "USD", &gid, "2026-03-12", f.a.ID)
	if err != nil || st == nil {
		t.Fatalf("Settle = %v, %v; the settlement must succeed although netting failed", st, err)
	}
	if n := nettingRows(t, f.svc); n != 0 {
		t.Errorf("%d netting rows after a failed netting pass, want 0", n)
	}
}

// TestNettingEntriesAreImmutable: nobody can edit, move or delete a netting
// entry -- not its payer, its other side, its creator, nor an admin -- and a
// refused attempt changes nothing and tells no one.
func TestNettingEntriesAreImmutable(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	if _, err := f.svc.SettleAll(ctx, []Transfer{{FromID: f.a.ID, ToID: f.c.ID, Amount: 5000, Currency: "USD"}},
		f.g.ID, "2026-03-12", f.a.ID); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := f.svc.Store.DB.QueryRow(`SELECT id FROM expenses WHERE split_type = 'NETTING' LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	entry, _ := f.svc.Store.GetExpense(ctx, id)
	parts, _ := f.svc.Store.GetParticipants(ctx, id)
	admin, _ := f.svc.Store.CreateUser(ctx, &store.User{Name: "Root", Email: "root@x.com", Role: store.RoleAdmin})
	notes0 := countRows(t, f.svc, "notifications")
	gid := f.g.ID

	for _, actor := range []int64{parts[0].UserID, parts[1].UserID, entry.AddedBy, admin.ID} {
		if f.svc.CanEditExpense(ctx, actor, entry) {
			t.Errorf("actor %d may edit a netting entry", actor)
		}
		in := ExpenseInput{Name: "x", Category: "general", Total: entry.Amount, Method: split.EQUAL, Currency: "USD",
			ExpenseDate: "2026-03-12", PaidBy: parts[0].UserID, GroupID: &gid,
			Lines: []split.Line{{UserID: parts[0].UserID}, {UserID: parts[1].UserID}}, ActorID: actor, Version: entry.Version}
		if _, err := f.svc.MoveExpense(ctx, id, in, true); err != ErrImmutable {
			t.Errorf("actor %d MoveExpense: got %v, want ErrImmutable", actor, err)
		}
		if _, err := f.svc.UpdateSettlement(ctx, id, SettlementInput{Amount: 1, Currency: "USD", GroupID: &gid,
			ActorID: actor, Version: entry.Version}, true); err != ErrImmutable {
			t.Errorf("actor %d UpdateSettlement: got %v, want ErrImmutable", actor, err)
		}
		if err := f.svc.DeleteExpense(ctx, id, actor, entry.Version); err != ErrImmutable {
			t.Errorf("actor %d DeleteExpense: got %v, want ErrImmutable", actor, err)
		}
	}
	after, _ := f.svc.Store.GetExpense(ctx, id)
	if after.Version != entry.Version || after.DeletedAt.Valid || after.SplitType != "NETTING" {
		t.Errorf("a refused change still touched the entry: %+v", after)
	}
	if n := countRows(t, f.svc, "notifications"); n != notes0 {
		t.Errorf("notifications %d -> %d after refused changes", notes0, n)
	}
	if Movable(after) {
		t.Error("a netting entry reports itself movable")
	}
}

// TestRecurrenceRefusesAutomaticEntries: a netting entry or a history collapse
// can never become a recurring template.
func TestRecurrenceRefusesAutomaticEntries(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	gid := f.g.ID
	var archived *store.Expense
	for _, kind := range []string{"NETTING", "ARCHIVE"} {
		e, err := f.svc.Store.CreateExpense(ctx, &store.Expense{Name: kind, Category: "general", Amount: 100, SplitType: kind,
			ExpenseDate: "2026-03-10", Currency: "USD", PaidBy: f.a.ID, AddedBy: f.a.ID, GroupID: nullInt(&gid)},
			[]store.ExpenseParticipant{{UserID: f.a.ID, Amount: 100}, {UserID: f.b.ID, Amount: -100}})
		if err != nil {
			t.Fatal(err)
		}
		archived = e
		if _, err := f.svc.CreateRecurrence(ctx, f.a.ID, e.ID, "0 9 1 * *"); err == nil {
			t.Errorf("%s template accepted for a recurrence", kind)
		}
	}
	if n := countRows(t, f.svc, "expense_recurrences"); n != 0 {
		t.Errorf("%d recurrences written, want 0", n)
	}

	// A recurrence scheduled before the guard existed must not clone it either.
	rec, err := f.svc.Store.CreateRecurrence(ctx, &store.ExpenseRecurrence{
		CronExpression: "0 9 1 * *", JobName: "legacy", TemplateExpenseID: archived.ID, CreatedBy: f.a.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := countRows(t, f.svc, "expenses")
	if err := f.svc.generateOne(ctx, rec); err == nil {
		t.Error("generateOne cloned a history collapse")
	}
	if n := countRows(t, f.svc, "expenses"); n != before {
		t.Errorf("expenses %d -> %d: a refused occurrence was written", before, n)
	}
}

// TestCollapseAfterNettingKeepsNets: collapsing a netted group's history leaves
// every net exactly as it was.
func TestCollapseAfterNettingKeepsNets(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	gid := f.g.ID
	if _, err := f.svc.Settle(ctx, f.a.ID, f.c.ID, 2000, "USD", &gid, "2026-03-12", f.a.ID); err != nil {
		t.Fatal(err)
	}
	before := netByUser(t, f.svc)
	if _, err := f.svc.ArchiveGroup(ctx, f.a.ID, f.g.ID, "2099-01-01"); err != nil {
		t.Fatal(err)
	}
	if after := netByUser(t, f.svc); after != before {
		t.Errorf("nets changed by the collapse: %s -> %s", before, after)
	}
}

// TestConcurrentNettingConverges: two passes over the same dirty group at once
// write the rewiring once, not twice (the second sees the first's result).
func TestConcurrentNettingConverges(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, true)
	// A settled cycle written without the hook: a paid c directly in the store.
	f.expense(t, "settlement", f.a.ID, []store.ExpenseParticipant{{UserID: f.a.ID, Amount: 5000}, {UserID: f.c.ID, Amount: -5000}})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.svc.netGroup(ctx, f.g.ID, "", f.a.ID, "", false)
		}()
	}
	wg.Wait()
	if n := nettingRows(t, f.svc); n != 3 {
		t.Errorf("%d netting rows after two concurrent passes, want 3", n)
	}
	assertSimplified(t, f.svc, f.g.ID)
}

// TestTidyGroup: the tidy-up clears a settled group's leftover pairs, for
// members only, and leaves a group with debts still owed untouched.
func TestTidyGroup(t *testing.T) {
	ctx := context.Background()
	f := newChain(t, false)
	if n, err := f.svc.TidyGroup(ctx, f.a, f.g.ID); err != nil || n != 0 {
		t.Fatalf("tidy with debts outstanding = %d, %v; want 0 entries", n, err)
	}
	f.expense(t, "settlement", f.a.ID, []store.ExpenseParticipant{{UserID: f.a.ID, Amount: 5000}, {UserID: f.c.ID, Amount: -5000}})
	outsider := covUser(t, f.svc, "Out", "out@x.com")
	if _, err := f.svc.TidyGroup(ctx, outsider, f.g.ID); err != ErrNotGroupMember {
		t.Fatalf("outsider tidy: got %v, want ErrNotGroupMember", err)
	}
	if n := nettingRows(t, f.svc); n != 0 {
		t.Fatalf("%d netting rows after a refused tidy, want 0", n)
	}
	if n, err := f.svc.TidyGroup(ctx, f.b, f.g.ID); err != nil || n != 3 {
		t.Fatalf("tidy = %d, %v; want 3 entries", n, err)
	}
	if p := pairsOf(t, f.svc, f.g.ID); len(p) != 0 {
		t.Errorf("pairs after tidying = %v, want none", p)
	}
	if n, _ := f.svc.TidyGroup(ctx, f.b, f.g.ID); n != 0 {
		t.Errorf("second tidy wrote %d entries, want 0", n)
	}
}
