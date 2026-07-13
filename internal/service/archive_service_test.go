package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

// netByUser dumps each user's total net per currency from balance_view — the
// invariant a collapse must preserve (pairwise routing may change for groups,
// but per-user net does not).
func netByUser(t *testing.T, svc *Service) string {
	t.Helper()
	rows, err := svc.Store.DB.Query(
		`SELECT user_id, currency, SUM(amount) FROM balance_view
		 GROUP BY user_id, currency HAVING SUM(amount) <> 0 ORDER BY user_id, currency`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var b strings.Builder
	for rows.Next() {
		var uid, amt int64
		var cur string
		if err := rows.Scan(&uid, &cur, &amt); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%d/%s=%d;", uid, cur, amt)
	}
	return b.String()
}

func countRows(t *testing.T, svc *Service, table string) int {
	t.Helper()
	var n int
	if err := svc.Store.DB.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func mkExp(t *testing.T, svc *Service, name string, amount int64, cur, date string, payer int64, gid *int64, parts []store.ExpenseParticipant) {
	t.Helper()
	e := &store.Expense{
		Name: name, Category: "general", Amount: amount, SplitType: "EQUAL",
		ExpenseDate: date, Currency: cur, PaidBy: payer, AddedBy: payer, GroupID: nullInt(gid),
	}
	if _, err := svc.Store.CreateExpense(context.Background(), e, parts); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveDirectPreservesNet(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})

	mkExp(t, svc, "Dinner", 1000, "SGD", "2026-01-10T00:00:00Z", a.ID, nil,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	mkExp(t, svc, "Taxi", 600, "SGD", "2026-02-10T00:00:00Z", b.ID, nil,
		[]store.ExpenseParticipant{{UserID: b.ID, Amount: 300}, {UserID: a.ID, Amount: -300}})
	// After the cutoff — must survive.
	mkExp(t, svc, "Later", 800, "SGD", "2026-06-10T00:00:00Z", a.ID, nil,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 400}, {UserID: b.ID, Amount: -400}})

	before := netByUser(t, svc)
	n, err := svc.ArchiveDirect(ctx, a.ID, b.ID, "2026-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("collapsed %d, want 2", n)
	}
	if after := netByUser(t, svc); after != before {
		t.Fatalf("net changed by archive: before %q after %q", before, after)
	}
	// Two originals moved to the archive; the "Later" expense stays live.
	if got := countRows(t, svc, "archived_expenses"); got != 2 {
		t.Errorf("archived_expenses = %d, want 2", got)
	}
	// One synthetic Historical Transactions expense now exists.
	var arch int
	_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses WHERE split_type = 'ARCHIVE'`).Scan(&arch)
	if arch != 1 {
		t.Errorf("archive expenses = %d, want 1", arch)
	}
}

func TestArchiveMultiCurrency(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	mkExp(t, svc, "SGD one", 1000, "SGD", "2026-01-10T00:00:00Z", a.ID, nil,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	mkExp(t, svc, "USD one", 400, "USD", "2026-01-11T00:00:00Z", b.ID, nil,
		[]store.ExpenseParticipant{{UserID: b.ID, Amount: 200}, {UserID: a.ID, Amount: -200}})

	before := netByUser(t, svc)
	if _, err := svc.ArchiveDirect(ctx, a.ID, b.ID, "2026-03-01"); err != nil {
		t.Fatal(err)
	}
	if after := netByUser(t, svc); after != before {
		t.Fatalf("net changed: before %q after %q", before, after)
	}
	var arch int
	_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses WHERE split_type = 'ARCHIVE'`).Scan(&arch)
	if arch != 2 { // one per currency
		t.Errorf("archive expenses = %d, want 2 (per currency)", arch)
	}
}

func TestArchiveGroupPreservesNet(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	c, _ := svc.Store.CreateUser(ctx, &store.User{Name: "C", Email: "c@x.com"})
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: a.ID, DefaultCurrency: "USD"})
	for _, uid := range []int64{a.ID, b.ID, c.ID} {
		_ = svc.Store.AddGroupMember(ctx, g.ID, uid)
	}
	// 3-way and 2-way group expenses (include a B<->C pair to exercise routing).
	mkExp(t, svc, "Hotel", 900, "USD", "2026-01-10T00:00:00Z", a.ID, &g.ID,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 600}, {UserID: b.ID, Amount: -300}, {UserID: c.ID, Amount: -300}})
	mkExp(t, svc, "Snacks", 200, "USD", "2026-02-10T00:00:00Z", b.ID, &g.ID,
		[]store.ExpenseParticipant{{UserID: b.ID, Amount: 100}, {UserID: c.ID, Amount: -100}})

	before := netByUser(t, svc)
	n, err := svc.ArchiveGroup(ctx, a.ID, g.ID, "2026-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("collapsed %d, want 2", n)
	}
	if after := netByUser(t, svc); after != before {
		t.Fatalf("group net changed: before %q after %q", before, after)
	}
	if got := countRows(t, svc, "archived_expenses"); got != 2 {
		t.Errorf("archived_expenses = %d, want 2", got)
	}
}

func TestArchiveCSVNoteFormat(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "Alice", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "Bob", Email: "b@x.com"})
	mkExp(t, svc, "Dinner", 1000, "SGD", "2026-01-10T00:00:00Z", a.ID, nil,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})

	if _, err := svc.ArchiveDirect(ctx, a.ID, b.ID, "2026-03-01"); err != nil {
		t.Fatal(err)
	}
	var note string
	if err := svc.Store.DB.QueryRow(`SELECT note FROM expenses WHERE split_type = 'ARCHIVE'`).Scan(&note); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Date,Description,Paid by,Amount,Currency,Category,Shares", "Dinner", "Alice", "Bob", "5.00"} {
		if !strings.Contains(note, want) {
			t.Errorf("CSV note missing %q; got:\n%s", want, note)
		}
	}
}

func TestArchiveNothing(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	if _, err := svc.ArchiveDirect(ctx, a.ID, b.ID, "2026-03-01"); err != ErrNothingToArchive {
		t.Fatalf("want ErrNothingToArchive, got %v", err)
	}
}
