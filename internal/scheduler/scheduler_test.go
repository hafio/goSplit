package scheduler

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/mail"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
)

// newSchedService builds a real service over a throwaway SQLite database so the
// scheduler's maintenance passes run against genuine store methods.
func newSchedService(t *testing.T) *service.Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "sched.db")
	st, err := store.Open(context.Background(), &config.Config{
		DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := &config.Config{BaseURL: "http://test.local", CacheRetentionInterval: 24 * time.Hour}
	svc := service.New(st, &mail.LogMailer{}, cfg)
	svc.SetSynchronousEmail() // no lingering mail goroutines during tests
	return svc
}

func TestNew(t *testing.T) {
	svc := newSchedService(t)
	s := New(svc, "holder-1")
	if s.Svc != svc || s.Holder != "holder-1" || s.Interval != time.Minute {
		t.Fatalf("New = %+v, want Svc set, holder-1, 1m", s)
	}
}

// TestTickGeneratesOnlyAsLeader: while another instance holds the lock a tick
// does nothing -- a due recurrence stays due. Once this instance leads, a tick
// generates it exactly once, and the next tick finds nothing due.
func TestTickGeneratesOnlyAsLeader(t *testing.T) {
	ctx := context.Background()
	svc := newSchedService(t)
	a, err := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.test"})
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Rent", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.CreateRecurrence(ctx, &store.ExpenseRecurrence{
		CronExpression: "0 0 * * *", JobName: "rent", TemplateExpenseID: tmpl.ID, CreatedBy: a.ID,
		NextRunAt: sql.NullString{String: "2020-01-01T00:00:00.000Z", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	generated := func() int {
		t.Helper()
		var n int
		if err := svc.Store.DB.QueryRow(`SELECT COUNT(*) FROM expenses WHERE recurrence_id IS NOT NULL`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if ok, err := svc.Store.AcquireLock(ctx, "cleanup", "other", time.Hour); err != nil || !ok {
		t.Fatalf("take the lock as another instance: ok=%v err=%v", ok, err)
	}
	s := New(svc, "me")
	s.tick(ctx)
	if n := generated(); n != 0 {
		t.Fatalf("a follower generated %d expenses", n)
	}

	if _, err := svc.Store.DB.Exec(`UPDATE scheduler_locks SET expires_at = '2000-01-01T00:00:00.000Z' WHERE id = 'cleanup'`); err != nil {
		t.Fatal(err)
	}
	s.tick(ctx)
	if n := generated(); n != 1 {
		t.Fatalf("the leader generated %d, want 1", n)
	}
	s.tick(ctx)
	if n := generated(); n != 1 {
		t.Errorf("the next tick generated again: %d in total, want 1", n)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	svc := newSchedService(t)
	s := New(svc, "runner")
	s.Interval = 5 * time.Millisecond // exercise the ticker branch quickly
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	time.Sleep(25 * time.Millisecond) // let a few ticks fire
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
